package service

import (
	"testing"

	"github.com/dedis/cothority_template"
	"github.com/stretchr/testify/assert"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/log"
)

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func TestService_ClockRequest(t *testing.T) {
	local := onet.NewTCPTest()
	// generate 5 hosts, they don't connect, they process messages, and they
	// don't register the tree or entitylist
	hosts, roster, _ := local.GenTree(5, true)
	defer local.CloseAll()

	services := local.GetServices(hosts, templateID)

	for _, s := range services {
		log.Lvl2("Sending request to", s)
		resp, err := s.(*Service).ClockRequest(
			&template.ClockRequest{Roster: roster},
		)
		log.ErrFatal(err)
		assert.Equal(t, resp.Children, len(roster.List))
	}
}

func TestService_CountRequest(t *testing.T) {
	local := onet.NewTCPTest()
	// generate 5 hosts, they don't connect, they process messages, and they
	// don't register the tree or entitylist
	hosts, roster, _ := local.GenTree(5, true)
	defer local.CloseAll()

	services := local.GetServices(hosts, templateID)

	for _, s := range services {
		log.Lvl2("Sending request to", s)
		resp, err := s.(*Service).ClockRequest(
			&template.ClockRequest{Roster: roster},
		)
		log.ErrFatal(err)
		assert.Equal(t, resp.Children, len(roster.List))
		count, err := s.(*Service).CountRequest(&template.CountRequest{})
		log.ErrFatal(err)
		assert.Equal(t, 1, count.Count)
	}
}
