package logging

// Shortcuts for event types.
// Any event that happens multiple times should have a single character representation.
const (
	// Frequent events
	UnitCreated           = "A"
	CreatorNotReady       = "B"
	CreatorProcessingUnit = "C"
	SendingUnitToCreator  = "D"
	AddPreunits           = "E"
	PreunitReady          = "F"
	UnitAdded             = "G"
	ReadyToAdd            = "H"
	DuplicatedUnits       = "I"
	DuplicatedPreunits    = "J"
	UnknownParents        = "K"
	NewTimingUnit         = "L"
	OwnUnitOrdered        = "M"
	LinearOrderExtended   = "N"
	UnitOrdered           = "O"
	SentUnit              = "P"
	PreunitReceived       = "Q"
	SyncStarted           = "R"
	SyncCompleted         = "S"
	GetInfo               = "T"
	SendInfo              = "U"
	GetUnits              = "V"
	SendUnits             = "W"
	PreblockProduced      = "Y"
	// Rare events
	NewEpoch              = "a"
	EpochEnd              = "b"
	SkippingEpoch         = "c"
	ServiceStarted        = "d"
	ServiceStopped        = "e"
	GotWTK                = "f"
	CreatorFinished       = "g"
	ForkDetected          = "h"
	MissingRandomBytes    = "i"
	InvalidControlHash    = "j"
	InvalidEpochProof     = "k"
	InvalidCreator        = "l"
	FreezedParent         = "m"
	FutureLastTiming      = "n"
	UnableToRetrieveEpoch = "o"
	RequestOverload       = "p"
)

// eventTypeDict maps short event names to human readable form.
var eventTypeDict = map[string]string{
	UnitCreated:           "new unit created",
	CreatorNotReady:       "creator not ready after update",
	CreatorProcessingUnit: "creator processing a unit from the belt",
	SendingUnitToCreator:  "putting a newly added unit on creator's belt",
	AddPreunits:           "putting preunits in adder started",
	PreunitReady:          "adding a ready waiting preunit started",
	UnitAdded:             "unit added to the dag",
	ReadyToAdd:            "added ready waiting preunits",
	DuplicatedUnits:       "trying to add units already present in dag",
	DuplicatedPreunits:    "trying to add preunits already present in adder",
	UnknownParents:        "trying to add a unit with missing parents",
	NewTimingUnit:         "new timing unit",
	OwnUnitOrdered:        "unit created by this process has been ordered",
	LinearOrderExtended:   "linear order extended",
	UnitOrdered:           "unit ordered",
	SentUnit:              "sent a unit through multicast",
	PreunitReceived:       "multicast has received a preunit",
	SyncStarted:           "new sync started",
	SyncCompleted:         "sync completed",
	GetInfo:               "receiving dag info started",
	SendInfo:              "sending dag info started",
	GetUnits:              "receiving preunits started",
	SendUnits:             "sending units started",
	PreblockProduced:      "new preblock",

	NewEpoch:              "new epoch",
	EpochEnd:              "epoch finished",
	SkippingEpoch:         "creator skipping epoch without finishing it",
	ServiceStarted:        "STARTED",
	ServiceStopped:        "STOPPED",
	GotWTK:                "received weak threshold key from the setup phase",
	CreatorFinished:       "creator has finished its work",
	ForkDetected:          "fork detected in adder",
	MissingRandomBytes:    "too early to choose the next timing unit, no random bytes for required level",
	InvalidControlHash:    "invalid control hash",
	InvalidEpochProof:     "invalid epoch's proof in a unit from a future epoch",
	InvalidCreator:        "invalid creator of a unit",
	FreezedParent:         "creator freezed a parent due to some non-compliance",
	FutureLastTiming:      "creator received timing unit from newer epoch that he's seen",
	UnableToRetrieveEpoch: "unable to retrieve an epoch",
	RequestOverload:       "sync server overloaded with requests",
}

// Field names.
const (
	Sent              = "A"
	Recv              = "B"
	Creator           = "C"
	ID                = "D"
	Epoch             = "E"
	ControlHash       = "F"
	Height            = "H"
	ISID              = "I"
	WTKShareProviders = "J"
	WTKThreshold      = "K"
	LogLevel          = "L"
	Message           = "M"
	Size              = "N"
	OSID              = "O"
	PID               = "P"
	Level             = "Q"
	Round             = "R"
	Service           = "S"
	Time              = "T"
)

// fieldNameDict maps short field names to human readable form.
var fieldNameDict = map[string]string{
	Sent:              "sent",
	Recv:              "received",
	Creator:           "creator",
	ID:                "ID",
	Epoch:             "epoch",
	ControlHash:       "hash",
	Height:            "height",
	ISID:              "inSID",
	WTKShareProviders: "wtkSP",
	WTKThreshold:      "wtkThr",
	LogLevel:          "lvl",
	Message:           "msg",
	Size:              "size",
	OSID:              "outSID",
	PID:               "PID",
	Level:             "level",
	Round:             "round",
	Service:           "service",
	Time:              "time",
}

// Service types.
const (
	CreatorService int = iota
	OrderService
	AdderService
	ExtenderService
	GossipService
	FetchService
	MCService
	RMCService
	AlertService
	NetworkService
)

// serviceTypeDict maps integer service types to human readable names.
var serviceTypeDict = map[int]string{
	CreatorService:  "CREATOR",
	OrderService:    "ORDERER",
	AdderService:    "ADDER",
	ExtenderService: "EXTENDER",
	GossipService:   "GOSSIP",
	FetchService:    "FETCH",
	MCService:       "MCAST",
	RMCService:      "RMC",
	AlertService:    "ALERT",
	NetworkService:  "NETWORK",
}

// Genesis was better with Phil Collins.
const Genesis = "genesis"
