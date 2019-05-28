package logging

// Shortcuts for event types.
// Any event that happens multiple times should have a single character representation
const (
	ServiceStarted        = "start"
	ServiceStopped        = "stop"
	UnitCreated           = "U"
	PrimeUnitCreated      = "P"
	NewTimingUnit         = "T"
	LinearOrderExtended   = "L"
	ConnectionReceived    = "R"
	ConnectionEstablished = "E"
	NotEnoughParents      = "Z"
)

// eventTypeDict maps short event names to human readable form
var eventTypeDict = map[string]string{
	UnitCreated:           "new regular unit created",
	PrimeUnitCreated:      "new prime unit created",
	NewTimingUnit:         "new timing unit",
	LinearOrderExtended:   "linear order extended",
	ConnectionReceived:    "listener received a TCP connection",
	ConnectionEstablished: "dialer established a TCP connection",
	NotEnoughParents:      "creating.NewUnit failed (not enough parents)",
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
	PID     = "P"
	SID     = "Y"
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
	PID:     "PID",
	SID:     "SyncID",
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
	CreateService:   "CREATE",
	OrderService:    "ORDER",
	SyncService:     "SYNC",
	ValidateService: "VALID",
	GenerateService: "GENER",
}

// Genesis was better with Phil Collins
const Genesis = "genesis"
