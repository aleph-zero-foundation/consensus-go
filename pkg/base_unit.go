package gomel

import "gitlab.com/alephledger/consensus-go/pkg/crypto/tcoin"

// BaseUnit defines the most general interface for units.
type BaseUnit interface {
	// Creator is the id of the process that created this unit
	Creator() int
	// Signature of this unit
	Signature() Signature
	// Hash value of this unit
	Hash() *Hash
	// Data is the slice of data contained in the unit
	Data() []byte
	// CoinShare returns coin share embedded in this unit
	// it is nil for non-prime units
	CoinShare() *tcoin.CoinShare
}

// Nickname of a unit is a short name, for the purpose of quick identification by a human.
func Nickname(bu BaseUnit) string {
	return bu.Hash().Short()
}
