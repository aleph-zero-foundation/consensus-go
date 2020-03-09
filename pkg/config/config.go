// Package config reads and writes the configuration of the program.
//
// This package handles both the parameters of the protocol, as well as all the needed keys and committee information.
package config

import (
	"strconv"
	"time"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/core-go/pkg/crypto/bn256"
	"gitlab.com/alephledger/core-go/pkg/crypto/p2p"
	"gitlab.com/alephledger/core-go/pkg/crypto/tss"
)

// Config represents a complete configuration needed for a process to start.
// Exported type is a pointer type to make sure that we always deal with only one underlying struct.
type Config *conf

type conf struct {
	Pid   uint16
	NProc uint16
	// epoch
	EpochLength     int
	NumberOfEpochs  int
	CanSkipLevel    bool
	OrderStartLevel int
	CRPFixedPrefix  uint16
	Checks          []gomel.UnitChecker
	// log
	LogFile        string
	LogLevel       int
	LogHuman       bool
	LogBuffer      int
	LogMemInterval int
	// keys
	WTKey         *tss.WeakThresholdKey
	PrivateKey    gomel.PrivateKey
	PublicKeys    []gomel.PublicKey
	P2PPublicKeys []*p2p.PublicKey
	P2PSecretKey  *p2p.SecretKey
	RMCPrivateKey *bn256.SecretKey
	RMCPublicKeys []*bn256.VerificationKey
	// sync
	GossipAbove     int
	FetchInterval   time.Duration
	Timeout         time.Duration
	RMCAddresses    []string
	RMCNetType      string
	GossipAddresses []string
	GossipNetType   string
	FetchAddresses  []string
	FetchNetType    string
	MCastAddresses  []string
	MCastNetType    string
	GossipWorkers   [3]int // nIn, nOut, nIdle
	FetchWorkers    [2]int // nIn, nOut
}

// AddCheck adds a unit checker to the given Config.
func AddCheck(c Config, check gomel.UnitChecker) {
	c.Checks = append(c.Checks, check)
}

// NewSetup returns a Config for setup phase given Member and Committee data.
func NewSetup(m *Member, c *Committee) Config {
	cnf := &conf{}
	addKeys(cnf, m, c)
	addSyncConf(cnf, c.SetupAddresses, true)
	addLogConf(cnf, strconv.Itoa(int(cnf.Pid))+".setup.log")
	addSetupConf(cnf)

	return cnf
}

// New returns a Config for regular consensus run from the given Member and Committee data.
func New(m *Member, c *Committee) Config {
	cnf := &conf{}
	addKeys(cnf, m, c)
	addSyncConf(cnf, c.Addresses, false)
	addLogConf(cnf, strconv.Itoa(int(cnf.Pid))+".log")
	addConsensusConf(cnf)

	return cnf
}

// Empty returns an empty Config populated by zero-values.
func Empty() Config {
	return &conf{}
}

func addKeys(cnf Config, m *Member, c *Committee) {
	cnf.Pid = m.Pid
	cnf.NProc = uint16(len(c.PublicKeys))
	cnf.PrivateKey = m.PrivateKey
	cnf.PublicKeys = c.PublicKeys
	cnf.RMCPrivateKey = m.RMCSecretKey
	cnf.RMCPublicKeys = c.RMCVerificationKeys
	cnf.P2PSecretKey = m.P2PSecretKey
	cnf.P2PPublicKeys = c.P2PPublicKeys
}

func addSyncConf(cnf Config, addresses map[string][]string, setup bool) {
	cnf.Timeout = time.Second
	cnf.FetchInterval = time.Second
	cnf.GossipAbove = 50

	cnf.RMCNetType = "pers"
	cnf.RMCAddresses = addresses["rmc"]

	cnf.GossipNetType = "pers"
	cnf.GossipAddresses = addresses["gossip"]

	cnf.FetchNetType = "pers"
	cnf.FetchAddresses = addresses["fetch"]

	cnf.MCastNetType = "pers"
	cnf.MCastAddresses = addresses["mcast"]

	n := int(cnf.NProc)
	cnf.GossipWorkers = [3]int{n/20 + 1, n/40 + 1, 1}
	cnf.FetchWorkers = [2]int{n / 2, n / 4}
}

func addLogConf(cnf Config, logFile string) {
	cnf.LogFile = logFile
	cnf.LogBuffer = 100000
	cnf.LogHuman = false
	cnf.LogLevel = 1
	cnf.LogMemInterval = 5
}

func addSetupConf(cnf Config) {
	cnf.CanSkipLevel = false
	cnf.OrderStartLevel = 6
	cnf.CRPFixedPrefix = 0
	cnf.EpochLength = 1
	cnf.NumberOfEpochs = 1
	cnf.Checks = setupChecks
}

func addConsensusConf(cnf Config) {
	cnf.CanSkipLevel = true
	cnf.OrderStartLevel = 0
	cnf.CRPFixedPrefix = 4
	cnf.EpochLength = 50
	cnf.NumberOfEpochs = 2
	cnf.Checks = consensusChecks
}
