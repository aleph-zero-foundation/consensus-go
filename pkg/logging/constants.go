package logging

// Shortcuts for event types.
// Any event that happens multiple times should have a single character representation.
const (
	ServiceStarted      = "A"
	ServiceStopped      = "B"
	GotWTK              = "C"
	UnitCreated         = "D"
	NewEpoch            = "E"
	EpochEnd            = "F"
	NewTimingUnit       = "G"
	OwnUnitOrdered      = "H"
	LinearOrderExtended = "I"
	UnitAdded           = "J"
	AddUnits            = "K"
	AddingStarted       = "L"
	ForkDetected        = "M"
	MissingParents      = "N"
	UnitBroadcasted     = "O"
	SyncStarted         = "P"
	SyncCompleted       = "Q"
	GetInfo             = "R"
	SendInfo            = "S"
	GetUnits            = "T"
	SendUnits           = "U"
	DuplicatedUnit      = "V"
	DuplicatedPreunit   = "W"
	UnknownParents      = "X"
)

// eventTypeDict maps short event names to human readable form.
var eventTypeDict = map[string]string{
	ServiceStarted:      "service started",
	ServiceStopped:      "service stopped",
	GotWTK:              "received weak threshold key from the setup phase",
	UnitCreated:         "new unit created",
	NewEpoch:            "new epoch started",
	EpochEnd:            "epoch finished",
	NewTimingUnit:       "new timing unit",
	OwnUnitOrdered:      "unit created by this process has been ordered",
	LinearOrderExtended: "linear order extended",
	UnitAdded:           "unit added to the dag",
	AddUnits:            "adding units batch started",
	AddingStarted:       "adding a ready waiting preunit started",
	ForkDetected:        "fork detected in adder",
	MissingParents:      "new waiting preunit with some missing parents",
	UnitBroadcasted:     "sent a unit through multicast",
	SyncStarted:         "new sync started",
	SyncCompleted:       "sync completed",
	GetInfo:             "receiving dag info started",
	SendInfo:            "sending dag info started",
	GetUnits:            "receiving preunits started",
	SendUnits:           "sending units started",
	DuplicatedUnit:      "trying to add unit already present in dag",
	DuplicatedPreunit:   "trying to add unit already present in adder",
	UnknownParents:      "trying to add a unit with missing parents",
}

// Field names.
const (
	Time    = "T"
	Level   = "L"
	Event   = "V"
	Service = "S"
	Size    = "N"
	Creator = "C"
	Height  = "H"
	Epoch   = "E"
	Lvl     = "Q"
	Round   = "R"
	ID      = "D"
	PID     = "P"
	ISID    = "I"
	OSID    = "O"
	Sent    = "A"
	Recv    = "B"
)

// fieldNameDict maps short field names to human readable form.
var fieldNameDict = map[string]string{
	Time:    "time",
	Level:   "level",
	Event:   "event",
	Service: "service",
	Size:    "size",
	Creator: "creator",
	Height:  "height",
	Epoch:   "epoch",
	Lvl:     "level",
	Round:   "round",
	ID:      "ID",
	PID:     "PID",
	ISID:    "inSID",
	OSID:    "outSID",
	Sent:    "sent",
	Recv:    "received",
}

// Service types.
const (
	CreateService int = iota
	OrderService
	AdderService
	ExtenderService
	GossipService
	FetchService
	MCService
	RMCService
	AlertService
)

// serviceTypeDict maps integer service types to human readable names.
var serviceTypeDict = map[int]string{
	CreateService:   "CREATOR",
	OrderService:    "ORDERER",
	AdderService:    "ADDER",
	ExtenderService: "EXTENDER",
	GossipService:   "GOSSIP",
	FetchService:    "FETCH",
	MCService:       "MCAST",
	RMCService:      "RMC",
	AlertService:    "ALERT",
}

// Genesis was better with Phil Collins.
const Genesis = "genesis"
