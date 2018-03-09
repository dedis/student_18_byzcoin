# Conode

This package implements the cothority server. Conodes are linked together to form cothorities, run decentralized protocols, and offer services to clients.

## Getting Started

To use the code of this package you need to:

-  Install [Golang](https://golang.org/doc/install)
-  Optional: Set [`$GOPATH`](https://golang.org/doc/code.html#GOPATH) to point to your Go workspace directory 
-  Add `$(go env GOPATH)/bin` to `$PATH` 

## run_conode.sh

The simplest way of using the conode is through the `run_conode.sh`-script. You can
run it either in local-mode for local testing, or in public-mode on a server
with a public IP-address.

### local mode

When running in local mode, `run_conode.sh` will create one directory for each
node you ask it to run. It is best if you create a new directory and run
the script from there:

```bash
cd ~
mkdir myconodes
$(go env GOPATH)/src/github.com/dedis/cothority_template/conode/run_conode.sh local 3
```

This will create three nodes and configure them with default values, then run
them in background. To check if they're running correctly, use:

```bash
$(go env GOPATH)/src/github.com/dedis/cothority_template/conode/run_conode.sh check
```

If you need some debugging information, you can add another argument to print
few (1), reasonable (3) or lots (5) information:

```bash
$(go env GOPATH)/src/github.com/dedis/cothority_template/conode/run_conode.sh local 3 3
```

The file `public.toml` contains the definition of all nodes that are being run.

### public mode

If you have a public server and want to run a node on it, simply use:

```bash
$(go env GOPATH)/src/github.com/dedis/cothority_template/conode/run_conode.sh public
```

The first time this runs it will ask you a couple of questions and verify if
the node is available from the internet. If you plan to run a node for a long
time, be sure to contact us at dedis@epfl.ch!