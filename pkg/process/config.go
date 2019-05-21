package process

import (
	"time"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
)

// Config represents a complete configuration needed for a process to start.
type Config struct {
	Poset      *gomel.PosetConfig
	Sync       *Sync
	Create     *Create
	Order      *Order
	TxValidate *TxValidate
	TxGenerate *TxGenerate
}

// Sync represents a complete configuration needed for a syncing service to start.
type Sync struct {
	LocalAddress         string
	RemoteAddresses      []string
	ListenQueueLength    int
	SyncQueueLength      int
	InitializedSyncLimit int
	ReceivedSyncLimit    int
	SyncInitDelay        int
	Timeout              time.Duration
}

// Create represents a complete configuration needed for a creating service to start.
type Create struct {
	Pid          int
	MaxParents   int
	PrivateKey   gomel.PrivateKey
	InitialDelay int
	AdjustFactor float64
	MaxLevel     int
	MaxHeight    int
}

// Order represents a complete configuration needed for an ordering service to start.
type Order struct {
	VotingLevel  int
	PiDeltaLevel int
}

// TxValidate represents a complete configuration needed for a transaction validation service to start.
// For now UserDb is a filename with list of users (we can use ../testdata/users.txt),
// it should be replaced with some actual Db handler
type TxValidate struct {
	UserDb string
}

// TxGenerate represents a complete configuration needed for a tx generation service to start.
// For now UserDb is a filename with list of users, it should be replaced with some actual
// Db handler
type TxGenerate struct {
	UserDb string
	Txpu   uint32
}
