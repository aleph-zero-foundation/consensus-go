// Package config reads and writes the configuration of the program.
//
// This package handles both the parameters of the protocol, as well as all the needed keys and committee information.
package config

const (
	// MaxDataBytesPerUnit is the maximal allowed size of data included in a unit, in bytes.
	MaxDataBytesPerUnit = 2e6
	// MaxRandomSourceDataBytesPerUnit is the maximal allowed size of random source data included in a unit, in bytes.
	MaxRandomSourceDataBytesPerUnit = 1e6
	// MaxUnitsInChunk is the maximal number of units in a chunk.
	MaxUnitsInChunk = 1e6
)

// Params represents a set of process parameters adjustable via JSON config files.
type Params struct {
	// Whether a process is allowed not to create a unit at a level.
	CanSkipLevel bool

	// Delay after attempting to create a new unit, before another attempt is made.
	CreateDelay float32

	// Name of the setup procedure.
	Setup string

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

// NewDefaultParams returns default set of parameters.
func NewDefaultParams() Params {
	result := Params{

		CanSkipLevel: true,

		CreateDelay: 0.1,

		Setup: "beacon",

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
