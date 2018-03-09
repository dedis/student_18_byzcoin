import CothorityProtobuf from './cothority-protobuf'

/**
 * Helpers to encode and decode messages of the Cothority
 *
 * @author Gaylor Bosson (gaylor.bosson@epfl.ch)
 */
class CothorityMessages extends CothorityProtobuf {

    /**
     * Create an encoded message to make a ClockRequest to a cothority node
     * @param {Array} servers - list of ServerIdentity
     * @returns {*|Buffer|Uint8Array}
     */
    createClockRequest(servers) {
        const fields = {
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
    decodeClockResponse(response) {
        response = new Uint8Array(response);

        return this.decodeMessage('ClockResponse', response);
    }

    /**
     * Create an encoded message to make a CountRequest to a cothority node
     * @returns {*|Buffer|Uint8Array}
     */
    createCountRequest() {
        return this.encodeMessage('CountRequest', {});
    }

    /**
     * Return the decoded response of a CountRequest
     * @param {*|Buffer|Uint8Array} response - Response of the Cothority
     * @returns {*}
     */
    decodeCountResponse(response) {
        response = new Uint8Array(response);

        return this.decodeMessage('CountResponse', response);
    }

    /**
     * Create an encoded message to make a StatusRequest to a cothority node
     * @returns {*|Buffer|Uint8Array}
     */
    createStatusRequest() {
        return this.encodeMessage('StatusRequest', {});
    }

    /**
     * Return the decoded response of a StatusRequest
     * @param {*|Buffer|Uint8Array} response - Response of the Cothority
     * @returns {*}
     */
    decodeStatusResponse(response) {
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
    createSocket(socket, address, message, callback, error) {
        if (!socket){
            socket = {}
        }
        var sock = socket[address];
        if (!sock || sock.readyState > 2) {
            sock = new WebSocket(address);
            sock.binaryType = 'arraybuffer';
            socket[address] = sock
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
            sock.addEventListener('open',() => {
                sock.send(message);
            });
        }
        else {
            sock.send(message);
        }

        return socket;
    }

    /**
     * Converts an arraybuffer to a hex-string
     * @param {ArrayBuffer} buffer
     * @returns {string} hexified ArrayBuffer
     */
    buf2hex(buffer) { // buffer is an ArrayBuffer
        return Array.prototype.map.call(new Uint8Array(buffer), x => ('00' + x.toString(16)).slice(-2)).join('');
    }

    /**
     * Converts a toml-string of public.toml to a roster that can be sent
     * to a service. Also calculates the Id of the ServerIdentities.
     * @param {string} toml of public.toml
     * @returns {object} Roster-object
     */
    toml_to_roster(toml){
        var parsed = {};
        var b2h = this.buf2hex;
        try {
            parsed = topl.parse(toml)
            parsed.servers.forEach(function (el) {
                var pubstr = Uint8Array.from(atob(el.Public), c => c.charCodeAt(0));
                var url = "https://dedis.epfl.ch/id/" + b2h(pubstr);
                el.Id = new UUID(5, "ns:URL", url).export();
            })
        }
        catch(err){
        }
        return parsed;
    }

    /**
     * Returns a websocket-url from a ServerIdentity
     * @param {ServerIdentity} the serveridentity to convert to a websocket-url
     * @returns {string} the url
     */
    si_to_ws(si, path){
        var ip_port = si.Address.replace("tcp://", "").split(":");
        ip_port[1] = parseInt(ip_port[1]) + 1;
        return "ws://" + ip_port.join(":") + path;
    }
}


/**
 * Singleton
 */
export default new CothorityMessages();

