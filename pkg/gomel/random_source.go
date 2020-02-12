package gomel

// RandomSource represents a source of randomness needed to run the protocol.
// It specifies what kind of data should be included in units,
// and can use this data to generate random bytes.
type RandomSource interface {
	// RandomBytes returns random bytes for a given process and level.
	RandomBytes(uint16, int) []byte
	// DataToInclude returns data which should be included in a unit
	// with the given creator and set of parents.
	DataToInclude(uint16, []Unit, int) ([]byte, error)
}

// RandomSourceFactory produces RandomSource for the given dag
type RandomSourceFactory interface {
	NewRandomSource(Dag) RandomSource
}
