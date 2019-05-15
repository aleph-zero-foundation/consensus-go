package process

import sign "gitlab.com/alephledger/consensus-go/pkg/crypto/signing"

// Config represents a complete configuration needed for a process to start.
type Config struct {
	Keys     []sign.PublicKey
	Sync     *Sync
	Create   *Create
	Order    *Order
	Validate *Validate
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
}

// Create represents a complete configuration needed for a creating service to start.
type Create struct {
}

// Order represents a complete configuration needed for an ordering service to start.
type Order struct {
}

// Validate represents a complete configuration needed for a transaction validation service to start.
type Validate struct {
}
