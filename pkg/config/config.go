package config

// Configuration represents project-wide configuration.
type Configuration struct {
	// maximal number of parents a unit can have
	NParents uint

	// delay after creating a new unit
	CreateDelay float32

	// a number in [0,1) describing how aggressive is the CreateDelay adjusting mechanism, large = aggressive, 0 = no adjustment at all
	StepSize float64

	// delay after initializing a sync with other processes
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

	// whether to use the adaptive strategy of determining create_delay
	AdaptiveDelay bool

	// level at which the first voting round occurs, this is "t" from the write-up
	VotingLevel uint

	// level at which to switch from the "fast" to the pi_delta algorithm
	PiDeltaLevel uint

	// level at which to start adding coin shares to units, it's safe to make it PI_DELTA_LEVEL - 1
	AddShares uint

	// log level: 0-debug 1-info 2-warn 3-error 4-fatal 5-panic
	LogLevel int

	// The size of log diode buffer in bytes. 0 disables the diode. Recommended at least 100k.
	LogBuffer int

	// How often (in seconds) log the memory usage. 0 to disable.
	LogMemInterval int

	// whether to write log in human readable form or in JSON.
	LogHuman bool
}

// NewDefaultConfiguration returns default set of parameters.
func NewDefaultConfiguration() Configuration {
	result := Configuration{

		NParents: 10,

		CreateDelay: 1.0,

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

		AdaptiveDelay: true,

		VotingLevel: 3,

		PiDeltaLevel: 12,

		AddShares: 0,

		LogLevel: 1,

		LogBuffer: 100000,

		LogMemInterval: 10,

		LogHuman: false,
	}
	result.AddShares = result.PiDeltaLevel - 1
	return result
}

// Value returns a pointer to memory location containing provided value.
func Value(value uint) *uint {
	return &value
}
