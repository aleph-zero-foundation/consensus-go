package tests

import (
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

func checkParentsConsistency(dag *Dag, pu gomel.Preunit) bool {
	parents := dag.Get(pu.Parents())
	for i := uint16(0); i < dag.NProc(); i++ {
		for j := uint16(0); j < dag.NProc(); j++ {
			if parents[j] == nil {
				continue
			}
			u := parents[j].Parents()[i]
			if parents[i] == nil {
				if u != nil {
					return false
				}
				continue
			}
			if parents[i].Below(u) && *u.Hash() != *parents[i].Hash() {
				return false
			}
		}
	}
	return true
}
