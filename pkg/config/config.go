// Package config reads and writes the configuration of the program.
//
// This package handles both the parameters of the protocol, as well as all the needed keys and committee information.
package config

import (
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
