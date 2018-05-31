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
	"os"
	"path"

	"gopkg.in/dedis/onet.v2/app"
	"gopkg.in/dedis/onet.v2/cfgpath"
	"gopkg.in/dedis/onet.v2/log"
	cli "gopkg.in/urfave/cli.v1"

	"gopkg.in/dedis/cothority.v2"
	// Import your service:
	_ "github.com/dedis/student_18_omniledger/omniledger/service"
	// Here you can import any other needed service for your conode.
	// For example, if your service needs cosi available in the server
	// as well, uncomment this:
	_ "gopkg.in/dedis/cothority.v2/skipchain"
	_ "gopkg.in/dedis/cothority.v2/status/service"
)

func main() {
	cliApp := cli.NewApp()
	cliApp.Name = "omniledger"
	cliApp.Usage = "basic file for an app"
	cliApp.Version = "0.1"

	cliApp.Commands = []cli.Command{
		{
			Name:    "setup",
			Aliases: []string{"s"},
			Usage:   "Setup server configuration (interactive)",
			Action: func(c *cli.Context) error {
				if c.String("config") != "" {
					log.Fatal("[-] Configuration file option cannot be used for the 'setup' command")
				}
				if c.String("debug") != "" {
					log.Fatal("[-] Debug option cannot be used for the 'setup' command")
				}
				app.InteractiveConfig(cothority.Suite, "omniledger")
				return nil
			},
		},
		{
			Name:  "server",
			Usage: "Start cothority server",
			Action: func(c *cli.Context) {
				runServer(c)
			},
		},
	}
	cliApp.Flags = []cli.Flag{
		cli.IntFlag{
			Name:  "debug, d",
			Value: 0,
			Usage: "debug-level: 1 for terse, 5 for maximal",
		},
		cli.StringFlag{
			Name:  "config, c",
			Value: path.Join(cfgpath.GetConfigPath("omniledger"), "config.bin"),
			Usage: "Configuration file of the server",
		},
	}
	cliApp.Before = func(c *cli.Context) error {
		log.SetDebugVisible(c.Int("debug"))
		return nil
	}

	log.ErrFatal(cliApp.Run(os.Args))
}

func runServer(c *cli.Context) error {
	app.RunServer(c.GlobalString("config"))
	return nil
}
