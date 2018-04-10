// Package service implements the lleap service using the collection library to
// handle the merkle-tree. Each call to SetKeyValue updates the Merkle-tree and
// creates a new block containing the root of the Merkle-tree plus the new value
// that has been stored last in the Merkle-tree.
package service

import (
    /*
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
    "time"
    */
    "encoding/json"
    "reflect"
	"fmt"
	"errors"
	"sync"

	"gopkg.in/dedis/crypto.v0/sign"

	// "github.com/dedis/cothority"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/kyber"
	// "github.com/dedis/kyber/sign/schnorr"
	// "github.com/dedis/kyber/util/key"
	"github.com/dedis/student_18_omniledger/lleap"
	"github.com/dedis/student_18_omniledger/lleap/collection"
    "github.com/dedis/student_18_omniledger/cothority_template/darc"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"gopkg.in/dedis/onet.v1/network"
)

// Used for tests
var lleapID onet.ServiceID

const keyMerkleRoot = "merkleroot"
const keyNewKey = "newkey"
const keyNewValue = "newvalue"

// storageID reflects the data we're storing - we could store more
// than one structure.
const storageID = "main"

func init() {
	var err error
	lleapID, err = onet.RegisterNewService(lleap.ServiceName, newService)
	log.ErrFatal(err)
	network.RegisterMessage(&storage{})
}

// Service is our lleap-service
type Service struct {
	// We need to embed the ServiceProcessor, so that incoming messages
	// are correctly handled.
	*onet.ServiceProcessor
	// collections cannot be stored, so they will be re-created whenever the
	// service reloads.
	collectionDB map[string]*collectionDB

	storage *storage
}

// storage is used to save our data locally.
type storage struct {
    // IDBlock stores one identity together with the latest and the currently
    // proposed skipblock.
    // Identities map[string]*identity.IDBlock
    DarcBlocks map[string]*DarcBlock
    // PL: Is used to sign the votes
	Private    map[string]kyber.Scalar
    // PL: Entities allowed to modify the data(-structure)?
	// Writers    map[string][]byte
	sync.Mutex
}

type DarcBlock struct {
    sync.Mutex
    Latest *Data
    // Proposed seems to be specific to identity, looking at skipchain's
    // interface (and grepping its source), it doesn't seem relevant.
    // Proposed *Data
    LatestSkipblock *skipchain.SkipBlock
}

type Data struct {
    // Root of the merkle tree after applying the transactions to the
    // kv store
    MerkleRoot []byte
    // We can have multiple Transactions in a single block.
    // However, they should not depend on each other, since if we do 
    // multi-submissions, the order is not well defined. So everything which
    // happens in a single block happends concurrently and no order is specified.
    // Thus we only need one Merkle root per block and not one per request.

    // The transactions applied to the kv store with this block
    Transactions []*lleap.Transaction
    Timestamp int64
}

func checkTx(tx lleap.Transaction) error {
    if tx.Key == nil {
        return errors.New("Key is nil")
    }
    if tx.Kind == nil {
        return errors.New("Kind is nil")
    }
    if tx.Value == nil {
        return errors.New("Value is nil")
    }
    return nil
}
// CreateSkipchain asks the cisc-service to create a new skipchain ready to store
// key/value pairs. If it is given exactly one writer, this writer will be stored
// in the skipchain.
// For faster access, all data is also stored locally in the Service.storage
// structure.
func (s *Service) CreateSkipchain(req *lleap.CreateSkipchain) (*lleap.CreateSkipchainResponse, error) {
	if req.Version != lleap.CurrentVersion {
		return nil, errors.New("version mismatch")
	}
    if err := checkTx(req.Transaction); err != nil {
        return nil, err
    }
	// kp := key.NewKeyPair(cothority.Suite)
    tmpColl := collection.New(collection.Data{}, collection.Data{})
    key := getKey(&req.Transaction)
    tmpColl.Add(key, req.Transaction.Value)
    // merkleroot := tmpColl.GetRoot()
    /*
    data := &Data{
        MerkleRoot: merkleroot,
        Transactions: []*lleap.Transaction{&req.Transaction},
    }
    */

    // Create skipchain here, using skipchain.Service
    // skb, err := skipchain.New()...
    /*
	if err != nil {
		return nil, err
	}
	gid := skb.SkipChainID()
    s.getCollection(gid).Store(key, req.Transaction.Value,
        req.Transaction.Signature)
    s.storage.DarcBlocks[gid] = &DarcBlock{
        Latest:          data,
		LatestSkipblock: skb,
	}
	s.storage.Private[gid] = kp.Private
	s.save()
	return &lleap.CreateSkipchainResponse{
		Version:   lleap.CurrentVersion,
		Skipblock: skb,
	}, nil
    */
    return nil, nil
}

