package logging

// Shortcuts for log messages.
// This is not very elegant, but helps to save lots of space for frequently occurring messages.
// Any event that happens multiple times should have a single character representation
const (
	ServiceStarted      = "start"
	ServiceStopped      = "stop"
	UnitCreated         = "U"
	PrimeUnitCreated    = "P"
	NewTimingUnit       = "T"
	LinearOrderExtended = "L"
	NotEnoughParents    = "Z"
)

// eventTypeDict maps single char event names to human readable names
// this will be used in future by JSONlog -> log4humans translator
var eventTypeDict = map[string]string{
	"U": "new regular unit created",
	"P": "new prime unit created",
	"T": "new timing unit",
	"L": "linear order extended",
	"Z": "creating.NewUnit failed (not enough parents)",
}

// Service types
const (
	CreateService int = iota
	OrderService
	SyncService
	ValidateService
	GenerateService
)

// serviceTypeDict maps integer service types to human readable names
// this will be used in future by JSONlog -> log4humans translator
var serviceTypeDict = map[int]string{
	0: "create",
	1: "order",
	2: "sync",
	3: "validate",
	4: "generate",
}
