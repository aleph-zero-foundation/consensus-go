package config

import (
	"reflect"
	"runtime"
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

var (
	setupChecks     = []gomel.UnitChecker{check.BasicCorrectness, check.ParentConsistency, check.NoLevelSkipping, check.NoForks}
	consensusChecks = []gomel.UnitChecker{check.BasicCorrectness, check.ParentConsistency, check.NoSelfForkingEvidence, check.ForkerMuting}
)

// checks if slice is of nProc length and if slice does not contain a nil
func noNils(slice interface{}, nProc uint16, keyType string) error {
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

func checkKeys(cnf Config) error {
	if cnf.NProc == uint16(0) {
		return gomel.NewConfigError("nProc set to 0 during keys check")
	}

	if cnf.PrivateKey == nil {
		return gomel.NewConfigError("Private key is missing")
	}
	if err := noNils(cnf.PublicKeys, cnf.NProc, "PublicKeys"); err != nil {
		return err
	}

	if cnf.RMCPrivateKey == nil {
		return gomel.NewConfigError("RMC private key is missing")
	}
	if err := noNils(cnf.RMCPublicKeys, cnf.NProc, "RMC verification keys"); err != nil {
		return err
	}

	if cnf.P2PSecretKey == nil {
		return gomel.NewConfigError("P2P private key is missing")
	}
	if err := noNils(cnf.P2PPublicKeys, cnf.NProc, "P2P public keys"); err != nil {
		return err
	}

	return nil
}

func checkSyncConf(cnf Config, setup bool) error {
	if cnf.Timeout == 0*time.Second {
		return gomel.NewConfigError("timeout cannot be 0")
	}
	if cnf.FetchInterval == 0*time.Second {
		return gomel.NewConfigError("fetch interval cannot be 0")
	}
	if cnf.GossipAbove == 0 {
		return gomel.NewConfigError("GossipAbove cannot be 0")
	}

	n := int(cnf.NProc)
	ok := func(s []string) bool { return len(s) == n }

	if !ok(cnf.RMCAddresses) {
		return gomel.NewConfigError("wrong number of rmc addresses")
	}
	if !ok(cnf.GossipAddresses) {
		return gomel.NewConfigError("wrong number of gossip addresses")
	}
	if !ok(cnf.FetchAddresses) {
		return gomel.NewConfigError("wrong number of fetch addresses")
	}
	if !setup && !ok(cnf.MCastAddresses) {
		return gomel.NewConfigError("wrong number of mcast addresses")
	}

	if cnf.GossipWorkers[0] == 0 {
		return gomel.NewConfigError("nIn gossip workers cannot be 0")
	}
	if cnf.GossipWorkers[1] == 0 {
		return gomel.NewConfigError("nOut gossip workers cannot be 0")
	}
	if cnf.FetchWorkers[0] == 0 {
		return gomel.NewConfigError("nIn fetch workers cannot be 0")
	}
	if cnf.FetchWorkers[1] == 0 {
		return gomel.NewConfigError("nOut fetch workers cannot be 0")
	}

	return nil
}

func funcName(i interface{}) string {
	return runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
}

func checkChecks(given, expected []gomel.UnitChecker) error {
	for _, sc := range expected {
		notFound := true
		fn := funcName(sc)
		for _, c := range given {
			if funcName(c) == fn {
				notFound = false
				break
			}
		}
		if notFound {
			return gomel.NewConfigError("missing check: " + funcName(sc))
		}
	}
	return nil
}

// checks basic validity of config for both setup and consensus
func valid(cnf Config) error {
	// epoch Checks
	if cnf.NProc < uint16(4) {
		return gomel.NewConfigError("nProc is " + strconv.Itoa(int(cnf.NProc)))
	}
	if cnf.EpochLength < 1 {
		return gomel.NewConfigError("EpochLength is " + strconv.Itoa(cnf.EpochLength))
	}

	// log checks
	if cnf.LogFile == "" {
		return gomel.NewConfigError("missing log filename")
	}
	if cnf.LogBuffer == 0 {
		return gomel.NewConfigError("Log buffer cannot be 0")
	}

	// keys checks
	if err := checkKeys(cnf); err != nil {
		return err
	}

	return nil
}

// Valid checks if a given config is in valid state for consensus
func Valid(cnf Config) error {
	if err := valid(cnf); err != nil {
		return err
	}
	if cnf.CRPFixedPrefix > cnf.NProc {
		return gomel.NewConfigError("CRPFixedPrefix connot exceed NProc")
	}
	if len(cnf.Checks) != len(consensusChecks) {
		return gomel.NewConfigError("wrong number of checks")
	}
	if err := checkChecks(cnf.Checks, consensusChecks); err != nil {
		return err
	}

	if err := checkSyncConf(cnf, false); err != nil {
		return err
	}

	return nil
}

// ValidSetup checks if a given config is in valid state for setup
func ValidSetup(cnf Config) error {
	if err := valid(cnf); err != nil {
		return err
	}
	if cnf.CanSkipLevel {
		return gomel.NewConfigError("Cannot skip level in setup")
	}
	if cnf.OrderStartLevel != 6 {
		return gomel.NewConfigError("OrderStartLevel should be 6 and not " + strconv.Itoa(cnf.OrderStartLevel))
	}
	if cnf.CRPFixedPrefix != 0 {
		return gomel.NewConfigError("CRPFixedPrefix connot be nonzero in setup")
	}
	if len(cnf.Checks) != len(setupChecks) {
		return gomel.NewConfigError("wrong number of checks")
	}
	if err := checkChecks(cnf.Checks, setupChecks); err != nil {
		return err
	}
	if err := checkSyncConf(cnf, true); err != nil {
		return err
	}

	return nil
}
