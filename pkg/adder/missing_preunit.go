package adder

import (
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/sync/fetch"
)

// gossipAbove is the number of missing units above which gossip is triggered instead of fetch.
const gossipAbove = 50

type missingPreunit struct {
	neededBy []*waitingPreunit // list of waitingPreunits that has this preunit as parent
}

// newMissing constructs a new missingPreunit that is needed by some waitingPreunit.
func newMissing(wp *waitingPreunit) *missingPreunit {
	mp := &missingPreunit{
		neededBy: make([]*waitingPreunit, 1, 8),
	}
	mp.neededBy[0] = wp
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
func (ad *adder) fetchMissing(wp *waitingPreunit, maxHeights []int) {
	if ad.fetchRequests == nil {
		if ad.gossipRequests != nil {
			ad.gossipRequests <- wp.source
		}
		return
	}
	nProc := ad.dag.NProc()
	missing := make([]uint64, 0, 8)
	for creator, height := range wp.pu.View().Heights {
		for h := height; h > maxHeights[creator]; h-- {
			id := gomel.ID(h, uint16(creator), nProc)
			if _, ok := ad.waitingByID[id]; !ok {
				missing = append(missing, id)
			}
		}
		if ad.gossipRequests != nil && len(missing) > gossipAbove {
			ad.gossipRequests <- wp.source
			return
		}
	}
	ad.fetchRequests <- fetch.Request{wp.source, missing}
}
