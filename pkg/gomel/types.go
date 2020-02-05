package gomel

// EpochID is used as a unique identifier of an epoch.
type EpochID uint32

// Hash is a type storing hash values, usually used to identify units.
type Hash [32]byte

// ErrorHandler is a function that processes errors encountered while checking a newly built unit.
// The last argument is the process ID who sent us that unit.
// If it cannot process a particular error, it should return it for further handling.
type ErrorHandler func(error, Unit, uint16) error

// UnitChecker is a function that performs a check on Unit before Prepare.
type UnitChecker func(Unit, Dag) error

// InsertHook is a function that performs some additional action on a unit before or after Insert.
type InsertHook func(Unit)
