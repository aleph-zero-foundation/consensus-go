package gomel

// RandomSource is an interface for sharing random source between units
type RandomSource interface {
	// GetCRP returns common random permutation for a given nonce
	GetCRP(int) []int
	// RandomBytes returns a random bits for a given unit and nonce
	RandomBytes(Unit, int) []byte
	// Update updates the RandomSource with data included in the preunit
	Update(Preunit) error
	// Rollback rolls back an update
	Rollback(Preunit)
	// DataToInclude returns data which should be included in the unit under
	// creation with given creator and set of parents
	DataToInclude(creator int, parents []Unit, level int) []byte
}
