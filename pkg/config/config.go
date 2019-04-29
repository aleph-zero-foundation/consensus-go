package config

// Configuration represents project-wide configuration.
type Configuration struct {
	// maximal number of parents a unit can have
	NParents uint

	// delay after creating a new unit
	CreateDelay float32

	// a number in (0,1) describing how aggresive is the create_delay adjusting mechanism, large = aggresive
	StepSize float32

	// delay after initianing a sync with other processes
	SyncInitDelay float32

	// number of allowed parallel received syncs
	NRecvSync uint

	// number of allowed parallel initiated syncs
	NInitSync uint

	// number of transactions per unit
	Txpu uint

	// limit of all txs generated for one process
	TxLimit uint

	// maximal level after which process shuts down
	LevelLimit uint

	// maximal number of units that are constructed
	UnitsLimit *uint

	// maximal number of syncs that are performed
	SyncsLimit *uint

	// whether to use threshold coin
	UseTcoin bool

	// precompute popularity proof to ease computational load of Poset.compute_vote procedure
	PrecomputePopularity bool

	// whether to use the adaptive strategy of determining create_delay
	AdaptiveDelay bool

	// level at which the first voting round occurs, this is "t" from the write-up
	VotingLevel uint

	// level at which to switch from the "fast" to the pi_delta algorithm
	PiDeltaLevel uint

	// level at which to start adding coin shares to units, it's safe to make it PI_DELTA_LEVEL - 1
	AddShares uint

	// default ip address of a process
	HostIp string

	// default port of incoming syncs
	HostPort uint

	// name of our logger and logfile
	LoggerName string

	// source of txs
	TxSource string
}

// NewDefaultConfiguration returns default set of parameters.
func NewDefaultConfiguration() Configuration {
	result := Configuration{

		NParents: 10,

		CreateDelay: 2.0,

		StepSize: 0.14,

		SyncInitDelay: 0.015625,

		NRecvSync: 10,

		NInitSync: 10,

		Txpu: 1,

		TxLimit: 1000000,

		LevelLimit: 20,

		UnitsLimit: nil,

		SyncsLimit: nil,

		UseTcoin: true,

		PrecomputePopularity: false,

		AdaptiveDelay: true,

		VotingLevel: 3,

		PiDeltaLevel: 12,

		AddShares: 0,

		HostIp: "127.0.0.1",

		HostPort: 8888,

		LoggerName: "aleph",

		TxSource: "tx_source_gen",
	}
	result.AddShares = result.PiDeltaLevel - 1
	return result
}

// Value returns a pointer to memory location containing provided value.
func Value(value uint) *uint {
	return &value
}
