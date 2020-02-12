package order

import (
	"sync"

	"github.com/rs/zerolog"

	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/core-go/pkg/core"
)

const (
	beltSize = 1000
)

type orderer struct {
	conf         config.Config
	syncer       gomel.Syncer
	rsf          gomel.RandomSourceFactory
	alerter      gomel.Alerter
	ps           core.PreblockSink
	current      *epoch
	previous     *epoch
	unitBelt     chan gomel.Unit
	orderedUnits chan []gomel.Unit
	mx           sync.RWMutex
	wg           sync.WaitGroup
	log          zerolog.Logger
}

// NewOrderer TODO
func NewOrderer(conf config.Config, rsf gomel.RandomSourceFactory, ps core.PreblockSink) gomel.Orderer {
	ord := &orderer{
		conf:         conf,
		rsf:          rsf,
		ps:           ps,
		unitBelt:     make(chan gomel.Unit, beltSize),
		orderedUnits: make(chan []gomel.Unit, 10),
	}
	return ord
}

func (ord *orderer) SetAlerter(alerter gomel.Alerter) {
	ord.alerter = alerter
}

func (ord *orderer) SetSyncer(syncer gomel.Syncer) {
	ord.syncer = syncer
}

func (ord *orderer) Start() error {
	ord.wg.Add(1)
	go ord.preblockMaker()
	return nil
}

func (ord *orderer) Stop() {
	close(ord.orderedUnits)
	ord.wg.Wait()
}

func (ord *orderer) preblockMaker() {
	defer ord.wg.Done()
	current := gomel.EpochID(0)
	for round := range ord.orderedUnits {
		epoch := round[0].EpochID()
		if epoch >= current {
			ord.ps <- gomel.ToPreblock(round)
		}
		current = epoch
	}
}

// AddPreunits sends preunits received from other committee members to their corresponding epochs.
// It assumes preunits are ordered by ascending epochID and, within each epoch, they are topologically sorted.
func (ord *orderer) AddPreunits(source uint16, preunits ...gomel.Preunit) {
	for len(preunits) > 0 {
		epoch := preunits[0].EpochID()
		end := 0
		for end < len(preunits) && preunits[end].EpochID() == epoch {
			end++
		}
		ep := ord.getEpoch(epoch)
		if ep == nil {

		}
		ep.adder.AddPreunits(source, preunits[:end]...) //TODO handle error
		preunits = preunits[end:]
	}

}

// UnitsByID allows to access units present in the orderer using their ids.
// The returned slice contains only existing units (no nil entries for non-present units)
// and can contain multiple units with the same id (forks). Because of that the length
// of the result can be different than the number of arguments.
func (ord *orderer) UnitsByID(ids ...uint64) []gomel.Unit {
	var result []gomel.Unit
	ord.mx.RLock()
	defer ord.mx.RUnlock()
	for _, id := range ids {
		_, _, epoch := gomel.DecodeID(id)
		if epoch == ord.current.id {
			result = append(result, ord.current.dag.GetByID(id)...)
			continue
		}
		if epoch == ord.previous.id {
			result = append(result, ord.previous.dag.GetByID(id)...)
		}
	}
	return result
}

// UnitsByHash allows to access units present in the orderer using their hashes.
// The length of the returned slice is equal to the number of argument hashes.
// For non-present units the returned slice contains nil on the corresponding position.
func (ord *orderer) UnitsByHash(hashes ...*gomel.Hash) []gomel.Unit {
	ord.mx.RLock()
	defer ord.mx.RUnlock()
	result := ord.current.dag.GetUnits(hashes)
	for i := range result {
		if result[i] == nil {
			result[i] = ord.previous.dag.GetUnit(hashes[i])
		}
	}
	return result
}

// MaxUnits returns maximal units per process from the chosen epoch.
// TODO this is used only by Alerts, maybe a different signature would be more convenient?
func (ord *orderer) MaxUnits(epoch gomel.EpochID) gomel.SlottedUnits {
	ep := ord.getEpoch(epoch)
	if ep != nil {
		return ep.dag.MaximalUnitsPerProcess()
	}
	return nil
}

// GetInfo returns DagInfo of the dag from the most recent epoch.
// TODO this could potentially be counterproductive, as we gossip only about our most recent epoch.
// That means just after switching to a new epoch due to "external proof", we immediately abandon the
// previous epoch, even though we might still benefit from one last gossip to help us produce last timing units.
// A potential solution would be to access the last-produced-preblock-epoch variable kept by preblockMaker() and
// gossip also about "previous" if we haven't produced any preblock from "current".
func (ord *orderer) GetInfo() *gomel.DagInfo {
	ord.mx.RLock()
	defer ord.mx.RUnlock()
	return gomel.MaxView(ord.current.dag)
}

// Delta returns all units present in the orderer that are newer than units
// described by the given DagInfo. This includes all units from the epoch given
// by the DagInfo above provided heights as well as ALL units from newer epochs.
func (ord *orderer) Delta(info *gomel.DagInfo) []gomel.Unit {
	ord.mx.RLock()
	defer ord.mx.RUnlock()
	if info.Epoch > ord.current.id {
		return nil
	}
	if info.Epoch == ord.current.id {
		return ord.current.unitsAbove(info.Heights)
	}
	if info.Epoch == ord.previous.id {
		return append(ord.previous.unitsAbove(info.Heights), ord.current.allUnits()...)
	}
	return ord.current.allUnits()
}

func (ord *orderer) getEpoch(epoch gomel.EpochID) *epoch {
	ord.mx.RLock()
	defer ord.mx.RUnlock()
	if epoch == ord.current.id {
		return ord.current
	}
	if epoch == ord.previous.id {
		return ord.previous
	}
	return nil
}

func (ord *orderer) newEpoch(epoch gomel.EpochID) *epoch {
	ord.mx.Lock()
	defer ord.mx.Unlock()
	if epoch > ord.current.id {
		ord.previous.close()
		ord.previous = ord.current
		ord.current = newEpoch(epoch, ord.conf, ord.syncer, ord.rsf, ord.alert, ord.unitBelt, ord.orderedUnits, ord.log)
		return ord.current
	}
	if epoch == ord.current.id {
		return ord.current
	}
	if epoch == ord.previous.id {
		return ord.previous
	}
	return nil
}

func (ord *orderer) insert(unit gomel.Unit) {
	ep := ord.getEpoch(unit.EpochID())
	if ep == nil {
		ep = ord.newEpoch(unit.EpochID())
	}
	ep.dag.Insert(unit)
}

// witness checks if the given preunit is a proof that a new epoch started.
func witness(pu gomel.Preunit) bool {
	//if !gomel.Dealing(pu) {
	return false
	//}
	// check threshold signature
	return true
}
