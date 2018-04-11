package ch.epfl.dedis.lleap;

import org.junit.jupiter.api.Test;

import java.security.PrivateKey;
import java.security.PublicKey;
import java.text.SimpleDateFormat;

import static org.junit.jupiter.api.Assertions.assertArrayEquals;

public class TestApp {
    @Test
    public void writeAndRead() throws Exception {
        // Initialising private/public keys
        PrivateKey privateKey = DEDISSkipchain.getPrivate();
        PublicKey publicKey = DEDISSkipchain.getPublic();

        // Connecting to the skipchain and verifying the connection
        SkipchainRPC sc = new SkipchainRPC();
        if (!sc.verify()) {
            throw new RuntimeException("couldn't connect to skipchain");
        }

        // Writing a key/value pair to the skipchain - we cannot overwrite
        // existing values, so we create a different value depending on
        // date/time.
        String keyStr = new SimpleDateFormat("yyyy.MM.dd.HH.mm.ss.SSS").format(new java.util.Date());
        byte[] key = keyStr.getBytes();
        byte[] value = "hashes".getBytes();
        sc.setKeyValue(key, value, privateKey);

        // Reading it back from the skipchain, verifying the signature and
        // returning the value.
        byte[] read = sc.getValue(key, publicKey);
        assertArrayEquals(value, read);
    }
}
