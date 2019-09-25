package gomel

// BaseUnit defines the most general interface for units.
type BaseUnit interface {
	// Creator is the id of the process that created this unit.
	Creator() uint16
	// Signature of this unit.
	Signature() Signature
	// Hash value of this unit.
	Hash() *Hash
	// ControlHash value of this unit.
	ControlHash() *Hash
	// Data is the slice of data contained in the unit.
	Data() []byte
	// RandomSourceData is data contained in the unit needed to maintain
	// the common random source among processes.
	RandomSourceData() []byte
}

// Nickname of a unit is a short name, for the purpose of quick identification by a human.
func Nickname(bu BaseUnit) string {
	return bu.Hash().Short()
}
