package gomel

// Random permutation shared between processes
type CommonRandomPermutation interface {
	Get(level int) []int
}
