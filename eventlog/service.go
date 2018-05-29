package eventlog

import (
	"encoding/binary"
	"errors"

	"github.com/dedis/protobuf"
	"github.com/dedis/student_18_omniledger/omniledger/collection"
	omniledger "github.com/dedis/student_18_omniledger/omniledger/service"
	"gopkg.in/dedis/onet.v2"
	"gopkg.in/dedis/onet.v2/log"
)

// ServiceName is the service name for the EventLog service.
var ServiceName = "EventLog"

var sid onet.ServiceID

var indexKey omniledger.ObjectID

func init() {
	var err error
	sid, err = onet.RegisterNewService(ServiceName, newService)
	if err != nil {
		log.Fatal(err)
	}

	// TODO: This is horrible, we'll be somehow using better well known keys later. Maybe.
	indexKey.DarcID = omniledger.ZeroDarc
	var keyNonce = omniledger.ZeroNonce
	copy(keyNonce[:], []byte("index"))
	indexKey.InstanceID = keyNonce
}

// Service is the EventLog service.
type Service struct {
	*onet.ServiceProcessor
	omni       *omniledger.Service
	bucketSize int
}

// Init will create a new event log. Logs will be accepted
// from the signers mentioned in the request.
func (s *Service) Init(req *InitRequest) (*InitResponse, error) {
	cg := &omniledger.CreateGenesisBlock{
		Version:     omniledger.CurrentVersion,
		GenesisDarc: req.Owner,
		Roster:      req.Roster,
	}
	cgr, err := s.omni.CreateGenesisBlock(cg)
	if err != nil {
		return nil, err
	}

	return &InitResponse{
		ID: cgr.Skipblock.Hash,
	}, nil
}

// Log will create a new event log entry.
func (s *Service) Log(req *LogRequest) (*LogResponse, error) {
	req2 := &omniledger.AddTxRequest{
		Version:     omniledger.CurrentVersion,
		SkipchainID: req.SkipchainID,
		Transaction: req.Transaction,
	}
	_, err := s.omni.AddTransaction(req2)
	if err != nil {
		return nil, err
	}
	return &LogResponse{}, nil
}

const contractName = "eventlog"

type bucketNonce struct {
	nonce [32]byte
}

func newBucketNonce() bucketNonce {
	// TODO write a proper way to increment slices
	x := uint64(8) // needs to be the same for every node
	var nonce [32]byte
	binary.PutUvarint(nonce[:], x)
	return bucketNonce{nonce}
}

func (n bucketNonce) increment() bucketNonce {
	buf := make([]byte, binary.MaxVarintLen64)
	x, _ := binary.Uvarint(buf)
	x++

	binary.PutUvarint(buf, x)
	copy(n.nonce[:], buf)
	return n
}

// contractFunction is the function that runs to process a transaction of
// type "eventlog"
func (s *Service) contractFunction(coll collection.Collection, tx omniledger.Instruction, c []omniledger.Coin) ([]omniledger.StateChange, []omniledger.Coin, error) {
	// Client only submits spawn transactions, updates on the index and
	// bucket are handled by this smart contract.
	if tx.Spawn == nil {
		return nil, nil, errors.New("expected a spawn tx")
	}

	// This is not strictly required, because since we know we are
	// a spawn, we know the contract comes directly from
	// tx.Spawn.ContractID.
	cid, _, err := tx.GetContractState(coll)
	if err != nil {
		return nil, nil, err
	}

	// All the state changes, at every step, go in here.
	scs := []omniledger.StateChange{}

	// Add the event itself.
	eventBuf := tx.Spawn.Args.Search("event")
	if eventBuf == nil {
		return nil, nil, errors.New("expected a named argument of \"event\"")
	}
	var event Event
	err = protobuf.Decode(eventBuf, &event)
	if err != nil {
		return nil, nil, err
	}

	scs = append(scs, omniledger.NewStateChange(omniledger.Create, tx.ObjectID, cid, eventBuf))

	// Get the bucket either update the existing one or create a new one,
	// depending on the bucket size.
	bIDCopy, b, err := getLatestBucket(coll)
	bID := append([]byte{}, bIDCopy...)
	if err == errIndexMissing {
		// TODO which darc do we use for the buckets?
		bID = omniledger.ObjectID{
			DarcID:     tx.ObjectID.DarcID,
			InstanceID: omniledger.Nonce(newBucketNonce().nonce),
		}.Slice()
		b = &bucket{
			Start:     event.Timestamp,
			End:       event.Timestamp,
			Prev:      []byte{},
			EventRefs: [][]byte{tx.ObjectID.Slice()},
		}
		bBuf, err := protobuf.Encode(b)
		if err != nil {
			return nil, nil, err
		}
		scs = append(scs, omniledger.StateChange{
			StateAction: omniledger.Create,
			ObjectID:    bID,
			ContractID:  []byte(cid),
			Value:       bBuf,
		})
	} else if err != nil {
		return nil, nil, err
	} else if len(b.EventRefs) >= s.bucketSize {
		// TODO which darc do we use for the buckets?
		var oldNonce [32]byte
		copy(oldNonce[:], bID[32:64])
		newNonce := bucketNonce{oldNonce}.increment()

		newbID := append([]byte{}, bID[0:32]...)
		newbID = append(newbID, newNonce.nonce[:]...)

		scsNew, newBucket, err := b.newLink(bID, newbID, tx.ObjectID.Slice())
		if err != nil {
			return nil, nil, err
		}
		scs = append(scs, scsNew...)
		bID = newbID
		b = newBucket
	} else {
		scsNew, err := b.updateBucket(bID, tx.ObjectID.Slice(), event)
		if err != nil {
			return nil, nil, err
		}
		scs = append(scs, scsNew...)
	}

	// Try to update the index, otherwise create it.
	_, err = getIndexValue(coll)
	if err == errIndexMissing {
		scs = append(scs, omniledger.NewStateChange(omniledger.Create, indexKey, cid, bID))
	} else if err == nil {
		scs = append(scs, omniledger.NewStateChange(omniledger.Update, indexKey, cid, bID))
	} else {
		return nil, nil, err
	}

	return scs, nil, nil
}

// newService receives the context that holds information about the node it's
// running on. Saving and loading can be done using the context. The data will
// be stored in memory for tests and simulations, and on disk for real
// deployments.
func newService(c *onet.Context) (onet.Service, error) {
	s := &Service{
		ServiceProcessor: onet.NewServiceProcessor(c),
		omni:             c.Service(omniledger.ServiceName).(*omniledger.Service),
		bucketSize:       10,
	}
	if err := s.RegisterHandlers(s.Init, s.Log); err != nil {
		log.ErrFatal(err, "Couldn't register messages")
	}

	omniledger.RegisterContract(s, contractName, s.contractFunction)
	return s, nil
}