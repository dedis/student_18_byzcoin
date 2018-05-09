// Package service implements the lleap service using the collection library to
// handle the merkle-tree. Each call to SetKeyValue updates the Merkle-tree and
// creates a new block containing the root of the Merkle-tree plus the new
// value that has been stored last in the Merkle-tree.
package service

import (
	"bytes"
	"errors"
	"fmt"
	"sync"
	"time"

	"gopkg.in/dedis/cothority.v2"
	"gopkg.in/dedis/cothority.v2/messaging"
	"gopkg.in/dedis/cothority.v2/skipchain"
	"gopkg.in/dedis/onet.v2"
	"gopkg.in/dedis/onet.v2/log"
	"gopkg.in/dedis/onet.v2/network"
	"gopkg.in/satori/go.uuid.v1"

	"github.com/dedis/student_18_omniledger/omniledger/collection"
	"github.com/dedis/student_18_omniledger/omniledger/darc"
)

const darcIDLen int = 32

// Used for tests
// TODO move to test
var omniledgerID onet.ServiceID
var verifyOmniledger = skipchain.VerifierID(uuid.NewV5(uuid.NamespaceURL, "Omniledger"))
var dummyKind = "dummy"

func init() {
	var err error
	omniledgerID, err = onet.RegisterNewService(ServiceName, newService)
	log.ErrFatal(err)
	network.RegisterMessages(&storage{}, &Data{}, &updateCollection{})
}

// Service is our lleap-service
type Service struct {
	// We need to embed the ServiceProcessor, so that incoming messages
	// are correctly handled.
	*onet.ServiceProcessor
	// collections cannot be stored, so they will be re-created whenever the
	// service reloads.
	collectionDB map[string]*collectionDB

	// queueWorkers is a map that points to channels that handle queueing and
	// starting of new blocks.
	queueWorkers map[string]chan Transaction
	// CloseQueues is closed when the queues should stop - this is mostly for
	// testing and there should be a better way to clean up services for testing...
	CloseQueues chan bool
	// verifiers map kinds to kind specific verification functions
	verifiers map[string]OmniledgerVerifier
	// propagate the new transactions
	propagateTransactions messaging.PropagationFunc

	storage *storage

	createSkipChainMut sync.Mutex
}

// storageID reflects the data we're storing - we could store more
// than one structure.
const storageID = "main"

// TODO: this should go into the genesis-configuration
var waitQueueing = 5 * time.Second

// storage is used to save our data locally.
type storage struct {
	sync.Mutex
	// PropTimeout is used when sending the request to integrate a new block
	// to all nodes.
	PropTimeout time.Duration
}

type updateCollection struct {
	ID skipchain.SkipBlockID
}

// CreateGenesisBlock asks the cisc-service to create a new skipchain ready to
// store key/value pairs. If it is given exactly one writer, this writer will
// be stored in the skipchain.
// For faster access, all data is also stored locally in the Service.storage
// structure.
func (s *Service) CreateGenesisBlock(req *CreateGenesisBlock) (
	*CreateGenesisBlockResponse, error) {
	// We use a big mutex here because we do not want to allow concurrent
	// creation of genesis blocks.
	// TODO an optimisation would be to lock on the skipchainID.
	s.createSkipChainMut.Lock()
	defer s.createSkipChainMut.Unlock()

	if req.Version != CurrentVersion {
		return nil, fmt.Errorf("version mismatch - got %d but need %d", req.Version, CurrentVersion)
	}
	if err := checkTxWithDarc(req.GenesisTx, &req.GenesisDarc); err != nil {
		return nil, err
	}

	// Create the genesis-transaction with a special key, it acts as a
	// reference to the actual genesis transaction.
	genesisTx := Transaction{
		Key:   []byte("genesis"),
		Value: req.GenesisTx.Key,
	}
	darcBuf, err := req.GenesisDarc.ToProto()
	if err != nil {
		return nil, err
	}
	darcTx := Transaction{
		Key:   req.GenesisDarc.GetID(),
		Kind:  []byte("darc"),
		Value: darcBuf,
	}

	sb, err := s.createNewBlock(nil, &req.Roster, []Transaction{darcTx, req.GenesisTx, genesisTx})
	if err != nil {
		return nil, err
	}
	s.save()

	s.queueWorkers[string(sb.SkipChainID())], err = s.createQueueWorker(sb.SkipChainID())
	if err != nil {
		return nil, err
	}
	return &CreateGenesisBlockResponse{
		Version:   CurrentVersion,
		Skipblock: sb,
	}, nil
}

