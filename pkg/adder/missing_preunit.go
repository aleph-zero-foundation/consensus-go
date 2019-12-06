package adder

import (
	"time"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type missingPreunit struct {
	neededBy  []*waitingPreunit // list of waitingPreunits that has this preunit as parent
	requested time.Time
}

// newMissing constructs a new missingPreunit that is needed by some waitingPreunit.
func newMissing() *missingPreunit {
	mp := &missingPreunit{
		neededBy: make([]*waitingPreunit, 0, 8),
	}
	return mp
}

// addNeeding adds another waitingPreunit that needs this missingPreunit.
func (mp *missingPreunit) addNeeding(wp *waitingPreunit) {
	mp.neededBy = append(mp.neededBy, wp)
}

// registerMissing registers the fact that the given waitingPreunit needs an unknown unit with the given id.
func (ad *adder) registerMissing(id uint64, wp *waitingPreunit) {
	if _, ok := ad.missing[id]; !ok {
		ad.missing[id] = newMissing()
	}
	ad.missing[id].addNeeding(wp)
}

// fetchMissing is called on a freshly created waitingPreunit that has some missing parents.
// Sends a signal to trigger fetch or gossip.
func (ad *adder) fetchMissing(wp *waitingPreunit, maxHeights []int) {
	if ad.requestFetch == nil {
		if ad.requestGossip != nil {
			ad.requestGossip(wp.source)
		}
		return
	}
	nProc := ad.dag.NProc()
	toRequest := make([]uint64, 0, 8)
	var mp *missingPreunit
	now := time.Now()
	for creator, height := range wp.pu.View().Heights {
		for h := height; h > maxHeights[creator]; h-- {
			id := gomel.ID(h, uint16(creator), nProc)
			if _, waiting := ad.waitingByID[id]; !waiting {
				if _, ok := ad.missing[id]; !ok {
					mp = newMissing()
					ad.missing[id] = mp
				} else {
					mp = ad.missing[id]
				}
				if now.Sub(mp.requested) > fetchInterval {
					toRequest = append(toRequest, id)
					mp.requested = now
				}
			}
		}
		if ad.requestGossip != nil && len(toRequest) > gossipAbove {
			ad.requestGossip(wp.source)
			return
		}
	}
	if len(toRequest) > 0 {
		ad.requestFetch(wp.source, toRequest)
	}
}
