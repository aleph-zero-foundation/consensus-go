package gomel

// DagInfo contains information about heights of the most recent units in a Dag.
type DagInfo struct {
	Epoch   int
	Heights []int
}

// MaxView returns the current DagInfo for the given Dag.
func MaxView(dag Dag) *DagInfo {
	maxes := dag.MaximalUnitsPerProcess()
	heights := make([]int, 0, dag.NProc())
	maxes.Iterate(func(units []Unit) bool {
		h := -1
		for _, u := range units {
			if u.Height() > h {
				h = u.Height()
			}
		}
		heights = append(heights, h)
		return true
	})
	return &DagInfo{
		Epoch:   0, // SHALL BE DONE!
		Heights: heights,
	}
}