func checkTxWithDarc(tx Transaction, d *darc.Darc) error {
	if len(tx.Key) < darcIDLen {
		return errors.New("incorrect key length")
	}
	if !bytes.Equal(tx.Key[0:darcIDLen], d.GetID()) {
		return errors.New("key is not the same as the darc ID")
	}
	if !bytes.Equal(tx.Kind, []byte(ActionAddGenesis)) {
		return fmt.Errorf("kind must be %s", ActionAddGenesis)
	}
	req, err := tx.ToDarcRequest()
	if err != nil {
		return err
	}
	if len(tx.Signatures) == 0 {
		return errors.New("no signatures")
	}
	if err := req.Verify(d); err != nil {
		return err
	}
	return nil
}

// SetKeyValue asks cisc to add a new key/value pair.
func (s *Service) SetKeyValue(req *SetKeyValue) (*SetKeyValueResponse, error) {
	if req.Version != CurrentVersion {
		return nil, errors.New("version mismatch")
	}

	c, ok := s.queueWorkers[string(req.SkipchainID)]
	if !ok {
		return nil, fmt.Errorf("we don't know skipchain ID %x", req.SkipchainID)
	}
	c <- req.Transaction

	return &SetKeyValueResponse{
		Version: CurrentVersion,
	}, nil
}

// GetProof searches for a key and returns a proof of the
// presence or the absence of this key.
func (s *Service) GetProof(req *GetProof) (resp *GetProofResponse, err error) {
	if req.Version != CurrentVersion {
		return nil, errors.New("version mismatch")
	}
	log.Lvlf2("%s: Getting proof for key %x on sc %x", s.ServerIdentity(), req.Key, req.ID)
	latest, err := s.db().GetLatest(s.db().GetByID(req.ID))
	if err != nil {
		return
	}
	proof, err := NewProof(s.getCollection(req.ID), s.db(), latest.Hash, req.Key)
	if err != nil {
		return
	}
	resp = &GetProofResponse{
		Version: CurrentVersion,
		Proof:   *proof,
	}
	return
}

// SetPropagationTimeout overrides the default propagation timeout that is used
// when a new block is announced to the nodes.
func (s *Service) SetPropagationTimeout(p time.Duration) {
	s.storage.Lock()
	s.storage.PropTimeout = p
	s.storage.Unlock()
}

// createNewBlock creates a new block and proposes it to the
// skipchain-service. Once the block has been created, we
// inform all nodes to update their internal collections
// to include the new transactions.
func (s *Service) createNewBlock(scID skipchain.SkipBlockID, r *onet.Roster, ts []Transaction) (*skipchain.SkipBlock, error) {
	var sb *skipchain.SkipBlock
	var mr []byte

	if scID.IsNull() {
		// For a genesis block, we create a throwaway collection.
		c := collection.New(&collection.Data{}, &collection.Data{})

		sb = skipchain.NewSkipBlock()
		sb.Roster = r
		sb.MaximumHeight = 10
		sb.BaseHeight = 10
		// We have to register the verification functions in the genesis block
		sb.VerifierIDs = []skipchain.VerifierID{skipchain.VerifyBase, verifyOmniledger}
		for _, t := range ts {
			// For the moment, we assume that in the genesis block, all
			// transactions are valid.
			t.Valid = true
			log.Printf("Adding transaction %+v", t)
			err := c.Add(t.Key, t.Value, t.Kind)
			if err != nil {
				return nil, errors.New("error while storing in collection: " + err.Error())
			}
		}
		mr = c.GetRoot()
	} else {
		// For further blocks, we use tryHash to get a hash and undo the changes.
		sbLatest, err := s.db().GetLatest(s.db().GetByID(scID))
		if err != nil {
			return nil, errors.New(
				"Could not get latest block from the skipchain: " + err.Error())
		}
		sb = sbLatest.Copy()
		if r != nil {
			sb.Roster = r
		}
		cdb := s.getCollection(scID)
		mr, err = cdb.tryHash(ts)
		if err != nil {
			return nil, errors.New("error while getting merkle root from collection: " + err.Error())
		}
		s.validateTransactions(cdb, ts)
	}

	data := &Data{
		MerkleRoot:   mr,
		Transactions: ts,
		Timestamp:    time.Now().Unix(),
	}

	var err error
	sb.Data, err = network.Marshal(data)
	if err != nil {
		return nil, errors.New("Couldn't marshal data: " + err.Error())
	}

	var ssb = skipchain.StoreSkipBlock{
		NewBlock:          sb,
		TargetSkipChainID: scID,
	}
	log.Lvl2("Storing skipblock with transactions %+v", ts)
	ssbReply, err := s.skService().StoreSkipBlock(&ssb)
	if err != nil {
		return nil, err
	}

	s.storage.Lock()
	pto := s.storage.PropTimeout
	s.storage.Unlock()
	// TODO: replace this with some kind of callback from the skipchain-service
	replies, err := s.propagateTransactions(sb.Roster, &updateCollection{sb.Hash}, pto)
	if err != nil {
		log.Lvl1("Propagation-error:", err.Error())
	}
	if replies != len(sb.Roster.List) {
		log.Lvl1(s.ServerIdentity(), "Only got", replies, "out of", len(sb.Roster.List))
	}

	return ssbReply.Latest, nil
}

