package ch.epfl.dedis.lleap;

import ch.epfl.dedis.lib.Roster;
import ch.epfl.dedis.lib.ServerIdentity;
import ch.epfl.dedis.lib.SkipblockId;
import ch.epfl.dedis.lib.exception.CothorityCommunicationException;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.proto.LleapProto;
import com.google.protobuf.ByteString;
import com.google.protobuf.InvalidProtocolBufferException;
import javafx.util.Pair;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import javax.xml.bind.DatatypeConverter;
import java.security.*;

/**
 * SkipchainRPC offers a reliable, fork-resistant storage of key/value pairs. This class connects to a
 * lleap service that uses the skipchain service (https://github.com/dedis/cothority/tree/master/skipchain)
 * from the cothority.
 * The skipchain service only stores arrays of data in each new skipblock. It is the Lleap service that
 * keeps the key/value pairs in a database, creates a merkle tree out of the key/value pairs and stores
 * the hash of the root-node together with the new key/value pairs in each new skipblock.
 * <p>
 * When creating a new skipchain, a public key is stored in the first block of the skipchain.
 * Every time a new key/value pair is stored, it needs to be signed by the corresponding private key
 * to proof that the writer does have access. The Lleap service stores the key/value pair together with
 * the signature in the skipchain.
 * <p>
 * When the corresponding value to a key is requested, the value together with the signature will be
 * returned to the service.
 * TODO: create a correct inclusion-proof of the value in the skipblock, and a proof that the skipblock is valid.
 * TODO: for a non-changing roster, it would be enough to check the hash of the skipblock, the signature of the
 * TODO: roster, the inclusion-proof in the merkle-tree and the hash of the root-node of the merkle tree in the
 * TODO: skipblock.
 */
public class SkipchainRPC {
    private Roster roster;
    private SkipblockId scid;
    private static int version = 1;
    private final Logger logger = LoggerFactory.getLogger(SkipchainRPC.class);

    /**
     * Initializes a Storage adapter with the standard node at roster. This uses the pre-stored and
     * pre-initialized values from the DEDISSkipchain class and accesses the nodes run by roster on the
     * server conode.dedis.ch on ports 15002-15006.
     *
     * @throws CothorityCommunicationException
     */
    public SkipchainRPC() throws CothorityCommunicationException, CothorityCryptoException {
        this(DEDISSkipchain.roster, new SkipblockId(DEDISSkipchain.skipchainID));
    }

    /**
     * Connects to an existing skipchain given the roster and the skipchain-id. To verify if
     * the roster is active, the verify-method can be called.
     *
     * @param roster the list of conodes
     * @param id     the skipchain-id to connect to
     * @throws CothorityCommunicationException
     */
    public SkipchainRPC(Roster roster, SkipblockId id) throws CothorityCommunicationException {
        this.roster = roster;
        this.scid = id;
    }

    /**
     * Initializes a new skipchain given a roster of conodes and a public key that is
     * allowed to write on it.
     * This will ask the nodes defined in the roster to create a new skipchain and store
     * the public key in the genesis block.
     */
    public SkipchainRPC(Roster roster, PublicKey pub) throws CothorityCommunicationException {
        this.roster = roster;
        LleapProto.CreateSkipchain.Builder request =
                LleapProto.CreateSkipchain.newBuilder();
        request.setRoster(roster.getProto());
        request.setVersion(version);
        request.addWriters(ByteString.copyFrom(pub.getEncoded()));

        ByteString msg = roster.sendMessage("Lleap/CreateSkipchain",
                request.build());

        try {
            LleapProto.CreateSkipchainResponse reply = LleapProto.CreateSkipchainResponse.parseFrom(msg);
            if (reply.getVersion() != version) {
                throw new CothorityCommunicationException("Version mismatch");
            }
            logger.info("Created new skipchain:");
            logger.info(DatatypeConverter.printHexBinary(reply.getSkipblock().getHash().toByteArray()));
            this.scid = new SkipblockId(reply.getSkipblock().getHash().toByteArray());
            return;
        } catch (InvalidProtocolBufferException e) {
            throw new CothorityCommunicationException(e);
        } catch (CothorityCryptoException e) {
            throw new CothorityCommunicationException(e.getMessage());
        }
    }

    /**
     * setKeyValue sends a key/value pair to the skipchain for inclusion. The skipchain will
     * verify the signature against the public key stored in the genesis-block and the message
     * which is the concatenation of (key | value).
     * <p>
     * setKeyValue will refuse to update a key - a key/value pair can only be stored once.
     *
     * @param key       under which key the value will be stored
     * @param value     the value to store, must be < 1MB
     * @param signature proofing that the writer is authorized. It should be a signature on
     *                  (key | value).
     * @throws CothorityCommunicationException
     */
    public void setKeyValue(byte[] key, byte[] value, byte[] signature) throws CothorityCommunicationException {
        LleapProto.SetKeyValue.Builder request =
                LleapProto.SetKeyValue.newBuilder();
        request.setKey(ByteString.copyFrom(key));
        request.setValue(ByteString.copyFrom(value));
        request.setSkipchainid(scid.toBS());
        request.setVersion(version);
        request.setSignature(ByteString.copyFrom(signature));

        ByteString msg = roster.sendMessage("Lleap/SetKeyValue",
                request.build());

        try {
            LleapProto.SetKeyValueResponse reply = LleapProto.SetKeyValueResponse.parseFrom(msg);
            if (reply.getVersion() != version) {
                throw new CothorityCommunicationException("Version mismatch");
            }
            logger.info("Set key/value pair");
            return;
        } catch (InvalidProtocolBufferException e) {
            throw new CothorityCommunicationException(e);
        }
    }