// SetKeyValue asks cisc to add a new key/value pair.
func (s *Service) SetKeyValue(req *lleap.SetKeyValue) (
        *lleap.SetKeyValueResponse, error) {
	// Check the input arguments
	if req.Version != lleap.CurrentVersion {
		return nil, errors.New("version mismatch")
	}
    if err := checkTx(req.Transaction); err != nil {
        return nil, err
    }
	gid := string(req.SkipchainID)
	darcblk := s.storage.DarcBlocks[gid]
	priv := s.storage.Private[gid]
	if darcblk == nil || priv == nil {
		return nil, errors.New("don't have this chain stored")
	}
    // Verify darc
    // Note: The verify function needs the collection to be up to date.
    // TODO: Make sure that is the case.
	log.Lvl1("Verifying signature")
    err := s.getCollection(req.SkipchainID).verify(&req.Transaction)
    if err != nil {
		log.Lvl1("signature verification failed")
        return nil, err
    }
	log.Lvl1("signature verification succeeded")

	// Store the pair in the collection
	coll := s.getCollection(req.SkipchainID)
    collKey := getKey(&req.Transaction)
	if _, _, err := coll.GetValue(collKey); err == nil {
		return nil, errors.New("cannot overwrite existing value")
	}
    sig, err := network.Marshal(req.Transaction.Signature)
    if err != nil {
        return nil, err
    }
    err = coll.Store(collKey, req.Transaction.Value, sig)
	if err != nil {
		return nil, errors.New("error while storing in collection: " + err.Error())
	}

    // here we should propose a new skipblock to skipchain



	// Update the identity
    /*
	prop := idb.Latest.Copy()
	prop.Storage[keyMerkleRoot] = string(coll.RootHash())
	prop.Storage[keyNewKey] = string(req.Key)
	prop.Storage[keyNewValue] = string(req.Value)
	// TODO: Should also store the signature.
    // PL: What does this function do? What is the equivalent function to
    // interface directly with skipchain?
	_, err = s.idService().ProposeSend(&identity.ProposeSend{
		ID:      identity.ID(req.SkipchainID),
		Propose: prop,
	})
	if err != nil {
		return nil, err
	}

	hash, err := prop.Hash(cothority.Suite)
	if err != nil {
		return nil, err
	}
	sig, err := schnorr.Sign(cothority.Suite, priv, hash)
	if err != nil {
		return nil, err
	}
    // See the PL above
	resp, err := s.idService().ProposeVote(&identity.ProposeVote{
		ID:        identity.ID(req.SkipchainID),
		Signer:    "service",
		Signature: sig,
	})
	if err != nil {
		return nil, err
	}
	if resp.Data == nil {
		return nil, errors.New("couldn't store new skipblock")
	}
	timestamp := int64(resp.Data.Index)
	return &lleap.SetKeyValueResponse{
		Version:     lleap.CurrentVersion,
		Timestamp:   &timestamp,
		SkipblockID: &resp.Data.Hash,
	}, nil
    */
    return nil, nil
}

// GetValue looks up the key in the given skipchain and returns the corresponding value.
func (s *Service) GetValue(req *lleap.GetValue) (*lleap.GetValueResponse, error) {
	if req.Version != lleap.CurrentVersion {
		return nil, errors.New("version mismatch")
	}

    if err := s.getCollection(req.SkipchainID).verify(&req.Transaction); err != nil {
        return nil, err
    }
    key := getKey(&req.Transaction)
	value, sig, err := s.getCollection(req.SkipchainID).GetValue(key)
	if err != nil {
		return nil, errors.New("couldn't get value for key: " + err.Error())
	}
	return &lleap.GetValueResponse{
		Version:   lleap.CurrentVersion,
		Value:     &value,
		Signature: &sig,
        // TODO: Proof
	}, nil
}

