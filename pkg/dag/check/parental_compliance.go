package check

import (
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type parentalCompliance struct {
	gomel.Dag
	check func(parents []gomel.Unit) error
}

func (dag *parentalCompliance) Check(u gomel.Unit) error {
	if err := dag.Dag.Check(u); err != nil {
		return err
	}
	return dag.check(u.Parents())
}

// ParentDiversity checks if all parents are created by pairwise different processes.
func ParentDiversity(dag gomel.Dag) gomel.Dag {
	return &parentalCompliance{
		dag, parentDiversityCheck,
	}
}

func parentDiversityCheck(parents []gomel.Unit) error {
	processFilter := map[uint16]bool{}
	for _, parent := range parents {
		if processFilter[parent.Creator()] {
			return gomel.NewComplianceError("Some of a unit's parents are created by the same process")
		}
		processFilter[parent.Creator()] = true
	}
	return nil
}

// NoSelfForkingEvidence checks if a unit does not provide evidence of its creator forking.
func NoSelfForkingEvidence(dag gomel.Dag) gomel.Dag {
	return &parentalCompliance{
		dag, noSelfForkingEvidenceCheck,
	}
}

func noSelfForkingEvidenceCheck(parents []gomel.Unit) error {
	if gomel.HasSelfForkingEvidence(parents) {
		return gomel.NewComplianceError("A unit is evidence of self forking")
	}
	return nil
}

// ForkerMuting checks if the set of units respects the forker-muting policy.
func ForkerMuting(dag gomel.Dag) gomel.Dag {
	return &parentalCompliance{
		dag, ForkerMutingCheck,
	}
}

// ForkerMutingCheck checks if the set of units respects the forker-muting policy, i.e.:
// The following situation is not allowed:
//   - There exists a process j, s.t. one of parents was created by j
//   AND
//   - one of the parents has evidence that j is forking.
func ForkerMutingCheck(parents []gomel.Unit) error {
	for _, parent1 := range parents {
		for _, parent2 := range parents {
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
