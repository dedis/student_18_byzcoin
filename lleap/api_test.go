package lleap_test

import (
	"testing"

	// We need to include the service so it is started.
	"gopkg.in/dedis/kyber.v2/suites"
	_ "github.com/dedis/lleap/service"
	"gopkg.in/dedis/onet.v2/log"
)

var tSuite = suites.MustFind("Ed25519")

func TestMain(m *testing.M) {
	log.MainTest(m)
}
