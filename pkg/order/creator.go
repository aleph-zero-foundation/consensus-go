package order

import (
	"sync"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/unit"
	"gitlab.com/alephledger/core-go/pkg/core"
)

type creator struct {
	conf       config.Config
	ord        *orderer
	ds         core.DataSource
	unitBelt   chan gomel.Unit
	epoch      gomel.EpochID
	candidates []gomel.Unit
	maxLvl     int // max level of units in candidates
	onMaxLvl   int // number of candidates on maxLvl
	level      int // level of unit we could produce with current candidates
	quorum     int
	frozen     map[uint16]bool
	mx         sync.Mutex
	log        zerolog.Logger
}

func newCreator(conf config.Config, ord *orderer, ds core.DataSource, unitBelt chan gomel.Unit, log zerolog.Logger) *creator {
	return &creator{
		conf:       conf,
		ord:        ord,
		ds:         ds,
		unitBelt:   unitBelt,
		candidates: make([]gomel.Unit, conf.NProc),
		maxLvl:     -1,
		quorum:     int(gomel.MinimalQuorum(conf.NProc)),
		frozen:     make(map[uint16]bool),
		log:        log,
	}
}

func (cr *creator) work() {
	cr.ord.wg.Add(1)
	defer cr.ord.wg.Done()
	var parents []gomel.Unit
	var level int
	for u := range cr.unitBelt {
		cr.mx.Lock()
		cr.update(u)
		if cr.ready() {
			// Step 1: update candidates with all units waiting on the unit belt
			n := len(cr.unitBelt)
			for i := 0; i < n; i++ {
				cr.update(<-cr.unitBelt)
			}
			if cr.ready() {
				// we need to check that again, in case epoch changed in Step 1.
				// Step 2: pick parents and level depending on creating strategy
				if cr.conf.CanSkipLevel {
					level = cr.level
					parents = cr.getParents()
				} else {
					level = cr.candidates[cr.conf.Pid].Level() + 1
					parents = cr.getParentsForLevel(level)
				}
				// Step 3: create unit
				cr.createUnit(parents, level, cr.ds.GetData())
			}
		}
		cr.mx.Unlock()
	}
}

func (cr *creator) ready() bool {
	return cr.level > cr.candidates[cr.conf.Pid].Level()
}

// update takes a unit that has recently be added to the orderer and updates
// creator internal state with information contained in that unit
func (cr *creator) update(u gomel.Unit) {
	// if unit's creator is known to be a forker we simply ignore it
	if cr.frozen[u.Creator()] {
		return
	}

	// if the unit is from a new epoch, switch to that epoch
	// since units appear on the belt in order they were added to the dag
	// the first unit from new epoch is always a witness unit
	if u.EpochID() > cr.epoch {
		if !witness(u) {
			panic("creator received non-witness unit from new epoch")
		}
		cr.newEpoch(u.EpochID(), u.Data())
		return
	}

	cr.updateCandidates(u)
	cr.updateShares(u)
}

// resetCandidates resets the candidates and all related variables to the initial state
// (a slice with NProc nils). This is useful when switching to a new epoch.
func (cr *creator) resetCandidates() {
	cr.candidates = make([]gomel.Unit, cr.conf.NProc)
	cr.maxLvl = -1
	cr.onMaxLvl = 0
	cr.level = 0
}

// updateCandidates puts the provided unit in parent candidates provided that
// the level is higher than the level of the previous candidate for that creator
func (cr *creator) updateCandidates(u gomel.Unit) {
	prev := cr.candidates[u.Creator()]
	if prev == nil || prev.Level() < u.Level() {
		cr.candidates[u.Creator()] = u
		if u.Level() == cr.maxLvl {
			cr.onMaxLvl++
		}
		if u.Level() > cr.maxLvl {
			cr.maxLvl = u.Level()
			cr.onMaxLvl = 1
		}
		cr.level = cr.maxLvl
		if cr.onMaxLvl >= cr.quorum {
			cr.level++
		}
	}

}

// updateShares extracts threshold signature shares from finishing units.
func (cr *creator) updateShares(u gomel.Unit) {
	//TODO
}

// freezeParent tells the creator to stop updating parent candidates for the given pid
// and use the corresponding parent of our last created unit instead. Returns that parent.
func (cr *creator) freezeParent(pid uint16) gomel.Unit {
	cr.mx.Lock()
	defer cr.mx.Unlock()
	u := cr.candidates[cr.conf.Pid].Parents()[pid]
	cr.candidates[pid] = u
	cr.frozen[pid] = true
	return u
}

// getParents returns a copy of current parent candidates.
func (cr *creator) getParents() []gomel.Unit {
	result := make([]gomel.Unit, cr.conf.NProc)
	copy(result, cr.candidates)
	return makeConsistent(result)
}

// getParentsForLevel returns a set of candidates such that their level is at most level-1.
func (cr *creator) getParentsForLevel(level int) []gomel.Unit {
	result := make([]gomel.Unit, cr.conf.NProc)
	for i, u := range cr.candidates {
		for u.Level() >= level {
			u = gomel.Predecessor(u)
		}
		result[i] = u
	}
	return makeConsistent(result)
}

// createUnit creates a unit with the given parents, level, and data. Assumes provided parameters
// are consistent, that means level == gomel.LevelFromParents(parents) and cr.epoch == parents[i].EpochID()
// Inserts the new unit into orderer and updates local info about candidates.
func (cr *creator) createUnit(parents []gomel.Unit, level int, data core.Data) {
	rsData := cr.ord.rsData(level, parents, cr.epoch)
	u := unit.New(cr.conf.Pid, cr.epoch, parents, level, data, rsData, cr.conf.PrivateKey)
	cr.ord.insert(u)
	cr.updateCandidates(u)
	cr.last = u
}

// newEpoch creates a dealing unit for the chosen epoch with the provided data.
func (cr *creator) newEpoch(epoch gomel.EpochID, data core.Data) {
	cr.epoch = epoch
	cr.createUnit(make([]gomel.Unit, cr.conf.NProc), 0, data)
}

func makeConsistent(parents []gomel.Unit) []gomel.Unit {
	for i := 0; i < len(parents); i++ {
		for j := 0; j < len(parents); j++ {
			if parents[j] == nil {
				continue
			}
			u := parents[j].Parents()[i]
			if parents[i] == nil || (u != nil && u.Level() > parents[i].Level()) {
				parents[i] = u
			}
		}
	}
	return parents
}
