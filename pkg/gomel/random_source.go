package gomel

// RandomSource represents a source of randomness needed to run the protocol.
// It specifies what kind of data should be included in units,
// and can use this data to generate random bytes.
type RandomSource interface {
	// RandomBytes returns random bytes for a given process and level.
	RandomBytes(uint16, int) []byte
	// DataToInclude returns data which should be included in a unit based on its level and parents.
	DataToInclude([]Unit, int) ([]byte, error)
}

// RandomSourceFactory produces RandomSource for the given dag
type RandomSourceFactory interface {
	// NewRandomSource produces a randomness source for the provided dag.
	NewRandomSource(Dag) RandomSource
	// DealingData returns random source data that should be included in the dealing unit for the given epoch.
	DealingData(EpochID) ([]byte, error)
}
