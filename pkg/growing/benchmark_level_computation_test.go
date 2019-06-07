package growing

import (
	"testing"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
	tests "gitlab.com/alephledger/consensus-go/pkg/tests"
)

type posetFactory struct{}

func (posetFactory) CreatePoset(pc gomel.PosetConfig) gomel.Poset {
	return NewPoset(&pc)
}

func levelByIteratingPrimes(u gomel.Unit, p gomel.Poset) int {
	if gomel.Dealing(u) {
		return 0
	}
	level := u.Parents()[0].Level()
	primes := p.PrimeUnits(level)
	nSeen := 0
	nNotSeen := p.NProc()
	primes.Iterate(func(units []gomel.Unit) bool {
		nNotSeen--
		for _, v := range units {
			if v.Below(u) {
				nSeen++
				if p.IsQuorum(nSeen) {
					return false
				}
				break
			}
		}
		if !p.IsQuorum(nSeen + nNotSeen) {
			return false
		}
		return true
	})
	if p.IsQuorum(nSeen) {
		return level + 1
	}
	return level
}

func levelByDFS(u gomel.Unit, poset gomel.Poset) int {
	if len(u.Parents()) == 0 {
		return 0
	}

	parents := u.Parents()
	level := parents[len(parents)-1].Level()
	procSeen := make(map[int]bool)
	unitsSeen := make(map[gomel.Hash]bool)
	stack := []gomel.Unit{u}
	for len(stack) > 0 {
		w := stack[len(stack)-1]
		stack = stack[:(len(stack) - 1)]

		for _, v := range w.Parents() {
			if v.Level() == level && !unitsSeen[*v.Hash()] {
				stack = append(stack, v)
				unitsSeen[*v.Hash()] = true
				procSeen[v.Creator()] = true
				if poset.IsQuorum(len(procSeen)) {
					return level + 1
				}
			}
		}
	}
	return level
}

func levelByBFS(u gomel.Unit, poset gomel.Poset) int {
	if len(u.Parents()) == 0 {
		return 0
	}

	parents := u.Parents()
	level := parents[len(parents)-1].Level()
	procSeen := make(map[int]bool)
	unitsSeen := make(map[gomel.Hash]bool)
	queue := []gomel.Unit{u}
	for len(queue) > 0 {
		w := queue[0]
		queue = queue[1:]

		for _, v := range w.Parents() {
			if v.Level() == level && !unitsSeen[*v.Hash()] {
				queue = append(queue, v)
				unitsSeen[*v.Hash()] = true
				procSeen[v.Creator()] = true
				if poset.IsQuorum(len(procSeen)) {
					return level + 1
				}
			}
		}
	}
	return level
}

// collectUnits runs dfs from maximal units in the given poset and returns a map
// creator => (height => slice of units by this creator on this height)
func collectUnits(p gomel.Poset) map[int]map[int][]gomel.Unit {
	seenUnits := make(map[gomel.Hash]bool)
	result := make(map[int]map[int][]gomel.Unit)
	for pid := 0; pid < p.NProc(); pid++ {
		result[pid] = make(map[int][]gomel.Unit)
	}

	var dfs func(u gomel.Unit)
	dfs = func(u gomel.Unit) {
		seenUnits[*u.Hash()] = true
		if _, ok := result[u.Creator()][u.Height()]; !ok {
			result[u.Creator()][u.Height()] = []gomel.Unit{}
		}
		result[u.Creator()][u.Height()] = append(result[u.Creator()][u.Height()], u)
		for _, uParent := range u.Parents() {
			if !seenUnits[*uParent.Hash()] {
				dfs(uParent)
			}
		}
	}
	p.MaximalUnitsPerProcess().Iterate(func(units []gomel.Unit) bool {
		for _, u := range units {
			if !seenUnits[*u.Hash()] {
				dfs(u)
			}
		}
		return true
	})
	return result
}

func BenchmarkLevelComputing(b *testing.B) {
	var (
		poset      gomel.Poset
		readingErr error
		pf         posetFactory
		units      map[int]map[int][]gomel.Unit
	)
	testfiles := []string{
		"random_10p_100u_2par.txt",
		"random_100p_5000u_10par.txt",
		"random_100p_5000u.txt",
	}
	for _, testfile := range testfiles {
		poset, readingErr = tests.CreatePosetFromTestFile("../testdata/"+testfile, pf)

		if readingErr != nil {
			panic(readingErr)
			return
		}
		units = collectUnits(poset)
		flatten := []gomel.Unit{}
		for pid := range units {
			for h := range units[pid] {
				flatten = append(flatten, units[pid][h]...)
			}
		}
		b.Run("With floors on "+testfile, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				for _, u := range flatten {
					u.(*unit).computeLevel()
				}
			}
		})
		b.Run("With iterating on "+testfile, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				for _, u := range flatten {
					levelByIteratingPrimes(u, poset)
				}
			}
		})
		b.Run("By simple dfs on "+testfile, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				for _, u := range flatten {
					levelByDFS(u, poset)
				}
			}
		})
		b.Run("By simple bfs on "+testfile, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				for _, u := range flatten {
					levelByBFS(u, poset)
				}
			}
		})
	}
}