package config

// Configuration represents project-wide configuration.
type Configuration struct {
	// maximal number of parents a unit can have
	N_PARENTS uint

	// delay after creating a new unit
	CREATE_DELAY float32

	// a number in (0,1) describing how aggresive is the create_delay adjusting mechanism, large = aggresive
	STEP_SIZE float32

	// delay after initianing a sync with other processes
	SYNC_INIT_DELAY float32

	// number of allowed parallel received syncs
	N_RECV_SYNC uint

	// number of allowed parallel initiated syncs
	N_INIT_SYNC uint

	// number of transactions per unit
	TXPU uint

	// limit of all txs generated for one process
	TX_LIMIT uint

	// maximal level after which process shuts down
	LEVEL_LIMIT uint

	// maximal number of units that are constructed
	UNITS_LIMIT *uint

	// maximal number of syncs that are performed
	SYNCS_LIMIT *uint

	// whether to use threshold coin
	USE_TCOIN bool

	// precompute popularity proof to ease computational load of Poset.compute_vote procedure
	PRECOMPUTE_POPULARITY bool

	// whether to use the adaptive strategy of determining create_delay
	ADAPTIVE_DELAY bool

	// level at which the first voting round occurs, this is "t" from the write-up
	VOTING_LEVEL uint

	// level at which to switch from the "fast" to the pi_delta algorithm
	PI_DELTA_LEVEL uint

	// level at which to start adding coin shares to units, it's safe to make it PI_DELTA_LEVEL - 1
	ADD_SHARES uint

	// default ip address of a process
	HOST_IP string

	// default port of incoming syncs
	HOST_PORT uint

	// name of our logger and logfile
	LOGGER_NAME string

	// source of txs
	TX_SOURCE string
}

// NewDefaultConfiguration returns default set of parameters.
func NewDefaultConfiguration() Configuration {
	result := Configuration{

		N_PARENTS: 10,

		CREATE_DELAY: 2.0,

		STEP_SIZE: 0.14,

		SYNC_INIT_DELAY: 0.015625,

		N_RECV_SYNC: 10,

		N_INIT_SYNC: 10,

		TXPU: 1,

		TX_LIMIT: 1000000,

		LEVEL_LIMIT: 20,

		UNITS_LIMIT: nil,

		SYNCS_LIMIT: nil,

		USE_TCOIN: true,

		PRECOMPUTE_POPULARITY: false,

		ADAPTIVE_DELAY: true,

		VOTING_LEVEL: 3,

		PI_DELTA_LEVEL: 12,

		ADD_SHARES: 0,

		HOST_IP: "127.0.0.1",

		HOST_PORT: 8888,

		LOGGER_NAME: "aleph",

		TX_SOURCE: "tx_source_gen",
	}
	result.ADD_SHARES = result.PI_DELTA_LEVEL - 1
	return result
}

// Value returns a pointer to memory location containing provided value.
func Value(value uint) *uint {
	return &value
}
