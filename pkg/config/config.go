package config

// SyncConfiguration represents parameters for a synchronization service
type SyncConfiguration struct {
	// Type describes the service type.
	Type string
	// Params holds additional parameters needed by a service of a given type
	Params map[string]string
	// Fallback is a name of a service that is to be used as a fallback to this service
	Fallback string
}

// Configuration represents project-wide configuration.
type Configuration struct {
	// How many parents we try to give every unit.
	// Depending on other settings and circumstances, this might be ignored in either direction.
	NParents uint

	// Whether only prime units should be created.
	PrimeOnly bool

	// Wheather a level can be skipped
	CanSkipLevel bool

	// Delay after attempting to create a new unit, before another attempt is made.
	CreateDelay float32

	// A positive number describing how aggressive the CreateDelay adjusting mechanism is initially.
	// A large value means aggressive adjustment, while 0 - no adjustment at all.
	StepSize float64

	// Configurations for synchronization services
	SyncSetup []SyncConfiguration

	// Name of setup procedure
	Setup string

	// Configurations for synchronization services
	Sync []SyncConfiguration

	// The number of transactions included in a unit.
	// Currently only simulated by including random bytes depending on this number.
	// Will be removed completely in the future, whengomel becomes transaction-agnostic.
	Txpu uint

	// When a unit of this level is added to the dag, the process shuts down.
	LevelLimit uint

	// The level at which the first voting round occurs, this is "t" from the write-up.
	VotingLevel uint

	// The level at which to switch from the "fast" to the pi_delta algorithm.
	PiDeltaLevel uint

	// The level at which to start adding coin shares to units.
	// It is safe to make it PiDeltaLevel - 1.
	AddShares uint

	// Log level: 0-debug 1-info 2-warn 3-error 4-fatal 5-panic.
	LogLevel int

	// The size of log diode buffer in bytes. 0 disables the diode. Recommended at least 100k.
	LogBuffer int

	// How often (in seconds) to log the memory usage. 0 to disable.
	LogMemInterval int

	// Whether to write the log in the human readable form or in JSON.
	LogHuman bool

	// The level from which we start ordering
	OrderStartLevel int

	// The number of default pids used before using the CRP
	CRPFixedPrefix int
}

// NewDefaultConfiguration returns default set of parameters.
func NewDefaultConfiguration() Configuration {
	syncConf := []SyncConfiguration{SyncConfiguration{
		Type:     "gossip",
		Params:   map[string]string{"nIn": "20", "nOut": "15", "timeout": "2"},
		Fallback: "",
	}, SyncConfiguration{
		Type:     "multicast",
		Params:   map[string]string{"mcType": "pers", "timeout": "2"},
		Fallback: "",
	},
	}

	result := Configuration{

		NParents: 10,

		PrimeOnly: true,

		CanSkipLevel: true,

		CreateDelay: 0.1,

		StepSize: 0.0,

		SyncSetup: syncConf,

		Setup: "coin",

		Sync: syncConf,

		Txpu: 1,

		LevelLimit: 20,

		VotingLevel: 3,

		PiDeltaLevel: 12,

		AddShares: 0,

		LogLevel: 1,

		LogBuffer: 100000,

		LogMemInterval: 10,

		LogHuman: false,

		OrderStartLevel: 0,

		CRPFixedPrefix: 5,
	}
	result.AddShares = result.PiDeltaLevel - 1
	return result
}

// Value returns a pointer to memory location containing provided value.
func Value(value uint) *uint {
	return &value
}
