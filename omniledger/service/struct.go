package service

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"sort"

	bolt "github.com/coreos/bbolt"
	"github.com/dedis/student_18_omniledger/omniledger/collection"
	"github.com/dedis/student_18_omniledger/omniledger/darc"
	"gopkg.in/dedis/cothority.v2"
	"gopkg.in/dedis/cothority.v2/skipchain"
	"gopkg.in/dedis/onet.v2/network"
)

func init() {
	network.RegisterMessages(&darc.Signature{},
		DataHeader{}, DataBody{})
}

type collectionDB struct {
	db         *bolt.DB
	bucketName []byte
	coll       collection.Collection
}

// OmniledgerClass is the type signature of the class functions
// which can be registered with the omniledger service.
// Since the outcome of the verification depends on the state of the collection
// which is to be modified, we pass it as a pointer here.
type OmniledgerClass func(cdb collection.Collection, tx Instruction, kind string, state []byte) ([]StateChange, error)

// newCollectionDB initialises a structure and reads all key/value pairs to store
// it in the collection.
func newCollectionDB(db *bolt.DB, name []byte) *collectionDB {
	c := &collectionDB{
		db:         db,
		bucketName: name,
		coll:       collection.New(collection.Data{}, collection.Data{}),
	}
	c.db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucket(name)
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		return nil
	})
	c.loadAll()
	// TODO: Check the merkle tree root.
	return c
}

func (c *collectionDB) loadAll() {
	c.db.View(func(tx *bolt.Tx) error {
		// Assume bucket exists and has keys
		b := tx.Bucket([]byte(c.bucketName))
		cur := b.Cursor()

		for k, v := cur.First(); k != nil; k, v = cur.Next() {
			sig := b.Get(append(k, []byte("sig")...))
			c.coll.Add(k, v, sig)
		}

		return nil
	})
}

func (c *collectionDB) Store(t *StateChange) error {
	c.coll.Add(t.Key, t.Value, t.Kind)
	err := c.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(c.bucketName))
		if err := bucket.Put(t.Key, t.Value); err != nil {
			return err
		}
		keykind := make([]byte, len(t.Key)+4)
		copy(keykind, t.Key)
		keykind = append(keykind, []byte("kind")...)
		if err := bucket.Put(keykind, t.Kind); err != nil {
			return err
		}
		return nil
	})
	return err
}

func (c *collectionDB) GetValueKind(key []byte) (value, kind []byte, err error) {
	proof, err := c.coll.Get(key).Record()
	if err != nil {
		return
	}
	hashes, err := proof.Values()
	if err != nil {
		return
	}
	if len(hashes) == 0 {
		err = errors.New("nothing stored under that key")
		return
	}
	value, ok := hashes[0].([]byte)
	if !ok {
		err = errors.New("the value is not of type []byte")
		return
	}
	kind, ok = hashes[1].([]byte)
	if !ok {
		err = errors.New("the signature is not of type []byte")
		return
	}
	return
}

// RootHash returns the hash of the root node in the merkle tree.
func (c *collectionDB) RootHash() []byte {
	return c.coll.GetRoot()
}

// tryHash returns the merkle root of the collection as if the key value pairs
// in the transactions had been added, without actually adding it.
func (c *collectionDB) tryHash(ts []OmniledgerTransaction) (mr []byte, rerr error) {
	for _, t := range ts {
		for _, sc := range t.StateChanges {
			err := c.coll.Add(sc.Key, sc.Value, sc.Kind)
			if err != nil {
				rerr = err
				return
			}
			// remove the pair after we got the merkle root.
			defer func(k []byte) {
				err = c.coll.Remove(k)
				if err != nil {
					rerr = err
					mr = nil
				}
			}(sc.Key)
		}
	}
	mr = c.coll.GetRoot()
	return
}

// RegisterVerification stores the verification in a map and will
// call it whenever a verification needs to be done.
// GetService makes it possible to give either an `onet.Context` or
// `onet.Server` to `RegisterVerification`.
func RegisterVerification(s skipchain.GetService, kind string, f OmniledgerClass) error {
	scs := s.Service(ServiceName)
	if scs == nil {
		return errors.New("Didn't find our service: " + ServiceName)
	}
	return scs.(*Service).registerVerification(kind, f)
}

// DataHeader is the data passed to the Skipchain
type DataHeader struct {
	// CollectionRoot is the root of the merkle tree of the colleciton after
	// applying the valid transactions.
	CollectionRoot []byte
	// TransactionHash is the sha256 hash of all the transactions in the body
	TransactionHash []byte
	// Timestamp is a unix timestamp in nanoseconds.
	Timestamp int64
}

// DataBody is stored in the body of the skipblock but is not hashed. This reduces
// the proof needed for a key/value pair.
type DataBody struct {
	Transactions []OmniledgerTransaction
}

// sortWithSalt sorts transactions according to their salted hash:
// The salt is prepended to the transactions []byte representation
// and this concatenation is hashed then.
// Using a salt here makes the resulting order of the transactions
// harder to guess.
func sortWithSalt(ts [][]byte, salt []byte) {
	less := func(i, j int) bool {
		h1 := sha256.Sum256(append(salt, ts[i]...))
		h2 := sha256.Sum256(append(salt, ts[j]...))
		return bytes.Compare(h1[:], h2[:]) == -1
	}
	sort.Slice(ts, less)
}

// sortTransactions needs to marshal transactions, if it fails to do so,
// it returns an error and leaves the slice unchange.
// The helper functions (sortWithSalt, xorTransactions) operate on []byte
// representations directly. This allows for some more compact error handling
// when (un)marshalling.
func sortTransactions(ts []ClientTransaction) error {
	bs := make([][]byte, len(ts))
	sortedTs := make([]*ClientTransaction, len(ts))
	var err error
	var ok bool
	for i := range ts {
		bs[i], err = network.Marshal(&ts[i])
		if err != nil {
			return err
		}
	}
	// An alternative to XOR-ing the transactions would have been to
	// concatenate them and hash the result. However, if we generate the salt
	// as the hash of the concatenation of the transactions, we have to
	// concatenate them in a specific order to be deterministic.
	// This means we would have to sort them, just to get the salt.
	// In order to avoid this, we XOR them.
	salt := xorTransactions(bs)
	sortWithSalt(bs, salt)
	for i := range bs {
		_, tmp, err := network.Unmarshal(bs[i], cothority.Suite)
		if err != nil {
			return err
		}
		sortedTs[i], ok = tmp.(*ClientTransaction)
		if !ok {
			return errors.New("Data of wrong type")
		}
	}
	for i := range sortedTs {
		ts[i] = *sortedTs[i]
	}
	return nil
}

// xorTransactions returns the XOR of the hash values of all the transactions.
func xorTransactions(ts [][]byte) []byte {
	result := make([]byte, sha256.Size)
	for _, t := range ts {
		hs := sha256.Sum256(t)
		for i := range result {
			result[i] = result[i] ^ hs[i]
		}
	}
	return result
}
