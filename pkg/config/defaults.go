package config

import (
	"time"

	"gitlab.com/alephledger/consensus-go/pkg/dag/check"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

func defaultTemplate() Config {
	return &conf{
		CreateDelay:    500 * time.Millisecond,
		EpochLength:    1000,
		CRPFixedPrefix: 5,
		GossipAbove:    50,
		FetchInterval:  2 * time.Second,
		LogFile:        "aleph.log",
		LogLevel:       1,
		LogHuman:       false,
		LogBuffer:      100000,
		LogMemInterval: 10,
		Checks:         []gomel.UnitChecker{check.BasicCorrectness, check.ParentConsistency},
	}
}

func generate(m *Member, c *Committee) Config {
	cnf := defaultTemplate()
	cnf.Pid = m.Pid
	cnf.NProc = uint16(len(c.PublicKeys))
	cnf.PrivateKey = m.PrivateKey
	cnf.PublicKeys = c.PublicKeys
	cnf.RMCPublicKeys = c.RMCVerificationKeys
	cnf.RMCPrivateKey = m.RMCSecretKey
	cnf.P2PPublicKeys = c.P2PPublicKeys
	cnf.P2PSecretKey = m.P2PSecretKey
	return cnf
}

func addSetup(cnf Config) Config {
	cnf.CanSkipLevel = false
	cnf.OrderStartLevel = 6
	cnf.Checks = append(cnf.Checks, check.NoLevelSkipping, check.NoForks)
	return cnf
}

func addMain(cnf Config) Config {
	cnf.CanSkipLevel = true
	cnf.OrderStartLevel = 0
	cnf.Checks = append(cnf.Checks, check.NoSelfForkingEvidence, check.ForkerMuting)
	return cnf
}

func NewSetup(m *Member, c *Committee) Config {
	return addSetup(generate(m, c))
}

func NewMain(m *Member, c *Committee) Config {
	return addMain(generate(m, c))
}
