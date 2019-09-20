package gomel

// RandomSource represents a source of randomness needed to run the protocol.
// It specifies what kind of data should be included in units,
// and can use this data to generate random bytes.
type RandomSource interface {
	// Bind the dag to the random source. The resulting dag should be used for adding any units.
	Bind(Dag) Dag
	// RandomBytes returns random bytes for a given process and level.
	RandomBytes(uint16, int) []byte
	// DataToInclude returns data which should be included in a unit
	// with the given creator and set of parents.
	DataToInclude(creator uint16, parents []Unit, level int) ([]byte, error)
}
