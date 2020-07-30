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
	EpochLength    int
	NumberOfEpochs int
	LastLevel      int // LastLevel = EpochLength + OrderStartLevel - 1
	CanSkipLevel   bool
	Checks         []gomel.UnitChecker
	// log
	LogFile   string
	LogLevel  int
	LogHuman  bool
	LogBuffer int
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
	GossipInterval  time.Duration
	Timeout         time.Duration
	RMCAddresses    []string
	RMCNetType      string
	GossipAddresses []string
	GossipNetType   string
	FetchAddresses  []string
	FetchNetType    string
	MCastAddresses  []string
	MCastNetType    string
	GossipWorkers   [2]int // nIn, nOut
	FetchWorkers    [2]int // nIn, nOut
	// linear
	OrderStartLevel               int
	CRPFixedPrefix                uint16
	ZeroVoteRoundForCommonVote    int
	FirstDecidingRound            int
	CommonVoteDeterministicPrefix int
}

// AddCheck adds a unit checker to the given Config.
func AddCheck(c Config, check gomel.UnitChecker) {
	c.Checks = append(c.Checks, check)
}

// NewSetup returns a Config for setup phase given Member and Committee data.
func NewSetup(m *Member, c *Committee) Config {
	cnf := requiredByLinear()
	addKeys(cnf, m, c)
	addSyncConf(cnf, c.SetupAddresses, true)
	addLogConf(cnf, strconv.Itoa(int(cnf.Pid))+".setup")
	addSetupConf(cnf)
	addLastLevel(cnf)
	return cnf
}

// New returns a Config for regular consensus run from the given Member and Committee data.
func New(m *Member, c *Committee) Config {
	cnf := requiredByLinear()
	addKeys(cnf, m, c)
	addSyncConf(cnf, c.Addresses, false)
	addLogConf(cnf, strconv.Itoa(int(cnf.Pid)))
	addConsensusConf(cnf)
	addLastLevel(cnf)
	return cnf
}

// Empty returns an empty Config populated by zero-values.
func Empty() Config {
	return requiredByLinear()
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
	cnf.Timeout = 5 * time.Second
	cnf.FetchInterval = time.Second
	cnf.GossipInterval = 100 * time.Millisecond
	cnf.GossipAbove = 50

	cnf.RMCNetType = "tcp"
	cnf.RMCAddresses = addresses["rmc"]

	cnf.GossipNetType = "tcp"
	cnf.GossipAddresses = addresses["gossip"]

	cnf.FetchNetType = "tcp"
	cnf.FetchAddresses = addresses["fetch"]

	cnf.MCastNetType = "tcp"
	cnf.MCastAddresses = addresses["mcast"]

	n := int(cnf.NProc)
	cnf.GossipWorkers = [2]int{n/20 + 1, n/40 + 1}
	cnf.FetchWorkers = [2]int{n / 2, n / 4}
}

func addLogConf(cnf Config, logFile string) {
	cnf.LogFile = logFile
	cnf.LogBuffer = 100000
	cnf.LogHuman = false
	cnf.LogLevel = 7
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
	cnf.EpochLength = 30
	cnf.NumberOfEpochs = 3
	cnf.Checks = consensusChecks
}

func requiredByLinear() Config {
	return &conf{
		FirstDecidingRound:            3,
		CommonVoteDeterministicPrefix: 10,
		ZeroVoteRoundForCommonVote:    3,
	}
}

func addLastLevel(cnf Config) {
	cnf.LastLevel = cnf.EpochLength + cnf.OrderStartLevel - 1
}
