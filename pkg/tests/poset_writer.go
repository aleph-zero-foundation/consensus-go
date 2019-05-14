package tests

import (
	"fmt"
	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"io"
)

// PosetWriter is meant to write posets
type PosetWriter struct{}

// NewPosetWriter returns instation of PosetWriter
func NewPosetWriter() PosetWriter {
	return PosetWriter{}
}

// WritePoset writes the given poset in the following format
// 1st line contains integer N - the number of processes
// Then there is one line per unit in the following format
// C-H-V [Parents]
// Where
// (1) C is the Creator of a unit,
// (2) H is the Height of a unit,
// (3) V is the Version of a unit (0 for non-forked units, forks created by the same process on the same height are enumerated with consecutive integers)
// (4) Parents is the list of units separated by a single space encoded in the same C-H-V format
func (PosetWriter) WritePoset(writer io.Writer, p gomel.Poset) error {
	fmt.Fprintf(writer, "%d\n", p.NProc())

	seenUnits := make(map[gomel.Hash]bool)
	unitToVersion := make(map[gomel.Hash]int)
	unitCreatorHeightToNForks := make(map[[2]int]int)

	var dfs func(u gomel.Unit)
	dfs = func(u gomel.Unit) {
		if _, exists := seenUnits[*u.Hash()]; !exists {
			seenUnits[*u.Hash()] = true
			if _, exists := unitToVersion[*u.Hash()]; !exists {
				unitToVersion[*u.Hash()] = unitCreatorHeightToNForks[[2]int{u.Creator(), u.Height()}]
				unitCreatorHeightToNForks[[2]int{u.Creator(), u.Height()}]++
			}
			for _, v := range u.Parents() {
				dfs(v)
			}
			fmt.Fprintf(writer, "%d-%d-%d", u.Creator(), u.Height(), unitToVersion[*u.Hash()])
			for _, v := range u.Parents() {
				fmt.Fprintf(writer, " %d-%d-%d", v.Creator(), v.Height(), unitToVersion[*v.Hash()])
			}
			fmt.Fprintf(writer, "\n")
		}
	}
	p.MaximalUnitsPerProcess().Iterate(func(units []gomel.Unit) bool {
		for _, v := range units {
			dfs(v)
		}
		return true
	})
	return nil
}
