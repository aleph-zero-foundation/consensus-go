package config

import (
	"time"

	"gitlab.com/alephledger/consensus-go/pkg/dag/check"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

const (
	// MaxDataBytesPerUnit is the maximal allowed size of data included in a unit, in bytes.
	MaxDataBytesPerUnit = 2e6
	// MaxRandomSourceDataBytesPerUnit is the maximal allowed size of random source data included in a unit, in bytes.
	MaxRandomSourceDataBytesPerUnit = 1e6
	// MaxUnitsInChunk is the maximal number of units in a chunk.
	MaxUnitsInChunk = 1e6
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

func addKeys(cnf Config, m *Member, c *Committee) {
	cnf.Pid = m.Pid
	cnf.NProc = uint16(len(c.PublicKeys))
	cnf.PrivateKey = m.PrivateKey
	cnf.PublicKeys = c.PublicKeys
	cnf.RMCPublicKeys = c.RMCVerificationKeys
	cnf.RMCPrivateKey = m.RMCSecretKey
	cnf.P2PPublicKeys = c.P2PPublicKeys
	cnf.P2PSecretKey = m.P2PSecretKey
}

func addAddresses(cnf Config, addresses map[string][]string) {
	cnf.RMCAddresses = addresses["rmc"]
	cnf.GossipAddresses = addresses["gossip"]
	cnf.FetchAddresses = addresses["fetch"]
	cnf.MCastAddresses = addresses["mcast"]
	cnf.GossipWorkers = [3]int{1, 1, 1}
	cnf.FetchWorkers = [2]int{int(cnf.NProc) / 2, int(cnf.NProc) / 4}
}

func addSetupConf(cnf Config) {
	cnf.CanSkipLevel = false
	cnf.OrderStartLevel = 6
	cnf.CRPFixedPrefix = 0
	cnf.EpochLength = 1
	cnf.NumberOfEpochs = 1
	cnf.Checks = append(cnf.Checks, check.NoLevelSkipping, check.NoForks)
}

func addConsensusConf(cnf Config) Config {
	cnf.CanSkipLevel = true
	cnf.OrderStartLevel = 0
	cnf.CRPFixedPrefix = 5
	cnf.EpochLength = 50
	cnf.NumberOfEpochs = 2
	cnf.Checks = append(cnf.Checks, check.NoSelfForkingEvidence, check.ForkerMuting)
	return cnf
}

// NewSetup returns a Config for setup phase given Member and Committee data.
func NewSetup(m *Member, c *Committee) Config {
	cnf := defaultTemplate()
	addKeys(cnf, m, c)
	addAddresses(cnf, c.SetupAddresses)
	addSetupConf(cnf)
	return cnf
}

// New returns a Config for regular consensus run from the given Member and Committee data.
func New(m *Member, c *Committee) Config {
	cnf := defaultTemplate()
	addKeys(cnf, m, c)
	addAddresses(cnf, c.Addresses)
	addConsensusConf(cnf)
	return cnf
}

// Empty returns an empty Config populated by zero-values.
func Empty() Config {
	return &conf{}
}
