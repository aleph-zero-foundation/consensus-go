package config

import (
	"time"

	"gitlab.com/alephledger/consensus-go/pkg/crypto/p2p"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/core-go/pkg/crypto/bn256"
)

// Config represents a complete configuration needed for a process to start.
type Config struct {
	NProc         uint16
	Alert         *Alert
	Sync          []*Sync
	SyncSetup     []*Sync
	Create        *Create
	CreateSetup   *Create
	Order         *Order
	OrderSetup    *Order
	MemLog        int
	Setup         string
	P2PPublicKeys []*p2p.PublicKey
	P2PSecretKey  *p2p.SecretKey
	PublicKeys    []gomel.PublicKey
}

// Alert represents a complete configuration needed for an alert system to start.
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
type Sync struct {
	Type            string
	Pid             uint16
	LocalAddress    string
	RemoteAddresses []string
	Params          map[string]string
	Fallback        string
	Pubs            []*bn256.VerificationKey
	Priv            *bn256.SecretKey
}

// Create represents a complete configuration needed for a creating service to start.
type Create struct {
	Pid          uint16
	PrimeOnly    bool
	CanSkipLevel bool
	PrivateKey   gomel.PrivateKey
	Delay        time.Duration
	MaxLevel     int
}

// Order represents a complete configuration needed for an ordering service to start.
type Order struct {
	Pid             uint16
	OrderStartLevel int
	CRPFixedPrefix  uint16
}
