package gomel

// BaseUnit defines the most general interface for units.
type BaseUnit interface {
	// Creator is the id of the process that created this unit.
	Creator() uint16
	// Signature of this unit.
	Signature() Signature
	// Hash value of this unit.
	Hash() *Hash
	// Height of a unit is the length of the path between this unit and a dealing unit in the (induced) sub-dag containing all units produced by the same creator.
	Height() int
	// View returns the crown of the dag below the unit.
	View() *Crown
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
