# Web-interface to cothority

The cothority-services can interact using protobuf over websockets with other
languages. This directory shows how to use javascript to interact with
services.

In `index.html` you can find an example how to contact the service from
javascript. The `js/`-directory has the needed libraries for the communication.
`js/bundle.js` is a compilation of the `src/`-directory where the protobuf-
definitions are stored.

## Updating `js/bundle.js`

If you change the protobuf-files or add new ones, you need to compile them
so they're available under javascript.
The protobuf-files are stored under `src/models/`. if you add a new file,
it will be automatically picked up by the compilation-scipt.
To compile all protobuf-files to `js/bundle.js`, launche the following:

```bash
cd src
make
```

This supposes you have `node` and `npm` installed, and will create a new
`js/bundle.js`.

## `CothorityMessages`

The main class in javascript that contains helper-functions for every
method of the service-api. It is not created automatically. So if you
add new proto-files or new messages to it, you need to extend it manually.
The class is defined in `src/index.js` and has some helper methods:

### createSocket

`createSocket` is a simple method to encode javascript-objects using
protobuf and send it through websockets to the service.

### toml_to_roster

Converts a toml-string of public.toml to a roster that can be sent
to a service. Also calculates the Id of the ServerIdentities.

### si_to_ws

Returns a websocket-url from a ServerIdentity.