// updateCollection is called once a skipblock has been stored.
// It is called by the leader, and every node will add the
// transactions in the block to its collection.
func (s *Service) updateCollection(msg network.Message) {
	uc, ok := msg.(*updateCollection)
	if !ok {
		return
	}

	sb, err := s.db().GetLatest(s.db().GetByID(uc.ID))
	if err != nil {
		log.Errorf("didn't find latest block for %x", uc.ID)
		return
	}
	_, dataI, err := network.Unmarshal(sb.Data, cothority.Suite)
	data, ok := dataI.(*Data)
	if err != nil || !ok {
		log.Errorf("couldn't unmarshal data")
		return
	}
	// TODO: wrap this in a transaction
	log.Lvlf2("%s: Updating transactions for %x", s.ServerIdentity(), sb.SkipChainID())
	cdb := s.getCollection(sb.SkipChainID())
	for _, t := range data.Transactions {
		if !t.Valid {
			continue
		}
		log.Lvlf2("Storing transaction key/kind/value: %x / %x / %x", t.Key, t.Kind, t.Value)
		err = cdb.Store(&t)
		if err != nil {
			log.Error(
				"error while storing in collection: " + err.Error())
		}
	}
	if !bytes.Equal(cdb.RootHash(), data.MerkleRoot) {
		log.Error("hash of collection doesn't correspond to root hash")
	}
}

func (s *Service) getCollection(id skipchain.SkipBlockID) *collectionDB {
	idStr := fmt.Sprintf("%x", id)
	col := s.collectionDB[idStr]
	if col == nil {
		db, name := s.GetAdditionalBucket([]byte(idStr))
		s.collectionDB[idStr] = newCollectionDB(db, name)
		return s.collectionDB[idStr]
	}
	return col
}

// interface to skipchain.Service
func (s *Service) skService() *skipchain.Service {
	return s.Service(skipchain.ServiceName).(*skipchain.Service)
}

// gives us access to the skipchain's database, so we can get blocks by ID
func (s *Service) db() *skipchain.SkipBlockDB {
	return s.skService().GetDB()
}

// saves all skipblocks.
func (s *Service) save() {
	s.storage.Lock()
	defer s.storage.Unlock()
	err := s.Save([]byte(storageID), s.storage)
	if err != nil {
		log.Error("Couldn't save file:", err)
	}
}

// createQueueWorker sets up a worker that will listen on a channel for
// incoming requests and then create a new block every epoch.
func (s *Service) createQueueWorker(scID skipchain.SkipBlockID) (chan Transaction, error) {
	c := make(chan Transaction)
	go func() {
		ts := []Transaction{}
		to := time.After(waitQueueing)
		for {
			select {
			case t := <-c:
				ts = append(ts, t)
				log.Lvlf2("%x: Stored transaction %+v - length is %d: %+v", scID, t, len(ts), ts)
			case <-to:
				log.Lvlf2("%x: New epoch and transaction-length: %d", scID, len(ts))
				if len(ts) > 0 {
					sb, err := s.db().GetLatest(s.db().GetByID(scID))
					if err != nil {
						panic("DB is in bad state and cannot find skipchain anymore: " + err.Error())
					}
					_, err = s.createNewBlock(scID, sb.Roster, ts)
					if err != nil {
						log.Error("couldn't create new block: " + err.Error())

						to = time.After(waitQueueing)
						continue
					}
					ts = []Transaction{}
				}
				to = time.After(waitQueueing)
			case <-s.CloseQueues:
				log.Lvlf2("closing queues...")
				return
			}
		}
	}()
	return c, nil
}

