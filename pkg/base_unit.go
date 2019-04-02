package gomel

// Common properties of all unit-like entities.
type BaseUnit interface {
	Creator() int
	Hash() *Hash
}

// A printable short name for the unit, for quick identification.
func Nickname(bu BaseUnit) string {
	return bu.Hash().Short()
}