    /**
     * Convenience function that will sign the key/value pair with the correct message
     * using the given privateKey. privateKey must correspond to the publicKey stored in
     * the genesis-block of the skipchain.
     * <p>
     * For the pre-configured skipchain, the private/public key is available in the
     * DEDISSkipchain class.
     *
     * @param key        under which key the value should be stored
     * @param value      any slice of bytes, must be < 1MB
     * @param privateKey will be used to sign the key/value pair
     * @throws CothorityCommunicationException
     */
    public void setKeyValue(byte[] key, byte[] value, PrivateKey privateKey) throws CothorityCommunicationException {
        try {
            // Create a signature on the key/value pair
            Signature signature = Signature.getInstance("SHA256withRSA");
            signature.initSign(privateKey);
            byte[] message = new byte[key.length + value.length];
            System.arraycopy(key, 0, message, 0, key.length);
            System.arraycopy(value, 0, message, key.length, value.length);
            signature.update(message);

            // And write using the signature
            byte[] sig = signature.sign();
            setKeyValue(key, value, sig);
        } catch (InvalidKeyException e) {
            throw new RuntimeException(e.getMessage());
        } catch (NoSuchAlgorithmException e) {
            throw new RuntimeException(e.getMessage());
        } catch (SignatureException e) {
            throw new RuntimeException(e.getMessage());
        }
    }

    /**
     * getValue returns the value/signature pair of a given key. The signature will verify
     * against the public key of the writer. The message of the signature is the concatenation
     * of (key | value).
     *
     * @param key which key to retrieve
     * @return a value / signature pair
     * @throws CothorityCommunicationException
     */
    public Pair<byte[], byte[]> getValue(byte[] key) throws CothorityCommunicationException {
        LleapProto.GetValue.Builder request =
                LleapProto.GetValue.newBuilder();
        request.setKey(ByteString.copyFrom(key));
        request.setSkipchainid(scid.toBS());
        request.setVersion(version);

        ByteString msg = roster.sendMessage("Lleap/GetValue",
                request.build());

        try {
            LleapProto.GetValueResponse reply = LleapProto.GetValueResponse.parseFrom(msg);
            if (reply.getVersion() != version) {
                throw new CothorityCommunicationException("Version mismatch");
            }
            logger.info("Got value");
            return new Pair<>(reply.getValue().toByteArray(),
                    reply.getSignature().toByteArray());
        } catch (InvalidProtocolBufferException e) {
            throw new CothorityCommunicationException(e);
        }
    }

    /**
     * Convenience method that will fetch the corresponding value to a key and verify it
     * against the public key. If the signature fails, a CothorityCommunicationException
     * is thrown.
     *
     * @param key       to lookup in the skipchain
     * @param publicKey to verify the returned value
     * @return the value if the signature could be verified
     * @throws CothorityCommunicationException
     */
    public byte[] getValue(byte[] key, PublicKey publicKey) throws CothorityCommunicationException {
        Pair<byte[], byte[]> valueSig = getValue(key);

        byte[] value = valueSig.getKey();
        byte[] message = new byte[key.length + value.length];
        System.arraycopy(key, 0, message, 0, key.length);
        System.arraycopy(value, 0, message, key.length, value.length);
        try {
            Signature verify = Signature.getInstance("SHA256withRSA");
            verify.initVerify(publicKey);
            verify.update(message);
            if (!verify.verify(valueSig.getValue())) {
                throw new CothorityCommunicationException("Signature verification failed");
            }
            // TODO: verify the inclusion proof
        } catch (InvalidKeyException e) {
            throw new RuntimeException(e.getMessage());
        } catch (NoSuchAlgorithmException e) {
            throw new RuntimeException(e.getMessage());
        } catch (SignatureException e) {
            throw new RuntimeException(e.getMessage());
        }
        return value;
    }

    /**
     * Contacts all nodes in the cothority and returns true only if _all_
     * nodes returned OK.
     *
     * @return true only if all nodes are OK, else false.
     */
    public boolean verify() {
        boolean ok = true;
        for (ServerIdentity n : roster.getNodes()) {
            logger.info("Testing node {}", n.getAddress());
            try {
                n.GetStatus();
            } catch (CothorityCommunicationException e) {
                logger.warn("Failing node {}", n.getAddress());
                ok = false;
            }
        }
        return ok;
    }
}
