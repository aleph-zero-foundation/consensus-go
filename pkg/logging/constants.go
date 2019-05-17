package logging

// Shortcuts for log messages.
// This is not very elegant, but helps to save lots of space for frequently occurring messages.
// Any event that happens multiple times should have a single character representation
const (
	ServiceStarted      = "started"
	ServiceStopped      = "stopped"
	NewTimingUnit       = "T"
	LinearOrderExtended = "L"
)

// eventTypeDict maps single char event names to human readable names
// this will be used in future by JSONlog -> log4humans translator
var eventTypeDict = map[string]string{
	"T": "new timing unit",
	"L": "linear order extended",
}

// Service types
const (
	CreateService int = iota
	OrderService
	SyncService
	ValidateService
)

// serviceTypeDict maps integer service types to human readable names
// this will be used in future by JSONlog -> log4humans translator
var serviceTypeDict = map[int]string{
	0: "create",
	1: "order",
	2: "sync",
	3: "validate",
}
