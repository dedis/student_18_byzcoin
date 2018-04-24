package service

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	bolt "github.com/coreos/bbolt"
	"github.com/stretchr/testify/require"
	"gopkg.in/dedis/cothority.v2"
	"gopkg.in/dedis/onet.v2"
	"gopkg.in/dedis/onet.v2/log"
)

var testName = []byte("coll1")

func TestCollectionDBStrange(t *testing.T) {
	tmpDB, err := ioutil.TempFile("", "tmpDB")
	require.Nil(t, err)
	tmpDB.Close()
	defer os.Remove(tmpDB.Name())

	db, err := bolt.Open(tmpDB.Name(), 0600, nil)
	require.Nil(t, err)

	cdb := newCollectionDB(db, testName)
	key := []byte("first")
	value := []byte("value")
	kind := []byte("mykind")
	err = cdb.Store(&Transaction{
		Key:   key,
		Value: value,
		Kind:  kind,
	})
	require.Nil(t, err)
	v, k, err := cdb.GetValueKind([]byte("first"))
	require.Nil(t, err)
	require.Equal(t, value, v)
	require.Equal(t, kind, k)
}

// TODO: Test good case, bad add case, bad remove case
func TestCollectionDBtryHash(t *testing.T) {
	tmpDB, err := ioutil.TempFile("", "tmpDB")
	require.Nil(t, err)
	tmpDB.Close()
	defer os.Remove(tmpDB.Name())

	db, err := bolt.Open(tmpDB.Name(), 0600, nil)
	require.Nil(t, err)

	cdb := newCollectionDB(db, testName)
	ts := []Transaction{
		Transaction{
			Key:   []byte("key1"),
			Kind:  []byte("kind1"),
			Value: []byte("value1"),
		},
		Transaction{
			Key:   []byte("key2"),
			Kind:  []byte("kind2"),
			Value: []byte("value2"),
		},
	}
	mrTrial, err := cdb.tryHash(ts)
	require.Nil(t, err)
	_, _, err = cdb.GetValueKind([]byte("key1"))
	require.EqualError(t, err, "no match found")
	_, _, err = cdb.GetValueKind([]byte("key2"))
	require.EqualError(t, err, "no match found")
	cdb.Store(&ts[0])
	cdb.Store(&ts[1])
	mrReal := cdb.RootHash()
	require.Equal(t, mrTrial, mrReal)
}

func TestCollectionDB(t *testing.T) {
	kvPairs := 16

	tmpDB, err := ioutil.TempFile("", "tmpDB")
	require.Nil(t, err)
	tmpDB.Close()
	defer os.Remove(tmpDB.Name())

	db, err := bolt.Open(tmpDB.Name(), 0600, nil)
	require.Nil(t, err)

	cdb := newCollectionDB(db, testName)
	pairs := map[string]string{}
	mykind := []byte("mykind")
	for i := 0; i < kvPairs; i++ {
		pairs[fmt.Sprintf("Key%d", i)] = fmt.Sprintf("value%d", i)
	}

	// Store all key/value pairs
	for k, v := range pairs {
		tr := &Transaction{
			Key:   []byte(k),
			Value: []byte(v),
			Kind:  mykind,
		}
		require.Nil(t, cdb.Store(tr))
	}

	// Verify it's all there
	for k, v := range pairs {
		stored, kind, err := cdb.GetValueKind([]byte(k))
		require.Nil(t, err)
		require.Equal(t, v, string(stored))
		require.Equal(t, mykind, kind)
	}

	// Get a new db handler
	cdb2 := newCollectionDB(db, testName)

	// Verify it's all there
	for k, v := range pairs {
		stored, _, err := cdb2.GetValueKind([]byte(k))
		require.Nil(t, err)
		require.Equal(t, v, string(stored))
	}
}

func TestService_Store(t *testing.T) {
	kvPairs := 2
	pairs := map[string][]byte{}

	// First create a roster to attach the data to it
	local := onet.NewLocalTest(cothority.Suite)
	defer local.CloseAll()
	var genService onet.Service
	_, roster, genService := local.MakeSRS(cothority.Suite, 4, lleapID)
	service := genService.(*Service)

	// Create a new skipchain
	resp, err := service.CreateSkipchain(&CreateSkipchain{
		Version: CurrentVersion,
		Roster:  *roster,
		Transaction: Transaction{
			Key: []byte("123"),
		},
	})
	require.Nil(t, err)
	genesis := resp.Skipblock

	// Store some keypairs
	for i := 0; i < kvPairs; i++ {
		key := []byte(fmt.Sprintf("Key%d", i))
		value := []byte(fmt.Sprintf("value%d", i))
		pairs[string(key)] = value
		_, err := service.SetKeyValue(&SetKeyValue{
			Version:     CurrentVersion,
			SkipchainID: genesis.Hash,
			Transaction: Transaction{
				Key:   key,
				Kind:  []byte("testKind"),
				Value: value,
			},
		})
		require.Nil(t, err)
	}
	time.Sleep(4 * waitQueueing)

	// Retrieve the keypairs
	for key, value := range pairs {
		gvResp, err := service.GetProof(&GetProof{
			Version: CurrentVersion,
			ID:      genesis.Hash,
			Key:     []byte(key),
		})
		require.Nil(t, err)
		_, vs, err := gvResp.Proof.KeyValue()
		require.Nil(t, err)
		require.Equal(t, 0, bytes.Compare(value, vs[0]))
	}

	// Now read the key/values from a new service
	// First create a roster to attach the data to it
	log.Lvl1("Recreate services and fetch keys again")
	service.tryLoad()

	// Retrieve the keypairs
	for key, value := range pairs {
		gvResp, err := service.GetProof(&GetProof{
			Version: CurrentVersion,
			ID:      genesis.Hash,
			Key:     []byte(key),
		})
		require.Nil(t, err)
		_, vs, err := gvResp.Proof.KeyValue()
		require.Nil(t, err)
		require.Equal(t, 0, bytes.Compare(value, vs[0]))
	}
}
