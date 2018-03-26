// Package service implements the lleap service using the collection library to
// handle the merkle-tree. Each call to SetKeyValue updates the Merkle-tree and
// creates a new block containing the root of the Merkle-tree plus the new value
// that has been stored last in the Merkle-tree.
package service

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"errors"
	"fmt"
	"sync"
    "time"

	"github.com/dedis/cothority"
	// "github.com/dedis/cothority/identity"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/kyber"
	"github.com/dedis/kyber/sign/schnorr"
	"github.com/dedis/kyber/util/key"
	"github.com/dedis/student_18_omniledger/lleap"
    "github.com/dedis/student_18_omniledger/cothority_template/darc"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
)

// Used for tests
var lleapID onet.ServiceID

const keyMerkleRoot = "merkleroot"
const keyNewKey = "newkey"
const keyNewValue = "newvalue"

const darcBytes = []bytes("darc:")

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

// storageID reflects the data we're storing - we could store more
// than one structure.
const storageID = "main"

type Data struct {
    // Darcs allowed to sign
    MerkleRoot []byte
    Roster *onet.Roster
    // Request represents the request which is sent by the client to update the
    // state of the skipchain. It contains information on which Darc and which
    // rule of that darc allow to perform the operation specified in message.
    /*From package darc:
    type Request struct {
	    //ID of the Darc having the access control policy
	    DarcID ID
    	//ID showing allowed rule
    	RuleID int
    	//Message - Can be a string or a marshalled JSON 
    	Message []byte
    } */
    // Requests []darc.Request
    // add this to the request struct itself
    // We can have multiple requests in a single block.
    // However, they should not depend on each other, since if we do 
    // multi-submissions, the order is not clear. Si everything which happens
    // in a single block happends concurrently and no order is specified.
    // Thus we only need one Merkle root per block and not one per request.
    Sigs []darc.Signature
    Timestamp time.Time
}


