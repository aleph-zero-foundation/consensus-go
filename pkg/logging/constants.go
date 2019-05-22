package logging

// Shortcuts for event types.
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

// eventTypeDict maps short event names to human readable form
var eventTypeDict = map[string]string{
	UnitCreated:         "new regular unit created",
	PrimeUnitCreated:    "new prime unit created",
	NewTimingUnit:       "new timing unit",
	LinearOrderExtended: "linear order extended",
	NotEnoughParents:    "creating.NewUnit failed (not enough parents)",
}

// Field names
const (
	Time    = "T"
	Level   = "L"
	Event   = "E"
	Service = "S"
	Size    = "N"
	Txs     = "X"
	Height  = "H"
	Round   = "R"
)

// fieldNameDict maps short field names to human readable form
var fieldNameDict = map[string]string{
	Time:    "time",
	Level:   "level",
	Event:   "event",
	Service: "service",
	Size:    "size",
	Txs:     "txs",
	Height:  "height",
	Round:   "round",
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
var serviceTypeDict = map[int]string{
	CreateService:   "create",
	OrderService:    "order",
	SyncService:     "sync",
	ValidateService: "validate",
	GenerateService: "generate",
}

// Genesis was better with Phil Collins
const Genesis = "genesis"
