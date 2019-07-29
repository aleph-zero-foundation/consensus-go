package process

import (
	"time"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

// Config represents a complete configuration needed for a process to start.
type Config struct {
	Dag         *gomel.DagConfig
	Sync        *Sync
	SyncSetup   *Sync
	Create      *Create
	CreateSetup *Create
	Order       *Order
	TxValidate  *TxValidate
	TxGenerate  *TxGenerate
	MemLog      int
}

// Sync represents a complete configuration needed for a syncing service to start.
type Sync struct {
	Pid               int
	LocalAddress      string
	RemoteAddresses   []string
	LocalMCAddress    string
	RemoteMCAddresses []string
	OutSyncLimit      uint
	InSyncLimit       uint
	Timeout           time.Duration
	Multicast         string
}

// Create represents a complete configuration needed for a creating service to start.
type Create struct {
	Pid          int
	MaxParents   int
	PrimeOnly    bool
	CanSkipLevel bool
	PrivateKey   gomel.PrivateKey
	InitialDelay time.Duration
	AdjustFactor float64
	MaxLevel     int
}

// Order represents a complete configuration needed for an ordering service to start.
type Order struct {
	Pid             int
	VotingLevel     int
	PiDeltaLevel    int
	OrderStartLevel int
}

// TxValidate represents a complete configuration needed for a transaction validation service to start.
// For now UserDb is a filename with list of users (we can use ../testdata/users.txt),
// it should be replaced with some actual Db handler
type TxValidate struct {
}

// TxGenerate represents a complete configuration needed for a tx generation service to start.
// For now UserDb is a filename with list of users, it should be replaced with some actual
// Db handler
type TxGenerate struct {
	CompressionLevel int
	Txpu             uint32
}
