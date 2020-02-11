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
	current  *epoch
	previous *epoch
	conf     config.Config
	syncer   gomel.Syncer
	rsf      gomel.RandomSourceFactory
	alert    gomel.Alerter
	ps       core.PreblockSink
	unitBelt chan gomel.Unit
	output   chan []gomel.Unit
	mx       sync.RWMutex
	log      zerolog.Logger
}

// NewOrderer TODO
func NewOrderer(conf config.Config, syncer gomel.Syncer, ps core.PreblockSink) gomel.Orderer {
	ord := &orderer{
		conf:     conf,
		syncer:   syncer,
		ps:       ps,
		unitBelt: make(chan gomel.Unit, beltSize),
	}
	return ord
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
		ep.addPreunits(source, preunits[:end]...)
		preunits = preunits[end:]
	}

}

func (ord *orderer) UnitsByID(ids ...uint64) []gomel.Unit {
	//TODO
	return nil
}

func (ord *orderer) UnitsByHash(hashes ...*gomel.Hash) []gomel.Unit {
	ord.mx.RLock()
	cur := ord.current.dag.GetUnits(hashes)
	prev := ord.previous.dag.GetUnits(hashes)
	ord.mx.RUnlock()
	for i := range cur {
		if cur[i] == nil {
			cur[i] = prev[i]
		}
	}
	return cur
}

func (ord *orderer) MaxUnits(epoch gomel.EpochID) gomel.SlottedUnits {
	ep := ord.getEpoch(epoch)
	if ep != nil {
		return ep.dag.MaximalUnitsPerProcess()
	}
	return nil
}

func (ord *orderer) GetInfo() *gomel.DagInfo {
	ord.mx.RLock()
	defer ord.mx.RUnlock()
	return gomel.MaxView(ord.current.dag)
}

func (ord *orderer) Delta(info *gomel.DagInfo) []gomel.Unit {
	//TODO
	return nil
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
		ord.current = newEpoch(epoch, ord.conf, ord.syncer, ord.rsf, ord.alert, ord.unitBelt, ord.output, ord.log)
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
