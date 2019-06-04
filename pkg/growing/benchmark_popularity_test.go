package growing

import (
	"testing"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/tests"
)

func provesPopularityBFS(uc gomel.Unit, v gomel.Unit, poset gomel.Poset) bool {
	if uc.Level() >= v.Level() || !uc.Below(v) {
		return false
	}
	// simple BFS from v
	seenProcesses := make(map[int]bool)
	seenUnits := make(map[gomel.Hash]bool)
	seenUnits[*v.Hash()] = true
	queue := []gomel.Unit{v}
	for len(queue) > 0 {
		w := queue[0]
		queue = queue[1:]
		if w.Level() <= v.Level()-2 || (w.Level() == v.Level()-1 && gomel.Prime(w)) {
			seenProcesses[w.Creator()] = true
			if poset.IsQuorum(len(seenProcesses)) {
				return true
			}
		}
		for _, wParent := range w.Parents() {
			if _, exists := seenUnits[*wParent.Hash()]; !exists && uc.Below(wParent) {
				queue = append(queue, wParent)
				seenUnits[*wParent.Hash()] = true
			}
		}
	}
	result := poset.IsQuorum(len(seenProcesses))
	return result
}

func provesPopularityWithFloors(uc gomel.Unit, v gomel.Unit, poset gomel.Poset) bool {
	if uc.Level() >= v.Level() || !uc.Below(v) {
		return false
	}
	level := v.Level()
	nSeen := 0
	nNotSeen := poset.NProc()
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
				if poset.IsQuorum(nSeen) {
					return true
				}
				break
			}
		}
		if !poset.IsQuorum(nSeen + nNotSeen) {
			return false
		}
	}
	return poset.IsQuorum(nSeen)
}

func BenchmarkPopularity(b *testing.B) {
	var (
		poset      gomel.Poset
		readingErr error
		pf         posetFactory
		units      map[int]map[int][]gomel.Unit
	)
	testfiles := []string{
		"random_10p_100u_2par.txt",
		"random_100p_5000u.txt",
	}
	for _, testfile := range testfiles {
		poset, readingErr = tests.CreatePosetFromTestFile("../testdata/"+testfile, pf)

		if readingErr != nil {
			panic(readingErr)
			return
		}
		units = collectUnits(poset)
		unitsByLevel := make(map[int][]gomel.Unit)
		maxLevel := 0
		for pid := range units {
			for h := range units[pid] {
				for _, u := range units[pid][h] {
					level := u.Level()
					if level > maxLevel {
						maxLevel = level
					}
					if _, ok := unitsByLevel[level]; ok {
						unitsByLevel[level] = append(unitsByLevel[level], u)
					} else {
						unitsByLevel[level] = []gomel.Unit{u}
					}
				}
			}
		}

		b.Run("By bfs "+testfile, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				for level := maxLevel - 3; level <= maxLevel; level++ {
					units := unitsByLevel[level]
					for l := level - 3; l >= level-5; l-- {
						for _, uc := range unitsByLevel[l] {
							for _, u := range units {
								provesPopularityBFS(uc, u, poset)
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
					for l := level - 3; l >= level-5; l-- {
						for _, uc := range unitsByLevel[l] {
							for _, u := range units {
								provesPopularityWithFloors(uc, u, poset)
							}
						}
					}
				}
			}
		})
	}
}
