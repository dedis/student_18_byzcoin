// Conode is the main binary for running a Cothority server.
// A conode can participate in various distributed protocols using the
// *onet* library as a network and overlay library and the *dedis/crypto*
// library for all cryptographic primitives.
// Basically, you first need to setup a config file for the server by using:
//
//  ./conode setup
//
// Then you can launch the daemon with:
//
//  ./conode
//
package main

import (
	// Here you can import any other needed service for your conode.
	_ "github.com/dedis/cothority/cosi/service"
	_ "github.com/dedis/cothority/status/service"
	_ "github.com/dedis/cothority_template/service"
	"gopkg.in/dedis/onet.v1/app"
)

func main() {
	app.Server()
}