func getKey(tx *lleap.Transaction) []byte {
    return append(append(tx.Kind, []byte(":")...), tx.Key...)
}
// TODO: Replace collectionsDB[idStr] with this
func (s *Service) getCollection(id skipchain.SkipBlockID) *collectionDB {
	idStr := fmt.Sprintf("%x", id)
	col := s.collectionDB[idStr]
	if col == nil {
		db, name := s.GetAdditionalBucket([]byte(idStr))
		s.collectionDB[idStr] = newCollectionDB(db, string(name))
		return s.collectionDB[idStr]
	}
	return col
}

// interface to identity.Service
/*
func (s *Service) idService() *identity.Service {
	return s.Service(identity.ServiceName).(*identity.Service)
}
*/

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
    // silence compiler for the moment, real code from before below
    return nil
}
/*
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
	if s.storage.Identities == nil {
		s.storage.Identities = map[string]*identity.IDBlock{}
	}
	if s.storage.Private == nil {
		s.storage.Private = map[string]kyber.Scalar{}
	}
	if s.storage.Writers == nil {
		s.storage.Writers = map[string][]byte{}
	}
	s.collectionDB = map[string]*collectionDB{}
	for _, id := range s.storage.Identities {
		s.getCollection(id.LatestSkipblock.SkipChainID())
	}
	return nil
}
*/

// newService receives the context that holds information about the node it's
// running on. Saving and loading can be done using the context. The data will
// be stored in memory for tests and simulations, and on disk for real deployments.
func newService(c *onet.Context) (onet.Service, error) {
	s := &Service{
		ServiceProcessor: onet.NewServiceProcessor(c),
	}
	if err := s.RegisterHandlers(s.CreateSkipchain, s.SetKeyValue,
		s.GetValue); err != nil {
		log.ErrFatal(err, "Couldn't register messages")
	}
	if err := s.tryLoad(); err != nil {
		log.Error(err)
		return nil, err
	}
	return s, nil
}

// VerifyBlock makes sure that the new block is legit. This function will be
// called by the skipchain on all nodes before they sign.
func (s *Service) VerifyBlock(sbID []byte, sb *skipchain.SkipBlock) bool {
	// Putting it all in a function for easier error-printing
    // idStr := fmt.Sprintf("%x", sbID)
	err := func() error {
		if sb.Index == 0 {
			log.Lvl4("Always accepting genesis-block")
			return nil
		}
		_, dataInt, err := network.Unmarshal(sb.Data)
		if err != nil {
			return errors.New("got unknown packet")
		}
		data, ok := dataInt.(*Data)
		if !ok {
			return fmt.Errorf("got packet-type %s", reflect.TypeOf(dataInt))
		}
        /*
		hash, err := data.Hash(s.Suite().(kyber.HashFactory))
		if err != nil {
			return err
		}
        */
		// Verify that all signatures work out
		if len(sb.BackLinkIDs) == 0 {
			return errors.New("No backlinks stored")
		}
		s.storage.Mutex.Lock() // does not exist in lleap. replace
		defer s.storage.Mutex.Unlock()
		var latest *skipchain.SkipBlock
		for _, dblk := range s.storage.DarcBlocks {
			if dblk.LatestSkipblock.Hash.Equal(sb.BackLinkIDs[0]) {
				latest = dblk.LatestSkipblock
			}
		}
		if latest == nil {
            // update with syncchain
		}
        // func Unmarshal(buf []byte, suite Suite) (MessageTypeID, Message, error)
        // thus dataInt : Message, and Message = interface{}
		_, dataInt, err = network.Unmarshal(latest.Data)
		if err != nil {
			return err
		}
        // latest : SkipBlock, so dataInt : []byte
		// dataLatest := dataInt.(*Data) is a type assertion, returning a value
        // of type *Data or -if not possible- panicking.
        // dataLatest is the data of the latest skipblock which has been
        // added to the skipchain, whereas data is the data contained in the
        // skipblock to be added
		// dataLatest := dataInt.(*Data)
        col := s.getCollection(sbID)
        for _, tx := range data.Transactions {
            if err := col.verify(tx); err != nil {
                return errors.New("invalid signature on request")
            }
        }
        /*
		sigCnt := 0
		for drc, sig := range data.Votes {
			if signer := dataLatest.Darcs[drc]; signer != nil {
				log.Lvl3("Against darc", signer)
                // if err := darc.Verify()... err == nil {}
                // but how can we pass the good parameters?
                // should we change some structures?
                // The data in each skipblock should probably contain the darc
                // and rule of the previous skipblock which allows for the
                // operation from which the new block is the result
                if err := schnorr.Verify(s.Suite(), pub.Point, hash, sig); err == nil {
					log.Lvl2("Found correct signature of device", dev)
					sigCnt++
				}
			} else {
				log.Lvl2("Not representative signature detected:", dev)
			}
		}
		if sigCnt >= dataLatest.Threshold || sigCnt == len(dataLatest.Device) {
			return nil
		}
		return errors.New("not enough signatures")
        */
        // Silence the compiler till we know what to do.
        return nil
	}()
	if err != nil {
		log.Lvl2("Error while validating block:", err)
		return false
	}
	return true
}

