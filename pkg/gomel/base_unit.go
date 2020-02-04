package gomel

import "gitlab.com/alephledger/core-go/pkg/core"

// BaseUnit defines the most general interface for units.
type BaseUnit interface {
	// EpochID is used a unique identifier of a set of creators who participate in creation of a dag to which this unit belongs.
	EpochID() EpochID
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
	Data() core.Data
	// RandomSourceData is data contained in the unit needed to maintain
	// the common random source among processes.
	RandomSourceData() []byte
}

// Nickname of a unit is a short name, for the purpose of quick identification by a human.
func Nickname(bu BaseUnit) string {
	return bu.Hash().Short()
}

// ID is a tuple (Height, Creator, Epoch) encoded as a single number.
func ID(height int, creator uint16, epoch EpochID) uint64 {
	result := uint64(height)
	result += uint64(creator) << 16
	result += uint64(epoch) << 32
	return result
}

// DecodeID that is a single number into a tuple (Height, Creator, Epoch).
func DecodeID(id uint64) (int, uint16, EpochID) {
	height := int(id & (1<<16 - 1))
	id >>= 16
	creator := uint16(id & (1<<16 - 1))
	return height, creator, EpochID(id >> 16)
}

// UnitID returns ID of the given BaseUnit.
func UnitID(u BaseUnit) uint64 {
	return ID(u.Height(), u.Creator(), u.EpochID())
}

// Equal checks if two units are the same.
func Equal(u, v BaseUnit) bool {
	return u.Creator() == v.Creator() && u.Height() == v.Height() && u.EpochID() == v.EpochID() && *u.Hash() == *v.Hash()
}
