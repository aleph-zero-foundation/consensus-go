package logging

// Shortcuts for event types.
// Any event that happens multiple times should have a single character representation.
const (
	ServiceStarted                = "A"
	ServiceStopped                = "B"
	GotWTK                        = "C"
	UnitCreated                   = "D"
	NewEpoch                      = "E"
	EpochEnd                      = "F"
	NewTimingUnit                 = "G"
	OwnUnitOrdered                = "H"
	LinearOrderExtended           = "I"
	UnitAdded                     = "J"
	AddUnits                      = "K"
	AddingStarted                 = "L"
	ForkDetected                  = "M"
	UnitBroadcasted               = "N"
	SyncStarted                   = "O"
	SyncCompleted                 = "P"
	GetInfo                       = "Q"
	SendInfo                      = "R"
	GetUnits                      = "S"
	SendUnits                     = "T"
	SuccesfulAdd                  = "U"
	DuplicatedUnits               = "V"
	DuplicatedPreunits            = "W"
	UnknownParents                = "X"
	MissingRandomBytes            = "Y"
	UnitOrdered                   = "Z"
	InvalidControlHash            = "AA"
	InvalidEpochProofFromFuture   = "AB"
	CreatorFinished               = "AC"
	InvalidCreator                = "AD"
	CreatorSwitchedToNewEpoch     = "AF"
	FreezedParent                 = "AG"
	UnableToRetrieveEpoch         = "AH"
	CreatorProcessingUnit         = "AI"
	PuttingOnCreatorsBelt         = "AL"
	CreatorNotReadyAfterUpdate    = "AM"
	CreatorNotReady               = "AN"
	BeforeSendingUnitToCreator    = "AO"
	MulticastReceivedPreunit      = "AQ"
	LastTimingUnitIsFromTheFuture = "AR"
)

// eventTypeDict maps short event names to human readable form.
var eventTypeDict = map[string]string{
	ServiceStarted:                "service started",
	ServiceStopped:                "service stopped",
	GotWTK:                        "received weak threshold key from the setup phase",
	UnitCreated:                   "new unit created",
	NewEpoch:                      "new epoch started",
	EpochEnd:                      "epoch finished",
	NewTimingUnit:                 "new timing unit",
	OwnUnitOrdered:                "unit created by this process has been ordered",
	LinearOrderExtended:           "linear order extended",
	UnitAdded:                     "unit added to the dag",
	AddUnits:                      "adding units started",
	AddingStarted:                 "adding a ready waiting preunit started",
	ForkDetected:                  "fork detected in adder",
	UnitBroadcasted:               "sent a unit through multicast",
	SyncStarted:                   "new sync started",
	SyncCompleted:                 "sync completed",
	GetInfo:                       "receiving dag info started",
	SendInfo:                      "sending dag info started",
	GetUnits:                      "receiving preunits started",
	SendUnits:                     "sending units started",
	SuccesfulAdd:                  "added ready waiting preunits",
	DuplicatedUnits:               "trying to add units already present in dag",
	DuplicatedPreunits:            "trying to add preunits already present in adder",
	UnknownParents:                "trying to add a unit with missing parents",
	MissingRandomBytes:            "missing random bytes",
	UnitOrdered:                   "unit ordered",
	InvalidControlHash:            "invalid control hash",
	InvalidEpochProofFromFuture:   "invalid epoch's proof in a unit from a future epoch",
	CreatorFinished:               "creator has finished its work",
	InvalidCreator:                "invalid creator of a unit",
	CreatorSwitchedToNewEpoch:     "creator switched to a new epoch",
	FreezedParent:                 "creator freezed a parent due to some non-compliance",
	UnableToRetrieveEpoch:         "unable to retrieve an epoch",
	CreatorProcessingUnit:         "creator is processing a unit",
	PuttingOnCreatorsBelt:         "unit was put on creator's belt",
	CreatorNotReady:               "creator is not ready after update",
	CreatorNotReadyAfterUpdate:    "creator was ready after update, but changed its mind after more updates",
	BeforeSendingUnitToCreator:    "called AfterInsert responsible for sending on creator's belt",
	LastTimingUnitIsFromTheFuture: "TIME TRAVEL ERROR: lastTiming received a unit from the future",
	MulticastReceivedPreunit:      "mulicast has received a preunit",
}

// Field names.
const (
	Time              = "T"
	Level             = "L"
	Event             = "V"
	Service           = "S"
	Size              = "N"
	Creator           = "C"
	Height            = "H"
	Epoch             = "E"
	Lvl               = "Q"
	Round             = "R"
	ID                = "D"
	PID               = "P"
	ISID              = "I"
	OSID              = "O"
	Sent              = "A"
	Recv              = "B"
	ControlHash       = "Z"
	WTKThreshold      = "AJ"
	WTKShareProviders = "AK"
	MaxOnLevel        = "AP"
)

// fieldNameDict maps short field names to human readable form.
var fieldNameDict = map[string]string{
	Time:              "time",
	Level:             "level",
	Event:             "event",
	Service:           "service",
	Size:              "size",
	Creator:           "creator",
	Height:            "height",
	Epoch:             "epoch",
	Lvl:               "level",
	Round:             "round",
	ID:                "ID",
	PID:               "PID",
	ISID:              "inSID",
	OSID:              "outSID",
	Sent:              "sent",
	Recv:              "received",
	WTKThreshold:      "wtk-threshold",
	WTKShareProviders: "wtk-share_providers",
	MaxOnLevel:        "max_on_level",
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
	EpochService
	NetworkService
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
	EpochService:    "EPOCH",
	NetworkService:  "NETWORK",
}

// Genesis was better with Phil Collins.
const Genesis = "genesis"

// names for logger
const (
	TimestampFieldName = "aa"
	LevelFieldName     = "ab"
	MessageFieldName   = "ac"
)
