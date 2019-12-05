package logging

// Shortcuts for event types.
// Any event that happens multiple times should have a single character representation.
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
	GetDagInfo            = "L"
	SendDagInfo           = "M"
	GetPreunits           = "N"
	SendUnits             = "O"
	GetRequests           = "P"
	SendRequests          = "Q"
	AdditionalExchange    = "R"
	AddUnits              = "S"
	SentUnits             = "T"
	ReceivedPreunits      = "U"
	DuplicateUnit         = "V"
	DuplicatePreunit      = "W"
	ConnectionClosed      = "X"
	MemoryUsage           = "Y"
	DataValidated         = "Z"
	TooManyIncoming       = "a"
	TooManyOutgoing       = "b"
	SendFreshUnits        = "c"
	SentFreshUnits        = "d"
	UnitBroadcasted       = "e"
	UnknownParents        = "f"
	AddedBCUnit           = "g"
	AddingStarted         = "h"
	DecodeParentsError    = "i"
	CheckError            = "j"
	GotRandomSource       = "k"
	OwnUnitOrdered        = "l"
	UnitAdded             = "m"
	AddUnitStarted        = "n"
	AddUnitsStarted       = "o"
	PreunitReady          = "p"
	FetchParents          = "r"
)

// eventTypeDict maps short event names to human readable form.
var eventTypeDict = map[string]string{
	ServiceStarted:        "service started",
	ServiceStopped:        "service stopped",
	UnitCreated:           "new regular unit created",
	PrimeUnitCreated:      "new prime unit created",
	NewTimingUnit:         "new timing unit",
	LinearOrderExtended:   "linear order extended",
	ConnectionReceived:    "listener received a TCP connection",
	ConnectionEstablished: "dialer established a TCP connection",
	NotEnoughParents:      "creating new unit failed (not enough parents)",
	SyncStarted:           "new sync started",
	SyncCompleted:         "sync completed",
	GetDagInfo:            "receiving dag info started",
	SendDagInfo:           "sending dag info started",
	GetPreunits:           "receiving preunits started",
	SendUnits:             "sending units started",
	GetRequests:           "receiving requests started",
	SendRequests:          "sending requests started",
	AdditionalExchange:    "additional sync exchange needed",
	AddUnits:              "adding received units started",
	SentUnits:             "successfully sent units",
	ReceivedPreunits:      "successfully received preunits",
	DuplicateUnit:         "trying to add unit already present in dag",
	DuplicatePreunit:      "trying to add unit already present in adder",
	ConnectionClosed:      "connection closed after sync (stats in bytes)",
	MemoryUsage:           "memory usage statistics",
	DataValidated:         "validated some bytes of data",
	TooManyIncoming:       "too many incoming connections",
	TooManyOutgoing:       "too many outgoing connections",
	SendFreshUnits:        "sending fresh units started",
	SentFreshUnits:        "sending fresh units finished",
	UnitBroadcasted:       "sent a unit through multicast",
	UnknownParents:        "trying to add a unit with missing parents",
	AddedBCUnit:           "unit from multicast was put into adder",
	AddingStarted:         "adding a ready waiting preunit started",
	DecodeParentsError:    "DecodeParents error, passing it to error handlers",
	CheckError:            "Check error, passing it to error handlers",
	GotRandomSource:       "received randomness source",
	OwnUnitOrdered:        "unit created by this process has been ordered",
	UnitAdded:             "unit successfully added to the dag",
	AddUnitStarted:        "adding a single unit received from PID started",
	AddUnitsStarted:       "adding a chunk of units received from PID started",
	PreunitReady:          "waiting preunit sent to the adding worker",
	FetchParents:          "new waiting preunit with some missing parents",
}

// Field names.
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
	Sent      = "U"
	FreshSent = "F"
	Recv      = "V"
	FreshRecv = "G"
	Creator   = "C"
	Memory    = "M"
	ID        = "D"
	Hash      = "#"
	Lvl       = "Q"
)

// fieldNameDict maps short field names to human readable form.
var fieldNameDict = map[string]string{
	Time:      "time",
	Level:     "level",
	Event:     "event",
	Service:   "service",
	Size:      "size",
	Height:    "height",
	Round:     "round",
	PID:       "PID",
	ISID:      "inSID",
	OSID:      "outSID",
	Sent:      "sent",
	FreshSent: "fresh s",
	Recv:      "received",
	FreshRecv: "fresh r",
	Creator:   "creator",
	Memory:    "bytes",
	ID:        "ID",
	Hash:      "hash",
	Lvl:       "level",
}

// Service types.
const (
	CreateService int = iota
	OrderService
	SyncService
	ValidateService
	MemLogService
	GossipService
	FetchService
	MCService
	RetryingService
	RMCService
	AlertService
	AdderService
)

// serviceTypeDict maps integer service types to human readable names.
var serviceTypeDict = map[int]string{
	CreateService:   "CREATE",
	OrderService:    "ORDER",
	SyncService:     "SYNC",
	ValidateService: "VALID",
	MemLogService:   "MEMLOG",
	GossipService:   "GOSSIP",
	FetchService:    "FETCH",
	MCService:       "MCAST",
	RetryingService: "RETRY",
	RMCService:      "RMC",
	AlertService:    "ALERT",
	AdderService:    "ADDER",
}

// Genesis was better with Phil Collins.
const Genesis = "genesis"
