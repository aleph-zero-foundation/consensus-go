package config

import (
	"time"

	"gitlab.com/alephledger/consensus-go/pkg/dag/check"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

// default template returns Config for consensus with default values.
func defaultTemplate() Config {
	return &conf{
		CRPFixedPrefix: 5,
		GossipAbove:    50,
		FetchInterval:  2 * time.Second,
		LogFile:        "aleph.log",
		LogLevel:       1,
		LogHuman:       false,
		LogBuffer:      100000,
		LogMemInterval: 10,
		Checks:         []gomel.UnitChecker{check.BasicCorrectness, check.ParentConsistency},
		Timeout:        2 * time.Second,
	}
}

// newConfig return a Config with default values together with Member and Committee data.
func newConfig(m *Member, c *Committee) Config {
	cnf := defaultTemplate()
	cnf.Pid = m.Pid
	cnf.NProc = uint16(len(c.PublicKeys))
	cnf.PrivateKey = m.PrivateKey
	cnf.PublicKeys = c.PublicKeys
	cnf.RMCPublicKeys = c.RMCVerificationKeys
	cnf.RMCPrivateKey = m.RMCSecretKey
	cnf.P2PPublicKeys = c.P2PPublicKeys
	cnf.P2PSecretKey = m.P2PSecretKey
	cnf.RMCAddresses = c.RMCAddresses
	cnf.GossipAddresses = c.Addresses["gossip"]
	cnf.FetchAddresses = c.Addresses["fetch"]
	cnf.MCastAddresses = c.Addresses["mcast"]
	cnf.GossipWorkers = [3]int{1, 1, 1}
	cnf.FetchWorkers = [2]int{int(cnf.NProc) / 2, int(cnf.NProc) / 4}
	return cnf
}

func addSetupConf(cnf Config) Config {
	cnf.CanSkipLevel = false
	cnf.OrderStartLevel = 6
	cnf.CRPFixedPrefix = 0
	cnf.EpochLength = 1
	cnf.NumberOfEpochs = 1
	cnf.Checks = append(cnf.Checks, check.NoLevelSkipping, check.NoForks)
	return cnf
}

func addRegularConf(cnf Config) Config {
	cnf.CanSkipLevel = true
	cnf.OrderStartLevel = 0
	cnf.CRPFixedPrefix = 5
	cnf.EpochLength = 1000
	cnf.NumberOfEpochs = 10
	cnf.Checks = append(cnf.Checks, check.NoSelfForkingEvidence, check.ForkerMuting)
	return cnf
}

// NewSetup returns a Config for setup phase given Member and Committee data.
func NewSetup(m *Member, c *Committee) Config {
	return addSetupConf(newConfig(m, c))
}

// New returns a Config for regular consensus run from the given Member and Committee data.
func New(m *Member, c *Committee) Config {
	return addRegularConf(newConfig(m, c))
}

// Empty returns an empty Config populated by zero-values.
func Empty() Config {
	return &conf{}
}
