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

	"github.com/dedis/student_18_omniledger/omniledger/collection"
)

// Used for tests
var lleapID onet.ServiceID

const keyMerkleRoot = "merkleroot"
const keyNewKey = "newkey"
const keyNewValue = "newvalue"

func init() {
	var err error
	lleapID, err = onet.RegisterNewService(ServiceName, newService)
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
	// propagate the new transactions
	propagateTransactions messaging.PropagationFunc
	propTimeout           time.Duration

	storage *storage
}

// storageID reflects the data we're storing - we could store more
// than one structure.
const storageID = "main"

// storage is used to save our data locally.
type storage struct {
	sync.Mutex
}

type updateCollection struct {
	ID skipchain.SkipBlockID
}

// CreateSkipchain asks the cisc-service to create a new skipchain ready to
// store key/value pairs. If it is given exactly one writer, this writer will
// be stored in the skipchain.
// For faster access, all data is also stored locally in the Service.storage
// structure.
func (s *Service) CreateSkipchain(req *CreateSkipchain) (
	*CreateSkipchainResponse, error) {
	if req.Version != CurrentVersion {
		return nil, errors.New("version mismatch")
	}

	sb, err := s.createNewBlock(nil, &req.Roster, []*Transaction{&req.Transaction})
	if err != nil {
		return nil, err
	}
	s.save()
	return &CreateSkipchainResponse{
		Version:   CurrentVersion,
		Skipblock: sb,
	}, nil
}

// createNewBlock creates a new block and proposes it to the
// skipchain-service. Once the block has been created, we
// inform all nodes to update their internal collections
// to include the new transactions.
func (s *Service) createNewBlock(scID skipchain.SkipBlockID, r *onet.Roster, ts []*Transaction) (*skipchain.SkipBlock, error) {
	var sb *skipchain.SkipBlock
	var c collection.Collection

	if scID.IsNull() {
		// For a genesis block, we create a throwaway collection.
		c = collection.New(&collection.Data{}, &collection.Data{})

		sb = skipchain.NewSkipBlock()
		sb.Roster = r
		sb.MaximumHeight = 10
		sb.BaseHeight = 10
	} else {
		// For further blocks, we create a clone of the collection - this is
		// TODO: not very memory-friendly - we need to use some kind of transactions.
		c = s.getCollection(scID).coll.Clone()

		sbLatest, err := s.db().GetLatest(s.db().GetByID(scID))
		if err != nil {
			return nil, errors.New(
				"Could not get latest block from the skipchain: " + err.Error())
		}
		sb = sbLatest.Copy()
		if r != nil {
			sb.Roster = r
		}
	}

	for _, t := range ts {
		sigBuf, err := network.Marshal(&t.Signature)
		if err != nil {
			return nil, errors.New("Couldn't marshal Signature: " + err.Error())
		}
		err = c.Add(t.Key, t.Value, sigBuf)
		if err != nil {
			return nil, errors.New("error while storing in collection: " + err.Error())
		}
	}
	mr := c.GetRoot()
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
	ssbReply, err := s.skService().StoreSkipBlock(&ssb)
	if err != nil {
		return nil, err
	}

	replies, err := s.propagateTransactions(sb.Roster, &updateCollection{sb.Hash}, s.propTimeout)
	if err != nil {
		return nil, err
	}
	if replies != len(sb.Roster.List) {
		log.Lvl1(s.ServerIdentity(), "Only got", replies, "out of", len(sb.Roster.List))
	}

	return ssbReply.Latest, nil
}

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
	cdb := s.getCollection(sb.SkipChainID())
	for _, t := range data.Transactions {

		err = cdb.Store(t)
		if err != nil {
			log.Error(
				"error while storing in collection: " + err.Error())
		}
		return
	}
	if !bytes.Equal(cdb.RootHash(), data.MerkleRoot) {
		log.Error("hash of collection doesn't correspond to root hash")
	}
}

// SetKeyValue asks cisc to add a new key/value pair.
func (s *Service) SetKeyValue(req *SetKeyValue) (*SetKeyValueResponse, error) {
	// Check the input arguments
	// TODO: verify the signature on the key/value pair
	if req.Version != CurrentVersion {
		return nil, errors.New("version mismatch")
	}

	sb, err := s.createNewBlock(req.SkipchainID, nil, []*Transaction{&req.Transaction})
	if err != nil {
		return nil, err
	}
	_, data, err := network.Unmarshal(sb.Data, cothority.Suite)
	return &SetKeyValueResponse{
		Version:     CurrentVersion,
		Timestamp:   &data.(*Data).Timestamp,
		SkipblockID: &sb.Hash,
	}, nil
}

// GetProof searches for a key and returns a proof of the
// presence or the absence of this key.
func (s *Service) GetProof(req *GetProof) (resp *GetProofResponse, err error) {
	if req.Version != CurrentVersion {
		return nil, errors.New("version mismatch")
	}
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

// Tries to load the configuration and updates the data in the service
// if it finds a valid config-file.
func (s *Service) tryLoad() error {
	s.storage = &storage{}
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

	gas := &skipchain.GetAllSkipchains{}
	gasr, err := s.skService().GetAllSkipchains(gas)
	if err != nil {
		return err
	}

	allSkipchains := gasr.SkipChains
	for _, sb := range allSkipchains {
		s.getCollection(sb.SkipChainID())
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
	}
	if err := s.RegisterHandlers(s.CreateSkipchain, s.SetKeyValue,
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
	s.propTimeout = 10 * time.Second
	return s, nil
}
