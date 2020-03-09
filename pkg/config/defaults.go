package config

import (
	"reflect"
	"strconv"
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

func checkKeys(slice interface{}, nProc uint16, keyType string) error {
	s := reflect.ValueOf(slice)
	if uint16(s.Len()) != nProc {
		return gomel.NewConfigError("wrong number of " + keyType)
	}

	for i := 0; i < s.Len(); i++ {
		if s.Index(i).IsNil() {
			return gomel.NewConfigError(keyType + " contains nil")
		}
	}
	return nil
}

func addKeys(cnf Config, m *Member, c *Committee) error {
	cnf.Pid = m.Pid
	cnf.NProc = uint16(len(c.PublicKeys))

	if m.PrivateKey == nil {
		return gomel.NewConfigError("Private key is missing")
	}
	cnf.PrivateKey = m.PrivateKey
	if err := checkKeys(c.PublicKeys, cnf.NProc, "PublicKeys"); err != nil {
		return err
	}
	cnf.PublicKeys = c.PublicKeys

	if m.RMCSecretKey == nil {
		return gomel.NewConfigError("RMC private key is missing")
	}
	cnf.RMCPrivateKey = m.RMCSecretKey
	if err := checkKeys(c.RMCVerificationKeys, cnf.NProc, "RMC verification keys"); err != nil {
		return err
	}
	cnf.RMCPublicKeys = c.RMCVerificationKeys

	if m.P2PSecretKey == nil {
		return gomel.NewConfigError("P2P private key is missing")
	}
	cnf.P2PSecretKey = m.P2PSecretKey
	if err := checkKeys(c.P2PPublicKeys, cnf.NProc, "P2P public keys"); err != nil {
		return err
	}
	cnf.P2PPublicKeys = c.P2PPublicKeys

	return nil
}

func addAddresses(cnf Config, addresses map[string][]string, setup bool) error {
	n := int(cnf.NProc)
	ok := func(s []string) bool { return len(s) == n }

	if !ok(addresses["rmc"]) {
		return gomel.NewConfigError("wrong number of rmc addresses")
	}
	cnf.RMCAddresses = addresses["rmc"]

	if !ok(addresses["gossip"]) {
		return gomel.NewConfigError("wrong number of gossip addresses")
	}
	cnf.GossipAddresses = addresses["gossip"]

	if !ok(addresses["fetch"]) {
		return gomel.NewConfigError("wrong number of fetch addresses")
	}
	cnf.FetchAddresses = addresses["fetch"]

	if !setup && !ok(addresses["mcast"]) {
		return gomel.NewConfigError("wrong number of mcast addresses")
	}
	cnf.MCastAddresses = addresses["mcast"]

	cnf.GossipWorkers = [3]int{n/20 + 1, n/40 + 1, 1}
	cnf.FetchWorkers = [2]int{n / 2, n / 4}
	return nil
}

func addSetupConf(cnf Config) {
	cnf.LogFile = strconv.Itoa(int(cnf.Pid)) + ".setup.log"
	cnf.CanSkipLevel = false
	cnf.OrderStartLevel = 6
	cnf.CRPFixedPrefix = 0
	cnf.EpochLength = 1
	cnf.NumberOfEpochs = 1
	cnf.Checks = append(cnf.Checks, check.NoLevelSkipping, check.NoForks)
}

func addConsensusConf(cnf Config) Config {
	cnf.LogFile = strconv.Itoa(int(cnf.Pid)) + ".log"
	cnf.CanSkipLevel = true
	cnf.OrderStartLevel = 0
	cnf.CRPFixedPrefix = 5
	cnf.EpochLength = 50
	cnf.NumberOfEpochs = 2
	cnf.Checks = append(cnf.Checks, check.NoSelfForkingEvidence, check.ForkerMuting)
	return cnf
}

// NewSetup returns a Config for setup phase given Member and Committee data.
func NewSetup(m *Member, c *Committee) (Config, error) {
	cnf := defaultTemplate()
	if err := addKeys(cnf, m, c); err != nil {
		return nil, err
	}
	if err := addAddresses(cnf, c.SetupAddresses, true); err != nil {
		return nil, err
	}
	addSetupConf(cnf)
	return cnf, nil
}

// New returns a Config for regular consensus run from the given Member and Committee data.
func New(m *Member, c *Committee) (Config, error) {
	cnf := defaultTemplate()
	if err := addKeys(cnf, m, c); err != nil {
		return nil, err
	}
	if err := addAddresses(cnf, c.SetupAddresses, false); err != nil {
		return nil, err
	}
	addConsensusConf(cnf)
	return cnf, nil
}

// Empty returns an empty Config populated by zero-values.
func Empty() Config {
	return &conf{}
}
