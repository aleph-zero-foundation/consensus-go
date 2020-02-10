package check

import (
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

// ParentConsistency checks the consistency rule.
func ParentConsistency(u gomel.Unit, dag gomel.Dag) error {
	parents := u.Parents()
	nProc := dag.NProc()
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
			if u != nil && parents[i].Level() < u.Level() {
				return gomel.NewComplianceError("parent consistency rule violated")
			}
		}
	}
	return nil
}

// NoSelfForkingEvidence checks if a unit does not provide evidence of its creator forking.
func NoSelfForkingEvidence(u gomel.Unit, _ gomel.Dag) error {
	if hasForkingEvidence(u, u.Creator()) {
		return gomel.NewComplianceError("A unit is evidence of self forking")
	}
	return nil
}

// ForkerMuting checks if unit's parents respects the forker-muting policy:
// The following situation is not allowed:
//   - There exists a process j, s.t. one of parents was created by j
//   AND
//   - one of the parents has evidence that j is forking.
func ForkerMuting(u gomel.Unit, _ gomel.Dag) error {
	for _, parent1 := range u.Parents() {
		if parent1 == nil {
			continue
		}
		for _, parent2 := range u.Parents() {
			if parent2 == nil {
				continue
			}
			if parent1 == parent2 {
				continue
			}
			if hasForkingEvidence(parent1, parent2.Creator()) {
				return gomel.NewComplianceError("Some parent has evidence of another parent being a forker")
			}
		}
	}
	return nil
}

// hasForkingEvidence checks whether the unit is sufficient evidence of the given creator forking,
// i.e. it is above two units created by creator that share a predecessor.
func hasForkingEvidence(u gomel.Unit, creator uint16) bool {
	if gomel.Dealing(u) {
		return false
	}
	f := u.Floor(creator)
	return len(f) > 1 || (len(f) == 1 && !gomel.Equal(f[0], u.Parents()[creator]))
}
