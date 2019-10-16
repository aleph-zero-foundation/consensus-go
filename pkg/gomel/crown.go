package gomel

// Crown represents nProc units created by different processes in a condensed form.
// It contains heights of the units and a combined hash of the units - the ControlHash.
// Any missing unit is represented by height -1, and ZeroHash.
type Crown struct {
	Heights     []int
	ControlHash Hash
}

// EmptyCrown is a crown with all the units missing.
func EmptyCrown(nProc uint16) *Crown {
	heights := make([]int, nProc)
	for i := range heights {
		heights[i] = -1
	}
	return &Crown{
		Heights:     heights,
		ControlHash: *CombineHashes(make([]*Hash, nProc)),
	}
}

// NewCrown returns a crown with given slice of heights, and given control hash.
func NewCrown(heights []int, hash *Hash) *Crown {
	return &Crown{
		Heights:     heights,
		ControlHash: *hash,
	}
}

// CrownFromParents returns a crown consisting of the given slice of units.
// It assumes that the given slice of parents is of the length nProc, and
// the i-th unit is created by the i-th process.
func CrownFromParents(parents []Unit) *Crown {
	nProc := len(parents)
	heights := make([]int, nProc)
	hashes := make([]*Hash, nProc)
	for i, u := range parents {
		if u == nil {
			heights[i] = -1
			hashes[i] = &ZeroHash
		} else {
			heights[i] = u.Height()
			hashes[i] = u.Hash()
		}
	}
	return &Crown{
		Heights:     heights,
		ControlHash: *CombineHashes(hashes),
	}
}
