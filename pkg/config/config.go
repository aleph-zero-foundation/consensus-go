package config

import (
	"time"

	"gitlab.com/alephledger/consensus-go/pkg/crypto/p2p"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/core-go/pkg/crypto/bn256"
)

// Config represents a complete configuration needed for a process to start.
// Exported type is a pointer type to make sure that we always deal with only one underlying struct.
type Config *conf

type conf struct {
	Pid   uint16
	NProc uint16
	// epoch
	CreateDelay     time.Duration
	EpochLength     int
	CanSkipLevel    bool
	OrderStartLevel int
	CRPFixedPrefix  uint16
	GossipAbove     int
	FetchInterval   time.Duration
	Checks          []gomel.UnitChecker
	// log
	LogFile        string
	LogLevel       int
	LogHuman       bool
	LogBuffer      int
	LogMemInterval int
	// keys
	PrivateKey    gomel.PrivateKey
	PublicKeys    []gomel.PublicKey
	RMCPrivateKey *bn256.SecretKey
	RMCPublicKeys []*bn256.VerificationKey
	P2PPublicKeys []*p2p.PublicKey
	P2PSecretKey  *p2p.SecretKey
	//shallbedone: remove these two to flatten config
	Alert *Alert
	Sync  []*Sync
}

// Alert represents a complete configuration needed for an alert system to start.
// shallbedone: remove
type Alert struct {
	Pid             uint16
	PublicKeys      []gomel.PublicKey
	Pubs            []*bn256.VerificationKey
	Priv            *bn256.SecretKey
	LocalAddress    string
	RemoteAddresses []string
	Timeout         time.Duration
}

// Sync represents a complete configuration needed for a syncing service to start.
// shallbedone: remove
type Sync struct {
	Type            string
	Pid             uint16
	LocalAddress    string
	RemoteAddresses []string
	Params          map[string]string
	Pubs            []*bn256.VerificationKey
	Priv            *bn256.SecretKey
}

// AddCheck adds a unit checker to the given Config.
func AddCheck(c Config, check gomel.UnitChecker) {
	c.Checks = append(c.Checks, check)
}