type DarcBlock struct {
    sync.Mutex
    Latest *Data
    Proposed *Data
    LatestSkipblock *skipchain.SkipBlock
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

// CreateSkipchain asks the cisc-service to create a new skipchain ready to store
// key/value pairs. If it is given exactly one writer, this writer will be stored
// in the skipchain.
// For faster access, all data is also stored locally in the Service.storage
// structure.
func (s *Service) CreateSkipchain(req *lleap.CreateSkipchain) (*lleap.CreateSkipchainResponse, error) {
	if req.Version != lleap.CurrentVersion {
		return nil, errors.New("version mismatch")
	}

	kp := key.NewKeyPair(cothority.Suite)
    // get rid of identity here by using a struct which contains a DARC and
    // some sort of kv store.
    // Also adjust the structure of req *lleap.CreateSkipchain.
    // And what is writer? The device allowed to modify the kv-store (as well
    // the set of Devices)?
    /*
	data := &identity.Data{
		Threshold: 2,
		Device:    map[string]*identity.Device{"service": &identity.Device{Point: kp.Public}},
		Roster:    &req.Roster,
	}
    */
    data := Data{
        Darc: req.Darc,
        Roster: &req.Roster,
    }

    /*
	if len(*req.Writers) == 1 {
		data.Storage = map[string]string{"writer": string((*req.Writers)[0])}
	}
    */

    // replace this by something interacting with skipchain directly

    ssb, err := skipchain.CreateGenesisSignature(req.Roster, 
                                                    10, 
                                                    10, 
                                                    verficTODO,
                                                    req.Data,
                                                    nil,
                                                    privTODO)
    /*
	cir, err := s.idService().CreateIdentityInternal(&identity.CreateIdentity{
		Data: data,
	}, "", "") */
	if err != nil {
		return nil, err
	}
	gid := string(cir.Genesis.SkipChainID())
    // if we modify data as described above, we can just use it here.
    // we can still use the genesisblock, but the one from skipchain
	s.storage.Identities[gid] = &identity.IDBlock{
		Latest:          data,
		LatestSkipblock: cir.Genesis,
	}
	s.storage.Private[gid] = kp.Private
	s.storage.Writers[gid] = []byte(data.Storage["writer"])
	s.save()
	return &lleap.CreateSkipchainResponse{
		Version:   lleap.CurrentVersion,
		Skipblock: cir.Genesis,
	}, nil
}

// SetKeyValue asks cisc to add a new key/value pair.
func (s *Service) SetKeyValue(req *lleap.SetKeyValue) (*lleap.SetKeyValueResponse, error) {
	// Check the input arguments
	// TODO: verify the signature on the key/value pair
	if req.Version != lleap.CurrentVersion {
		return nil, errors.New("version mismatch")
	}
	gid := string(req.SkipchainID)
	idb := s.storage.Identities[gid]
	priv := s.storage.Private[gid]
	if idb == nil || priv == nil {
		return nil, errors.New("don't have this identity stored")
	}
    // PL: Does this check make sense? What if pub == nil?
	if pub := s.storage.Writers[gid]; pub != nil {
		log.Lvl1("Verifying signature")
		public, err := x509.ParsePKIXPublicKey(pub)
		if err != nil || public == nil {
			return nil, err
		}
		hash := sha256.New()
		hash.Write(req.Key)
		hash.Write(req.Value)
		hashed := hash.Sum(nil)[:]
		err = rsa.VerifyPKCS1v15(public.(*rsa.PublicKey), crypto.SHA256, hashed, req.Signature)
		if err != nil {
			log.Lvl1("signature verification failed")
			return nil, errors.New("couldn't verify signature")
		}
		log.Lvl1("signature verification succeeded")
	}

	// Store the pair in the collection
	coll := s.getCollection(req.SkipchainID)
	if _, _, err := coll.GetValue(req.Key); err == nil {
		return nil, errors.New("cannot overwrite existing value")
	}
	err := coll.Store(req.Key, req.Value, req.Signature)
	if err != nil {
		return nil, errors.New("error while storing in collection: " + err.Error())
	}

	// Update the identity
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
}

// GetValue looks up the key in the given skipchain and returns the corresponding value.
func (s *Service) GetValue(req *lleap.GetValue) (*lleap.GetValueResponse, error) {
	if req.Version != lleap.CurrentVersion {
		return nil, errors.New("version mismatch")
	}

	value, sig, err := s.getCollection(req.SkipchainID).GetValue(req.Key)
	if err != nil {
		return nil, errors.New("couldn't get value for key: " + err.Error())
	}
	return &lleap.GetValueResponse{
		Version:   lleap.CurrentVersion,
		Value:     &value,
		Signature: &sig,
	}, nil
}

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
    idStr := fmt.Sprintf("%x", sbID)
	err := func() error {
		if sb.Index == 0 {
			log.Lvl4("Always accepting genesis-block")
			return nil
		}
		_, dataInt, err := network.Unmarshal(sb.Data, s.Suite())
		if err != nil {
			return errors.New("got unknown packet")
		}
		data, ok := dataInt.(*Data)
		if !ok {
			return fmt.Errorf("got packet-type %s", reflect.TypeOf(dataInt))
		}
		hash, err := data.Hash(s.Suite().(kyber.HashFactory))
		if err != nil {
			return err
		}
		// Verify that all signatures work out
		if len(sb.BackLinkIDs) == 0 {
			return errors.New("No backlinks stored")
		}
		s.storageMutex.Lock() // does not exist in lleap. replace
		defer s.storageMutex.Unlock()
		var latest *skipchain.SkipBlock
		for _, dblk := range s.storage.DarcBlocks {
			if dblk.LatestSkipblock.Hash.Equal(sb.BackLinkIDs[0]) {
				latest = dblk.LatestSkipblock
			}
		}
		if latest == nil {
			// If we don't have the block, the leader should have it.
	        // also: since we don't have the latest block, we must update our
            // collectionDB.
            var err error
            recentBytes, err := s.skipchain.getUpdateChain(skipchain.GetUpdateChain{
                LatestID: s.storage.DarcBlocks[idStr].LatestSkipblock.SkipBlockID
            })
            if err != nil {
                return errors.New("Could not update skipchain")
            }
            recentBlocks, ok = recentBytes.([]*SkiprBblock)
            if !ok {
			    return fmt.Errorf("got block-type %s", reflect.TypeOf(dataInt))
            }
            if len(recentBlocks) == 0 {
                return errors.New("Did not get any recent blocks")
            }
            if recentBlocks[0].BackLinkIDs[0] != s.storage.DarcBlocks[idStr].LatestSkipblock.SkipBlockID {
                return errors.New("Unmatching skipblock ID")
            }

            latest = recentBlocks[len(recentBlocks)-1]

            // TODO: Make this less ugly
            for _, b := range recentBlocks {
                if err := b.VerifyForwardSignatures(), err != nil {
                    return err
                }
                for _, sigs := range b.Data.Sigs {
                    skv, ok := sigs.Request.Message.(lleap.SetKeyValue)
                    if ok {
                      s.collectionDB[idStr].Add(skv.Key, skv.Value)
                    }
                }
            }

            // check merkle root
            if s.collectionDB[idStr].GetRoot() != latest.Data.MerkleRoot {
                return errors.New("local merkle root is not equal to merkle
                                    root in latest skipblock")
            }
            // now our chain and our collection are up to date, so we can
            // start to actually do the verification


            /*
			latest, err = s.skipchain.GetSingleBlock(sb.Roster, sb.BackLinkIDs[0])
			if err != nil {
				return err
			}
			if latest == nil {
				// Block is not here and not with the leader.
				return errors.New("didn't find latest block")


			}
            */
		}
        // func Unmarshal(buf []byte, suite Suite) (MessageTypeID, Message, error)
        // thus dataInt : Message, and Message = interface{}
		_, dataInt, err = network.Unmarshal(latest.Data, s.Suite())
		if err != nil {
			return err
		}
        // latest : SkipBlock, so dataInt : []byte
		// dataLatest := dataInt.(*Data) is a type assertion, returning a value
        // of type *Data or -if not possible- panicking.
        // dataLatest is the data of the latest skipblock which has been
        // added to the skipchain, whereas data is the data contained in the
        // skipblock to be added
		dataLatest := dataInt.(*Data)
        col := s.collectionDB[idStr]
        for _, sig := range data.Sigs {
            if err := col.verify(sig), err != nil {
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
	}()
	if err != nil {
		log.Lvl2("Error while validating block:", err)
		return false
	}
	return true
}

func (darcs *collectionDB) findDarc (darcid darc.ID) (*darc.Darc, error) {
    darcKey = append(darcBytes, darcid...)
    d, err := darcs.Get(darcKey).Record()
    if err != nil {
        log.lvl2("Error while getting record from collection:", err)
        return nil, err
    }
    if !d.Match() {
        log.lvl2("No match found for darc key.", err)
        return nil, errors.New("Could not find match.")
    }
    values := d.Values()
    // TODO: Check the (unlikely) case that len(values) > 1
    if len(values) < 1 {
        log.lvl2("Empty slice for darcid:", darcKey)
        return nil, errors.New("Found empty slice as value for darc key")
    }
    resultDarc, ok := values[0].(darc.Darc)
    if !ok {
        return nil, fmt.Errorf("got data-type %s", reflect.TypeOf(resultDarc))
    }
    return resultDarc, nil
}

func (darcs *collectionDB) getPath(sig *darc.Signature) {
    //Find Darc from request DarcID
	targetDarc, err := darcs.findDarc(sig.Request.DarcID)
	if err != nil {
		return err
	}
	rules := *targetDarc.Rules
	targetRule, err := darc.FindRule(rules, req.RuleID)
	if err != nil {
		return err
	}
	signer := sig.Signer
	subs := *targetRule.Subjects
	var pathIndex []int
	_, err = .darcs.findSubject(subs, &Subject{PK: &signer}, pathIndex)
	//fmt.Println(pa)
	return err

}
func (darcs *collectionDB) findSubject(subjects []*Subject, requester *Subject,
                                        pathIndex []int) ([]int, error) {
    for i, s := range subjects {
		if CompareSubjects(s, requester) == true {
			pathIndex = append(pathIndex, i)
			return pathIndex, nil
		} else if s.Darc != nil {
			targetDarc, err := darcs.FindDarc(s.Darc.ID)
			if err != nil {
				return nil, err
			}
			ruleind, err := darcs.FindUserRuleIndex(*targetDarc.Rules)
			if err != nil {
				return nil, errors.New("User rule ID not found")
			}
			subs := *(*targetDarc.Rules)[ruleind].Subjects
			pathIndex = append(pathIndex, i)
			pa, err := darcs.findSubject(subs, requester, pathIndex)
			if err != nil {
				pathIndex = pathIndex[:len(pathIndex)-1]
			} else {
				return pa, nil
			}
		}
	}
	return nil, errors.New("Subject not found")
}

func (darcs *collectionDB) verify(sig *darc.Signature) error {
    if sig == nil || len(sig.Signature) == 0 {
		return errors.New("No signature available")
	}
	rc := sig.Request.CopyReq()
	b, err := protobuf.Encode(rc)
	if err != nil {
		return err
	}
	if b == nil {
		return errors.New("nothing to verify, message is empty")
	}
	pub := sig.Signer.Point
	err = sign.VerifySchnorr(network.Suite, pub, b, sig.Signature)
	if err != nil {
		return err
	}
	//Check if path from rule to signer is correct
	err = darcs.getPath(darcs, req, sig)
	if err != nil {
		return err
	}
	return err

}

