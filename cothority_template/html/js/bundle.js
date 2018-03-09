var CothorityProtobuf = (function (protobuf) {
'use strict';

protobuf = 'default' in protobuf ? protobuf['default'] : protobuf;

var skeleton = '{"nested":{"cothority":{},"ClockRequest":{"fields":{"Roster":{"rule":"required","type":"Roster","id":1}}},"ClockResponse":{"fields":{"Time":{"rule":"required","type":"double","id":1},"Children":{"rule":"required","type":"sint32","id":2}}},"CountRequest":{"fields":{}},"CountResponse":{"fields":{"Count":{"rule":"required","type":"sint32","id":1}}},"Roster":{"fields":{"Id":{"type":"bytes","id":1},"List":{"rule":"repeated","type":"ServerIdentity","id":2,"options":{"packed":false}},"Aggregate":{"type":"bytes","id":3}}},"ServerIdentity":{"fields":{"Public":{"type":"bytes","id":1},"Id":{"type":"bytes","id":2},"Address":{"rule":"required","type":"string","id":3},"Description":{"type":"string","id":4}}},"StatusRequest":{"fields":{}},"StatusResponse":{"fields":{"system":{"keyType":"string","type":"Status","id":1},"server":{"type":"ServerIdentity","id":2}},"nested":{"Status":{"fields":{"field":{"keyType":"string","type":"string","id":1}}}}}}}';

var Root = protobuf.Root;

/**
 * As we need to create a bundle, we cannot use the *.proto files and the a script will wrap
 * them in a skeleton file that contains the JSON representation that can be used in the js code
 */

var Root$1 = Root.fromJSON(JSON.parse(skeleton));

var classCallCheck = function (instance, Constructor) {
  if (!(instance instanceof Constructor)) {
    throw new TypeError("Cannot call a class as a function");
  }
};

var createClass = function () {
  function defineProperties(target, props) {
    for (var i = 0; i < props.length; i++) {
      var descriptor = props[i];
      descriptor.enumerable = descriptor.enumerable || false;
      descriptor.configurable = true;
      if ("value" in descriptor) descriptor.writable = true;
      Object.defineProperty(target, descriptor.key, descriptor);
    }
  }

  return function (Constructor, protoProps, staticProps) {
    if (protoProps) defineProperties(Constructor.prototype, protoProps);
    if (staticProps) defineProperties(Constructor, staticProps);
    return Constructor;
  };
}();









var inherits = function (subClass, superClass) {
  if (typeof superClass !== "function" && superClass !== null) {
    throw new TypeError("Super expression must either be null or a function, not " + typeof superClass);
  }

  subClass.prototype = Object.create(superClass && superClass.prototype, {
    constructor: {
      value: subClass,
      enumerable: false,
      writable: true,
      configurable: true
    }
  });
  if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass;
};











var possibleConstructorReturn = function (self, call) {
  if (!self) {
    throw new ReferenceError("this hasn't been initialised - super() hasn't been called");
  }

  return call && (typeof call === "object" || typeof call === "function") ? call : self;
};

/**
 * Base class for the protobuf library that provides helpers to encode and decode
 * messages according to a given model
 *
 * @author Gaylor Bosson (gaylor.bosson@epfl.ch)
 */

var CothorityProtobuf = function () {

  /**
   * @constructor
   */
  function CothorityProtobuf() {
    classCallCheck(this, CothorityProtobuf);

    this.root = Root$1;
  }

  /**
   * Encode a model to be transmitted over websocket
   * @param {String} name
   * @param {Object} fields
   * @returns {*|Buffer|Uint8Array}
   */


  createClass(CothorityProtobuf, [{
    key: 'encodeMessage',
    value: function encodeMessage(name, fields) {
      var model = this.getModel(name);

      // Create the message with the model
      var msg = model.create(fields);

      // Encode the message in a BufferArray
      return model.encode(msg).finish();
    }

    /**
     * Decode a message coming from a websocket
     * @param {String} name
     * @param {*|Buffer|Uint8Array} buffer
     */

  }, {
    key: 'decodeMessage',
    value: function decodeMessage(name, buffer) {
      var model = this.getModel(name);
      return model.decode(buffer);
    }

    /**
     * Return the protobuf loaded model
     * @param {String} name
     * @returns {ReflectionObject|?ReflectionObject|string}
     */

  }, {
    key: 'getModel',
    value: function getModel(name) {
      return this.root.lookup('' + name);
    }
  }]);
  return CothorityProtobuf;
}();

/**
 * Helpers to encode and decode messages of the Cothority
 *
 * @author Gaylor Bosson (gaylor.bosson@epfl.ch)
 */

var CothorityMessages = function (_CothorityProtobuf) {
    inherits(CothorityMessages, _CothorityProtobuf);

    function CothorityMessages() {
        classCallCheck(this, CothorityMessages);
        return possibleConstructorReturn(this, (CothorityMessages.__proto__ || Object.getPrototypeOf(CothorityMessages)).apply(this, arguments));
    }

    createClass(CothorityMessages, [{
        key: 'createClockRequest',


        /**
         * Create an encoded message to make a ClockRequest to a cothority node
         * @param {Array} servers - list of ServerIdentity
         * @returns {*|Buffer|Uint8Array}
         */
        value: function createClockRequest(servers) {
            var fields = {
                Roster: {
                    List: servers
                }
            };
            return this.encodeMessage('ClockRequest', fields);
        }

        /**
         * Return the decoded response of a ClockRequest
         * @param {*|Buffer|Uint8Array} response - Response of the Cothority
         * @returns {Object}
         */

    }, {
        key: 'decodeClockResponse',
        value: function decodeClockResponse(response) {
            response = new Uint8Array(response);

            return this.decodeMessage('ClockResponse', response);
        }

        /**
         * Create an encoded message to make a CountRequest to a cothority node
         * @returns {*|Buffer|Uint8Array}
         */

    }, {
        key: 'createCountRequest',
        value: function createCountRequest() {
            return this.encodeMessage('CountRequest', {});
        }

        /**
         * Return the decoded response of a CountRequest
         * @param {*|Buffer|Uint8Array} response - Response of the Cothority
         * @returns {*}
         */

    }, {
        key: 'decodeCountResponse',
        value: function decodeCountResponse(response) {
            response = new Uint8Array(response);

            return this.decodeMessage('CountResponse', response);
        }

        /**
         * Create an encoded message to make a StatusRequest to a cothority node
         * @returns {*|Buffer|Uint8Array}
         */

    }, {
        key: 'createStatusRequest',
        value: function createStatusRequest() {
            return this.encodeMessage('StatusRequest', {});
        }

        /**
         * Return the decoded response of a StatusRequest
         * @param {*|Buffer|Uint8Array} response - Response of the Cothority
         * @returns {*}
         */

    }, {
        key: 'decodeStatusResponse',
        value: function decodeStatusResponse(response) {
            response = new Uint8Array(response);

            return this.decodeMessage('StatusResponse', response);
        }

        /**
         * Use the existing socket or create a new one if required
         * @param socket - WebSocket-array
         * @param address - String ws address
         * @param message - ArrayBuffer the message to send
         * @param callback - Function callback when a message is received
         * @param error - Function callback if an error occurred
         * @returns {*}
         */

    }, {
        key: 'createSocket',
        value: function createSocket(socket, address, message, callback, error) {
            if (!socket) {
                socket = {};
            }
            var sock = socket[address];
            if (!sock || sock.readyState > 2) {
                sock = new WebSocket(address);
                sock.binaryType = 'arraybuffer';
                socket[address] = sock;
            }

            function onError(e) {
                sock.removeEventListener('error', onError);
                error(e);
            }
            sock.addEventListener('error', onError);

            function onMessage(e) {
                sock.removeEventListener('message', onMessage);
                callback(e.data);
            }
            sock.addEventListener('message', onMessage);

            if (sock.readyState === 0) {
                sock.addEventListener('open', function () {
                    sock.send(message);
                });
            } else {
                sock.send(message);
            }

            return socket;
        }

        /**
         * Converts an arraybuffer to a hex-string
         * @param {ArrayBuffer} buffer
         * @returns {string} hexified ArrayBuffer
         */

    }, {
        key: 'buf2hex',
        value: function buf2hex(buffer) {
            // buffer is an ArrayBuffer
            return Array.prototype.map.call(new Uint8Array(buffer), function (x) {
                return ('00' + x.toString(16)).slice(-2);
            }).join('');
        }

        /**
         * Converts a toml-string of public.toml to a roster that can be sent
         * to a service. Also calculates the Id of the ServerIdentities.
         * @param {string} toml of public.toml
         * @returns {object} Roster-object
         */

    }, {
        key: 'toml_to_roster',
        value: function toml_to_roster(toml) {
            var parsed = {};
            var b2h = this.buf2hex;
            try {
                parsed = topl.parse(toml);
                parsed.servers.forEach(function (el) {
                    var pubstr = Uint8Array.from(atob(el.Public), function (c) {
                        return c.charCodeAt(0);
                    });
                    var url = "https://dedis.epfl.ch/id/" + b2h(pubstr);
                    el.Id = new UUID(5, "ns:URL", url).export();
                });
            } catch (err) {}
            return parsed;
        }

        /**
         * Returns a websocket-url from a ServerIdentity
         * @param {ServerIdentity} the serveridentity to convert to a websocket-url
         * @returns {string} the url
         */

    }, {
        key: 'si_to_ws',
        value: function si_to_ws(si, path) {
            var ip_port = si.Address.replace("tcp://", "").split(":");
            ip_port[1] = parseInt(ip_port[1]) + 1;
            return "ws://" + ip_port.join(":") + path;
        }
    }]);
    return CothorityMessages;
}(CothorityProtobuf);

/**
 * Singleton
 */


var index = new CothorityMessages();

return index;

}(protobuf));