// Tries to load the configuration and updates the data in the service
// if it finds a valid config-file.
func (s *Service) tryLoad() error {
	s.storage = &storage{
		PropTimeout: 10 * time.Second,
	}
	msg, err := s.Load([]byte(storageID))
	if err != nil {
		return err
	}
	if msg != nil {
		var ok bool
		s.storage, ok = msg.(*storage)
		if !ok {
			return errors.New("Data of wrong type")
		}
	}
	if s.storage == nil {
		s.storage = &storage{}
	}
	s.collectionDB = map[string]*collectionDB{}
	s.queueWorkers = map[string]chan Transaction{}

	gas := &skipchain.GetAllSkipchains{}
	gasr, err := s.skService().GetAllSkipchains(gas)
	if err != nil {
		return err
	}
	// GetAllSkipchains erronously returns all skipBLOCKS, so we need
	// to filter out the skipchainIDs.
	scIDs := map[string]bool{}
	for _, sb := range gasr.SkipChains {
		scIDs[string(sb.SkipChainID())] = true
	}

	for scID := range scIDs {
		sbID := skipchain.SkipBlockID(scID)
		s.getCollection(sbID)
		s.queueWorkers[scID], err = s.createQueueWorker(sbID)
		if err != nil {
			return err
		}
	}

	return nil
}

// newService receives the context that holds information about the node it's
// running on. Saving and loading can be done using the context. The data will
// be stored in memory for tests and simulations, and on disk for real
// deployments.
func newService(c *onet.Context) (onet.Service, error) {
	s := &Service{
		ServiceProcessor: onet.NewServiceProcessor(c),
		CloseQueues:      make(chan bool),
	}
	if err := s.RegisterHandlers(s.CreateGenesisBlock, s.SetKeyValue,
		s.GetProof); err != nil {
		log.ErrFatal(err, "Couldn't register messages")
	}
	if err := s.tryLoad(); err != nil {
		log.Error(err)
		return nil, err
	}

	var err error
	s.propagateTransactions, err = messaging.NewPropagationFunc(c, "OmniledgerPropagate", s.updateCollection, -1)
	if err != nil {
		return nil, err
	}

	s.verifiers = make(map[string]OmniledgerVerifier)
	// For testing
	s.verifiers[dummyKind] = func(cdb *collectionDB, tx *Transaction) bool {
		return true
	}
	skipchain.RegisterVerification(c, verifyOmniledger, s.verifySkipBlock)
	return s, nil
}

// We use the omniledger as a receiver (as is done in the identity service),
// so we can access e.g. the collectionDBs of the service.
func (s *Service) verifySkipBlock(newID []byte, newSB *skipchain.SkipBlock) bool {
	_, dataI, err := network.Unmarshal(newSB.Data, cothority.Suite)
	data, ok := dataI.(*Data)
	if err != nil || !ok {
		log.Errorf("couldn't unmarshal data")
		return false
	}
	txs := data.Transactions
	cdb := s.getCollection(newSB.Hash)
	for _, tx := range txs {
		f, exists := s.verifiers[string(tx.Kind)]
		if !exists || tx.Valid != f(cdb, &tx) {
			return false
		}
	}
	return true
}

// validateTransactions sets the valid-flag of the transaction according to the
// registered OmniledgerVerifiers.
func (s *Service) validateTransactions(cdb *collectionDB, txs []Transaction) {
	for i := range txs {
		f, exists := s.verifiers[string(txs[i].Kind)]
		// If the leader does not have a verifier for this kind, it drops the
		// transaction.
		if !exists {
			txs = append(txs[:i], txs[i+1:]...)
			continue
		}
		txs[i].Valid = f(cdb, &txs[i])
	}
}

// RegisterVerification stores the verification in a map and will
// call it whenever a verification needs to be done.
func (s *Service) registerVerification(kind string, f OmniledgerVerifier) error {
	s.verifiers[kind] = f
	return nil
}

// RegisterVerification stores the verification in a map and will
// call it whenever a verification needs to be done.
// GetService makes it possible to give either an `onet.Context` or
// `onet.Server` to `RegisterVerification`.
func RegisterVerification(s skipchain.GetService, kind string, f OmniledgerVerifier) error {
	scs := s.Service(ServiceName)
	if scs == nil {
		return errors.New("Didn't find our service: " + ServiceName)
	}
	return scs.(*Service).registerVerification(kind, f)
}

func (a Action) String() string {
	switch a {
	case Create:
		return "create"
	case Update:
		return "update"
	case Remove:
		return "remove"
	default:
		return "invalid action"
	}
}
