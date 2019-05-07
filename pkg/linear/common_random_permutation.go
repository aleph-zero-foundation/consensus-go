package linear

// CommonRandomPermutation represents random permutation shared between processes.
type CommonRandomPermutation interface {
	Get(level int) []int
}
