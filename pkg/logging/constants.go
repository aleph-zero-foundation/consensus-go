package logging

// Shortcuts for log messages.
// This is not very elegant, but helps to save lots of space for frequently occurring messages.
// Any event that happens multiple times should have a single character representation
const (
	ServiceStarted = "started"
	ServiceStopped = "stopped"
)

// eventTypeDict maps single char event names to human readable names
// this will be used in future by JSONlog -> log4humans translator
const eventTypeDict = map[string]string{}

// Service types
const (
	CreateService int = iota
	OrderService
	SyncService
	ValidateService
)

// serviceTypeDict maps integer service types to human readable names
// this will be used in future by JSONlog -> log4humans translator
const serviceTypeDict = map[int]string{
	0: "create",
	1: "order",
	2: "sync",
	3: "validate",
}
