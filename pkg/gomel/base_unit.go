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
	Data() Data
	// RandomSourceData is data contained in the unit needed to maintain
	// the common random source among processes.
	RandomSourceData() []byte
}

// Nickname of a unit is a short name, for the purpose of quick identification by a human.
func Nickname(bu BaseUnit) string {
	return bu.Hash().Short()
}

// ID is a pair (Height, Creator) encoded as a single number.
func ID(height int, creator, nProc uint16) uint64 {
	return uint64(creator) + uint64(nProc)*uint64(height)
}

// DecodeID that is a single number into a pair (Height, Creator).
func DecodeID(id uint64, nProc uint16) (int, uint16) {
	return int(id / uint64(nProc)), uint16(id % uint64(nProc))
}

// UnitID returns ID of the given BaseUnit.
func UnitID(u BaseUnit) uint64 {
	return ID(u.Height(), u.Creator(), uint16(len(u.View().Heights)))
}

// Equal checks if two units are the same.
func Equal(u, v BaseUnit) bool {
	return u.Creator() == v.Creator() && u.Height() == v.Height() && *u.Hash() == *v.Hash()
}
