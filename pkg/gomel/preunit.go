package gomel

// Preunit represents a unit which does not (yet) belong to a dag, so either just created or transferred through the network.
type Preunit interface {
	BaseUnit
	// SetSignature sets a signature of this preunit.
	SetSignature(Signature)
}

// DealingHeights returns a slice of ints of given length containing -1 at each position.
// It is the correct slice of heights of parents for a dealing unit.
func DealingHeights(nProc uint16) []int {
	result := make([]int, nProc)
	for i := range result {
		result[i] = -1
	}
	return result
}
