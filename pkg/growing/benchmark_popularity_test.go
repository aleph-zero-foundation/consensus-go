package growing

import (
	"testing"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/tests"
)

func provesPopularityBFS(uc gomel.Unit, v gomel.Unit, dag gomel.Dag) bool {
	if uc.Level() >= v.Level() || !uc.Below(v) {
		return false
	}
	// simple BFS from v
	seenProcesses := make([]bool, dag.NProc())
	nSeenProcesses := 0
	seenUnits := make(map[gomel.Hash]bool)
	seenUnits[*v.Hash()] = true
	queue := []gomel.Unit{v}
	for len(queue) > 0 {
		w := queue[0]
		queue = queue[1:]
		if w.Level() <= v.Level()-2 || (w.Level() == v.Level()-1 && gomel.Prime(w)) {
			if !seenProcesses[w.Creator()] {
				seenProcesses[w.Creator()] = true
				nSeenProcesses++
				if dag.IsQuorum(nSeenProcesses) {
					return true
				}
			}
		}
		for _, wParent := range w.Parents() {
			if _, exists := seenUnits[*wParent.Hash()]; !exists && uc.Below(wParent) {
				queue = append(queue, wParent)
				seenUnits[*wParent.Hash()] = true
			}
		}
	}
	result := dag.IsQuorum(nSeenProcesses)
	return result
}

func provesPopularityWithFloors(uc gomel.Unit, v gomel.Unit, dag gomel.Dag) bool {
	if uc.Level() >= v.Level() || !uc.Below(v) {
		return false
	}
	level := v.Level()
	nSeen := 0
	nNotSeen := dag.NProc()
	for _, myFloor := range v.(*unit).floor {
		nNotSeen--
		for _, w := range myFloor {
			var reachedBottom error
			for w.Above(uc) && ((w.Level() > level-2) && (w.Level() != level-1 || !gomel.Prime(w))) {
				var wPre gomel.Unit
				wPre, reachedBottom = gomel.Predecessor(w)
				if reachedBottom != nil {
					break
				}
				w = wPre.(*unit)
			}
			if reachedBottom == nil && w.Above(uc) && ((w.Level() == level-2) || ((w.Level() == level-1) && gomel.Prime(w))) {
				nSeen++
				if dag.IsQuorum(nSeen) {
					return true
				}
				break
			}
		}
		if !dag.IsQuorum(nSeen + nNotSeen) {
			return false
		}
	}
	return dag.IsQuorum(nSeen)
}

func BenchmarkPopularity(b *testing.B) {
	var (
		dag        gomel.Dag
		readingErr error
		pf         dagFactory
		units      map[int]map[int][]gomel.Unit
	)
	testfiles := []string{
		"random_10p_100u_2par.txt",
		"random_100p_5000u.txt",
	}
	for _, testfile := range testfiles {
		dag, readingErr = tests.CreateDagFromTestFile("../testdata/"+testfile, pf)

		if readingErr != nil {
			panic(readingErr)
			return
		}
		units = collectUnits(dag)
		unitsByLevel := make(map[int][]gomel.Unit)
		maxLevel := 0
		for pid := range units {
			for h := range units[pid] {
				for _, u := range units[pid][h] {
					level := u.Level()
					if level > maxLevel {
						maxLevel = level
					}
					unitsByLevel[level] = append(unitsByLevel[level], u)
				}
			}
		}

		b.Run("By bfs "+testfile, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				for level := maxLevel - 3; level <= maxLevel; level++ {
					units := unitsByLevel[level]
					for l := level - 2; l >= level-4; l-- {
						for _, uc := range unitsByLevel[l] {
							for _, u := range units {
								provesPopularityBFS(uc, u, dag)
							}
						}
					}
				}
			}
		})
		b.Run("With floors "+testfile, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				for level := maxLevel - 3; level <= maxLevel; level++ {
					units := unitsByLevel[level]
					for l := level - 2; l >= level-4; l-- {
						for _, uc := range unitsByLevel[l] {
							for _, u := range units {
								provesPopularityWithFloors(uc, u, dag)
							}
						}
					}
				}
			}
		})
	}
}
