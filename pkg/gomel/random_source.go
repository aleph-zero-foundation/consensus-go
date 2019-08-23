package gomel

// RandomSource represents a source of randomness needed to run the protocol.
// It specifies what kind of data should be included in units,
// and can use this data to generate random bytes.
type RandomSource interface {
	// Init initializes the random source with the given dag.
	Init(Dag)
	// RandomBytes returns random bytes for a given process and level.
	RandomBytes(int, int) []byte
	// CheckCompliance checks whether the data included in the unit
	// is compliant.
	CheckCompliance(Unit) error
	// Update the RandomSource with the data included in the unit.
	Update(Unit)
	// DataToInclude returns data which should be included in a unit
	// with the given creator and set of parents.
	DataToInclude(creator int, parents []Unit, level int) ([]byte, error)
}
