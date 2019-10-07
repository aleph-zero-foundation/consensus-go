// Package config reads and writes the configuration of the program.
//
// This package handles both the parameters of the protocol, as well as all the needed keys and committee information.
package config

const (
	// MaxDataBytesPerUnit is the maximal allowed size of data included in a unit, in bytes.
	MaxDataBytesPerUnit = 2e6
	// MaxRandomSourceDataBytesPerUnit is the maximal allowed size of random source data included in a unit, in bytes.
	MaxRandomSourceDataBytesPerUnit = 1e6
	// MaxAntichainsInChunk is the maximal number of antichains in a chunk.
	MaxAntichainsInChunk = 255
)

// SyncConfiguration represents parameters for a synchronization service.
type SyncConfiguration struct {
	// Type describes the service type.
	Type string
	// Params holds additional parameters needed by a service of a given type.
	Params map[string]string
	// Fallback is a name of a service that is to be used as a fallback to this service.
	Fallback string
	// Retry tells how often retry the fallback (0 disables retrying)
	Retry string
}

// Configuration represents project-wide configuration.
type Configuration struct {
	// Whether a process is allowed not to create a unit at a level.
	CanSkipLevel bool

	// Delay after attempting to create a new unit, before another attempt is made.
	CreateDelay float32

	// A positive number describing how aggressive the CreateDelay adjusting mechanism is initially.
	// A large value means aggressive adjustment, while 0 - no adjustment at all.
	StepSize float64

	// Configurations for synchronization services during the setup phase.
	SyncSetup []SyncConfiguration

	// Name of the setup procedure.
	Setup string

	// Configurations for synchronization services during the main protocol operation.
	Sync []SyncConfiguration

	// The number of transactions included in a unit.
	// Currently only simulated by including random bytes depending on this number.
	// Will be removed completely in the future, when gomel becomes transaction-agnostic.
	Txpu int

	// When a unit of this level is added to the dag, the process shuts down.
	LevelLimit int

	// Log level: 0-debug 1-info 2-warn 3-error 4-fatal 5-panic.
	LogLevel int

	// The size of log diode buffer in bytes. 0 disables the diode. Recommended at least 100k.
	LogBuffer int

	// How often (in seconds) to log the memory usage. 0 to disable.
	LogMemInterval int

	// Whether to write the log in the human readable form or in JSON.
	LogHuman bool

	// The level from which we start ordering units.
	OrderStartLevel int

	// The number of default, pseudo-random, pids used before using the truly random CRP.
	CRPFixedPrefix uint16
}

// NewDefaultConfiguration returns default set of parameters.
func NewDefaultConfiguration() Configuration {
	syncConf := []SyncConfiguration{SyncConfiguration{
		Type:     "gossip",
		Params:   map[string]string{"nIn": "20", "nOut": "15", "timeout": "2s"},
		Fallback: "",
	}, SyncConfiguration{
		Type:     "multicast",
		Params:   map[string]string{"network": "pers", "timeout": "2s"},
		Fallback: "",
	},
	}

	syncSetupConf := []SyncConfiguration{SyncConfiguration{
		Type:     "gossip",
		Params:   map[string]string{"nIn": "20", "nOut": "15", "timeout": "2s"},
		Fallback: "",
	}, SyncConfiguration{
		Type:     "multicast",
		Params:   map[string]string{"network": "pers", "timeout": "2s"},
		Fallback: "",
	},
	}

	result := Configuration{

		CanSkipLevel: true,

		CreateDelay: 0.1,

		StepSize: 0.0,

		SyncSetup: syncSetupConf,

		Setup: "beacon",

		Sync: syncConf,

		Txpu: 1,

		LevelLimit: 20,

		LogLevel: 1,

		LogBuffer: 100000,

		LogMemInterval: 10,

		LogHuman: false,

		OrderStartLevel: 0,

		CRPFixedPrefix: uint16(5),
	}
	return result
}
