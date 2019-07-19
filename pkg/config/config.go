package config

// Configuration represents project-wide configuration.
type Configuration struct {
	// How many parents we try to give every unit.
	// Depending on other settings and circumstances, this might be ignored in either direction.
	NParents uint

	// Whether only prime units should be created.
	PrimeOnly bool

	// Delay after attempting to create a new unit, before another attempt is made.
	CreateDelay float32

	// A positive number describing how aggressive the CreateDelay adjusting mechanism is initially.
	// A large value means aggressive adjustment, while 0 - no adjustment at all.
	StepSize float64

	// The number of parallel received syncs that are allowed to happen at once.
	NInSync uint

	// The number of parallel initiated syncs that are allowed to happen at once.
	NOutSync uint

	// Connection timeout in seconds
	Timeout float32

	// Whether to use multicast. Possible values "tcp", "udp". Any other value disables multicast.
	Multicast string

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
}

// NewDefaultConfiguration returns default set of parameters.
func NewDefaultConfiguration() Configuration {
	result := Configuration{

		NParents: 10,

		PrimeOnly: true,

		CreateDelay: 0.1,

		StepSize: 0.0,

		NInSync: 32,

		NOutSync: 32,

		Timeout: 2,

		Multicast: "tcp",

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
	}
	result.AddShares = result.PiDeltaLevel - 1
	return result
}

// Value returns a pointer to memory location containing provided value.
func Value(value uint) *uint {
	return &value
}
