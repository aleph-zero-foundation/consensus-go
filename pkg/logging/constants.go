package logging

// Shortcuts for event types.
// Any event that happens multiple times should have a single character representation
const (
	ServiceStarted        = "A"
	ServiceStopped        = "B"
	UnitCreated           = "C"
	PrimeUnitCreated      = "D"
	NewTimingUnit         = "E"
	LinearOrderExtended   = "F"
	ConnectionReceived    = "G"
	ConnectionEstablished = "H"
	NotEnoughParents      = "I"
	SyncStarted           = "J"
	SyncCompleted         = "K"
	GetPosetInfo          = "L"
	SendPosetInfo         = "M"
	GetPreunits           = "N"
	SendUnits             = "O"
	GetRequests           = "P"
	SendRequests          = "Q"
	AdditionalExchange    = "R"
	AddUnits              = "S"
	SentUnits             = "T"
	ReceivedPreunits      = "U"
)

// eventTypeDict maps short event names to human readable form
var eventTypeDict = map[string]string{
	ServiceStarted:        "service started",
	ServiceStopped:        "service stopped",
	UnitCreated:           "new regular unit created",
	PrimeUnitCreated:      "new prime unit created",
	NewTimingUnit:         "new timing unit",
	LinearOrderExtended:   "linear order extended",
	ConnectionReceived:    "listener received a TCP connection",
	ConnectionEstablished: "dialer established a TCP connection",
	NotEnoughParents:      "creating.NewUnit failed (not enough parents)",
	SyncStarted:           "new sync started",
	SyncCompleted:         "sync completed",
	GetPosetInfo:          "receiving poset info started",
	SendPosetInfo:         "sending poset info started",
	GetPreunits:           "receiving preunits started",
	SendUnits:             "sending units started",
	GetRequests:           "receiving requests started",
	SendRequests:          "sending requests started",
	AdditionalExchange:    "additional sync exchange needed",
	AddUnits:              "adding received units started",
	SentUnits:             "successfully sent units",
	ReceivedPreunits:      "successfully received preunits",
}

// Field names
const (
	Time      = "T"
	Level     = "L"
	Event     = "E"
	Service   = "S"
	Size      = "N"
	Height    = "H"
	Round     = "R"
	PID       = "P"
	ISID      = "I"
	OSID      = "O"
	UnitsSent = "U"
	UnitsRecv = "V"
)

// fieldNameDict maps short field names to human readable form
var fieldNameDict = map[string]string{
	Time:      "time",
	Level:     "level",
	Event:     "event",
	Service:   "service",
	Size:      "size",
	Height:    "height",
	Round:     "round",
	PID:       "PID",
	ISID:      "inSyncID",
	OSID:      "outSyncID",
	UnitsSent: "units sent",
	UnitsRecv: "units received",
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
