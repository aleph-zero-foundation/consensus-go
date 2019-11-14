package adder

type missingPreunit struct {
	neededBy []*waitingPreunit // list of waitingPreunits that has this preunit as parent
}

// newMissing constructs a new missingPreunit that is needed by some waitingPreunit.
func newMissing(wp *waitingPreunit) *missingPreunit {
	mp := &missingPreunit{
		neededBy: make([]*waitingPreunit, 1, 4),
	}
	mp.neededBy[0] = wp
	return mp
}

// addNeeded adds another waitingPreunit that needs this missingPreunit.
func (mp *missingPreunit) addNeeded(wp *waitingPreunit) {
	mp.neededBy = append(mp.neededBy, wp)
}

// registerMissing registers the fact that the given waitingPreunit needs an unknown unit with the given id.
func (ad *adder) registerMissing(id uint64, wp *waitingPreunit) {
	if mp, ok := ad.missing[id]; ok {
		mp.addNeeded(wp)
		return
	}
	ad.missing[id] = newMissing(wp)
}

// resolveMissing
func (ad *adder) resolveMissing(wp *waitingPreunit) {}
