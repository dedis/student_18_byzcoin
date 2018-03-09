# LLEAP

LLEAP is a project to proof authenticity of diplomas through the use of a
blockchain. For this project we use the skipchain from the dedis-lab:
https://github.com/dedis/cothority/tree/master/skipchain .

Internally it uses a merkle-tree that is implemented in the collection-library
to store the key/value pairs and then only stores the new key/value pairs on
the skipchain and the hash of the root node of the merkle-tree.

Directories:
- lleap/ - defines the golang-api to communicate to the lleap-service
- lleap/service - handles api-calls and uses the cisc-service to store
  key/value pairs
- lleap/collection - library used to create the merkle-tree
- lleap/conode - the server application running on the dedis-servers
- lleap/app - command line interface to create new skipchains and to store the
  public key for authentication when storing key/value pairs
- lleap/external/java - a java api to interact with the lleap service and
  store key/value pairs

Roadmap:
Date - version# - short description
February 2nd: 1 - first beta version
February 9th: 2 - adding collections for better data storage
February 16th: 2 - documentation
