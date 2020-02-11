package gomel

// EpochID is used as a unique identifier of an epoch.
type EpochID uint32

// Signature of a unit.
type Signature []byte

// Hash is a type storing hash values, usually used to identify units.
type Hash [32]byte

// UnitChecker is a function that performs a check on Unit before Prepare.
type UnitChecker func(Unit, Dag) error

// InsertHook is a function that performs some additional action on a unit before or after Insert.
type InsertHook func(Unit)

// RSFactory produces RandomSource for the given dag
type RSFactory func(Dag) RandomSource
