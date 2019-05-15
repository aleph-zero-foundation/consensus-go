package linear

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
)

// CommonRandomPermutation represents random permutation shared between processes.
type CommonRandomPermutation interface {
	Get(level int) []int
}

type commonRandomPermutation struct {
	n int
}

func (crp *commonRandomPermutation) Get(level int) []int {
	permutation := make([]int, crp.n, crp.n)
	for i := 0; i < crp.n; i++ {
		permutation[i] = (i + level) % crp.n
	}
	return permutation
}

func NewCommonRandomPermutation(p gomel.Poset) CommonRandomPermutation {
	return &commonRandomPermutation{
		n: p.NProc(),
	}
}
