package check

import (
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

// ParentConsistency checks the consistency rule.
func ParentConsistency(dag gomel.Dag) gomel.Dag {
	return Units(dag, func(u gomel.Unit) error { return parentConsistencyCheck(u.Parents(), dag.NProc()) })
}

func parentConsistencyCheck(parents []gomel.Unit, nProc uint16) error {
	for i := uint16(0); i < nProc; i++ {
		for j := uint16(0); j < nProc; j++ {
			if parents[j] == nil {
				continue
			}
			u := parents[j].Parents()[i]
			if parents[i] == nil {
				if u != nil {
					return gomel.NewComplianceError("parent consistency rule violated")
				}
				continue
			}
			if parents[i].Below(u) && *u.Hash() != *parents[i].Hash() {
				return gomel.NewComplianceError("parent consistency rule violated")
			}
		}
	}
	return nil
}

// NoSelfForkingEvidence checks if a unit does not provide evidence of its creator forking.
func NoSelfForkingEvidence(dag gomel.Dag) gomel.Dag {
	return Units(dag, func(u gomel.Unit) error { return noSelfForkingEvidenceCheck(u.Parents(), u.Creator()) })
}

func noSelfForkingEvidenceCheck(parents []gomel.Unit, creator uint16) error {
	if gomel.HasSelfForkingEvidence(parents, creator) {
		return gomel.NewComplianceError("A unit is evidence of self forking")
	}
	return nil
}

// ForkerMuting checks if the set of units respects the forker-muting policy.
func ForkerMuting(dag gomel.Dag) gomel.Dag {
	return Units(dag, func(u gomel.Unit) error { return ForkerMutingCheck(u.Parents()) })
}

// ForkerMutingCheck checks if the set of units respects the forker-muting policy, i.e.:
// The following situation is not allowed:
//   - There exists a process j, s.t. one of parents was created by j
//   AND
//   - one of the parents has evidence that j is forking.
func ForkerMutingCheck(parents []gomel.Unit) error {
	for _, parent1 := range parents {
		if parent1 == nil {
			continue
		}
		for _, parent2 := range parents {
			if parent2 == nil {
				continue
			}
			if parent1 == parent2 {
				continue
			}
			if gomel.HasForkingEvidence(parent1, parent2.Creator()) {
				return gomel.NewComplianceError("Some parent has evidence of another parent being a forker")
			}
		}
	}
	return nil
}
