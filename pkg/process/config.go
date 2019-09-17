package process

import (
	"time"

	"gitlab.com/alephledger/consensus-go/pkg/crypto/bn256"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

// Config represents a complete configuration needed for a process to start.
type Config struct {
	Dag         *gomel.DagConfig
	Sync        []*Sync
	SyncSetup   []*Sync
	Create      *Create
	CreateSetup *Create
	Order       *Order
	OrderSetup  *Order
	TxValidate  *TxValidate
	TxGenerate  *TxGenerate
	MemLog      int
	Setup       string
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
	MaxParents   uint16
	PrimeOnly    bool
	CanSkipLevel bool
	PrivateKey   gomel.PrivateKey
	InitialDelay time.Duration
	AdjustFactor float64
	MaxLevel     int
}

// Order represents a complete configuration needed for an ordering service to start.
type Order struct {
	Pid             uint16
	OrderStartLevel int
	CRPFixedPrefix  uint16
}

// TxValidate represents a complete configuration needed for a transaction validation service to start.
type TxValidate struct {
}

// TxGenerate represents a complete configuration needed for a tx generation service to start.
type TxGenerate struct {
	CompressionLevel int
	Txpu             int
}
