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
	DuplicatedUnit        = "V"
	OwnUnitOrdered        = "W"
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
	AddedToBacklog        = "h"
	RemovedFromBacklog    = "i"
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
	NotEnoughParents:      "creating.NewUnit failed (not enough parents)",
	SyncStarted:           "new sync started",
	SyncCompleted:         "sync completed (stats = units)",
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
	DuplicatedUnit:        "attempting to add unit already present in dag",
	OwnUnitOrdered:        "unit created by this process has been ordered",
	ConnectionClosed:      "connection closed after sync (stats = bytes)",
	MemoryUsage:           "memory usage statistics",
	DataValidated:         "validated some bytes of data",
	TooManyIncoming:       "too many incoming connections",
	TooManyOutgoing:       "too many outgoing connections",
	SendFreshUnits:        "sending fresh units started",
	UnitBroadcasted:       "sent a unit through multicast",
	UnknownParents:        "unable to add unit due to missing parents",
	AddedBCUnit:           "successfully added unit from multicast",
	AddedToBacklog:        "added unit to retrying backlog",
	RemovedFromBacklog:    "removed unit from retrying backlog",
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
	NParents  = "A"
	Memory    = "M"
	ID        = "D"
	Hash      = "#"
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
	NParents:  "parents",
	Memory:    "bytes",
	ID:        "ID",
	Hash:      "hash",
}

// Service types.
const (
	CreateService int = iota
	OrderService
	SyncService
	ValidateService
	GenerateService
	MemLogService
	GossipService
	FetchService
	MCService
	RetryingService
)

// serviceTypeDict maps integer service types to human readable names.
var serviceTypeDict = map[int]string{
	CreateService:   "CREATE",
	OrderService:    "ORDER",
	SyncService:     "SYNC",
	ValidateService: "VALID",
	GenerateService: "GENER",
	MemLogService:   "MEMLOG",
	GossipService:   "GOSSIP",
	FetchService:    "FETCH",
	MCService:       "MCAST",
	RetryingService: "RETRY",
}

// Genesis was better with Phil Collins.
const Genesis = "genesis"
