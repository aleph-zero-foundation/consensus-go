package adder

type missingPreunit struct {
	neededBy []*waitingPreunit // list of waitingPreunits that has this preunit as parent
}

// newMissing constructs a new missingPreunit that is needed by some waitingPreunit.
func newMissing(wp *waitingPreunit) *missingPreunit {
	mp := &missingPreunit{
		neededBy: []*waitingPreunit{wp},
	}
	return mp
}

// addNeeding adds another waitingPreunit that needs this missingPreunit.
func (mp *missingPreunit) addNeeding(wp *waitingPreunit) {
	mp.neededBy = append(mp.neededBy, wp)
}

// registerMissing registers the fact that the given waitingPreunit needs an unknown unit with the given id.
func (ad *adder) registerMissing(id uint64, wp *waitingPreunit) {
	if mp, ok := ad.missing[id]; ok {
		mp.addNeeding(wp)
		return
	}
	ad.missing[id] = newMissing(wp)
}

// fetchMissing is called on a freshly created waitingPreunit that has some missing parents.
// Sends a signal to trigger fetch or gossip.
func (ad *adder) fetchMissing(wp *waitingPreunit) {}
