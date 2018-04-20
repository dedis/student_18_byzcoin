package service

/*
This holds the messages used to communicate with the service over the network.
*/

import (
	"gopkg.in/dedis/cothority.v2/skipchain"
	"gopkg.in/dedis/onet.v2"
	"gopkg.in/dedis/onet.v2/network"
)

// We need to register all messages so the network knows how to handle them.
func init() {
	network.RegisterMessages(
		&CreateSkipchain{}, &CreateSkipchainResponse{},
		&SetKeyValue{}, &SetKeyValueResponse{},
	)
}

const (
	// ErrorParse indicates an error while parsing the protobuf-file.
	ErrorParse = iota + 4000
)

// Version indicates what version this client runs. In the first development
// phase, each next version will break the preceeding versions. Later on,
// new versions might correctly interpret earlier versions.
type Version int

// CurrentVersion is what we're running now
const CurrentVersion Version = 1

// PROTOSTART
// import "skipblock.proto";
// import "roster.proto";
//
// option java_package = "ch.epfl.dedis.proto";
// option java_outer_classname = "LleapProto";

// ***
// These are the messages used in the API-calls
// ***

// CreateSkipchain asks the cisc-service to set up a new skipchain.
type CreateSkipchain struct {
	// Version of the protocol
	Version Version
	// Roster defines which nodes participate in the skipchain.
	Roster onet.Roster
	// Transaction contains the master darc which defines who is allowed to
	// write to this skipchain. we will only store its hash.
	Transaction Transaction
}

// CreateSkipchainResponse holds the genesis-block of the new skipchain.
type CreateSkipchainResponse struct {
	// Version of the protocol
	Version Version
	// Skipblock of the created skipchain or empty if there was an error.
	Skipblock *skipchain.SkipBlock
}

// SetKeyValue asks for inclusion for a new key/value pair. The value needs
// to be signed by one of the Writers from the createSkipchain call.
type SetKeyValue struct {
	// Version of the protocol
	Version Version
	// SkipchainID is the hash of the first skipblock
	SkipchainID skipchain.SkipBlockID
	// Transaction to be apllied to the kv-store
	Transaction Transaction
}

// SetKeyValueResponse gives the timestamp and the skipblock-id
type SetKeyValueResponse struct {
	// Version of the protocol
	Version Version
	// QueueLength indicates how many transactions are waiting
	QueueLength int
}

// GetProof returns the proof that the given key is in the collection.
type GetProof struct {
	// Version of the protocol
	Version Version
	// Key is the key we want to look up
	Key []byte
	// ID is any block that is know to us in the skipchain, can be the genesis
	// block or any later block. The proof returned will be starting at this block.
	ID skipchain.SkipBlockID
}

// GetProofResponse can be used together with the Genesis block to proof that
// the returned key/value pair is in the collection.
type GetProofResponse struct {
	// Version of the protocol
	Version Version
	// Proof contains everything necessary to prove the inclusion
	// of the included key/value pair given a genesis skipblock.
	Proof Proof
}