func (darcs *collectionDB) findDarc (darcid darc.ID) (*darc.Darc, error) {
    darcKey := append([]byte("darc:"), darcid...)
    d, _, err := darcs.GetValue(darcKey)
    if err != nil {
        log.Lvl2("Error while getting record from collectionDB:", err)
        return nil, err
    }
    var tmp interface{}
    err = json.Unmarshal(d, tmp)
    if err != nil {
        return nil, err
    }
    res, ok := tmp.(*darc.Darc)
    if !ok {
        return nil, errors.New("Could not assert type")
    }
    return res, nil
}

// getPath finds a path from the master darc to the signer. The path consists
// of a slice of int, where path[i] is the rule in the darc at tree level i
// which indicates the next subject.
// We consider only the action "user". It implies read-write rights for the
// keys which do not belong to a darc. If a darc is to be modified, the signer
// must be in it's admin rule.
func (darcs *collectionDB) checkUserPath(signer darc.SubjectPK) (
        []int, error) {
    // TODO: Find a generic way to refer to the master darc directly.
    // Perhaps via the genesis block?
	masterDarc, err := darcs.findDarc([]byte("masterDarcKey"))
	if err != nil {
		return nil, err
	}
    subs, err := getUserSubjects(masterDarc)
    if err != nil {
        return nil, err
    }
    // recursively search the tree of subjects for the signer,
    // following user rules and updating the pathIndex.
	var pathIndex []int
    pa, err := darcs.findSubject(subs, &darc.Subject{PK: &signer}, pathIndex)
	//fmt.Println(pa)
	return pa, err
}

func getUserSubjects (d *darc.Darc) ([]*darc.Subject, error) {
	rules := *d.Rules
    var subs []*darc.Subject
    for _, r := range rules {
        if r.Action == "User" {
            subs = append(subs, *r.Subjects...)
        }
    }
    if len(subs) == 0 {
        return nil, errors.New("no user subects found")
    }
    return subs, nil
}

// TODO: Make this less ugly
// TODO: Check for cyclic inclusions in darcs, could be an attack vector for
// DoS
func (darcs *collectionDB) findSubject(subjects []*darc.Subject, requester *darc.Subject,
                                        pathIndex []int) ([]int, error) {
    for i, s := range subjects {
		if darc.CompareSubjects(s, requester) == true {
			pathIndex = append(pathIndex, i)
			return pathIndex, nil
		} else if s.Darc != nil {
			targetDarc, err := darcs.findDarc(s.Darc.ID)
			if err != nil {
                continue
				// return nil, err
			}
            subs, err := getUserSubjects(targetDarc)
			if err != nil {
                continue
				// return nil, errors.New("User rule ID not found")
			}
			pa, err := darcs.findSubject(subs, requester, append(pathIndex, i))
			if err != nil {
                continue
			} else {
				return pa, nil
			}
		}
	}
	return nil, errors.New("Subject not found")
}

// verify checks that the Transaction has a non-nil Key and Kind and that the 
// signature in the Transaction is indeed correct. Furthermore, it checks
// that there exists a valid path from the master darc to the signer via user
// rules.
func (darcs *collectionDB) verify(tx *lleap.Transaction) error {
    if tx == nil {
		return errors.New("Transaction is nil")
	}
    if tx.Kind == nil {
		return errors.New("Kind is nil")
    }
    if tx.Key == nil {
		return errors.New("Key is nil")
    }
    b := append(append(tx.Key, tx.Kind...), tx.Value...)
	if b == nil {
		return errors.New("nothing to verify, message is empty")
	}
    sig := tx.Signature
	pub := sig.Signer.Point
    err := sign.VerifySchnorr(network.Suite, pub, b, sig.Signature)
	if err != nil {
		return err
	}
	//Check if path from rule to signer is correct
	_, err = darcs.checkUserPath(sig.Signer)
	if err != nil {
		return err
	}
	return err
}

