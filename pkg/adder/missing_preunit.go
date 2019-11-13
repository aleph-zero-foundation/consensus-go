package adder

type missingPreunit struct {
	neededBy []*waitingPreunit // list of waitingPreunits that has this preunit as parent
}

// registerMissing registers the fact that the given waitingPreunit needs an unknown unit with the given id.
func (ad *adder) registerMissing(id uint64, wp *waitingPreunit) {
	if _, ok := ad.missing[id]; !ok {
		ad.missing[id] = &missingPreunit{
			neededBy: make([]*waitingPreunit, 0, 8),
		}
	}
	nb := ad.missing[id].neededBy
	nb = append(nb, wp)
}

// resolveMissing
func (ad *adder) resolveMissing(wp *waitingPreunit) {}
