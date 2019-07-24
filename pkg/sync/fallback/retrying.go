package fallback

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	gsync "gitlab.com/alephledger/consensus-go/pkg/sync"
	"gitlab.com/alephledger/consensus-go/pkg/sync/add"
)

// Retrying is a wrapper for a fallback that continously tries adding the problematic preunits to the dag.
type Retrying struct {
	dag       gomel.Dag
	rs        gomel.RandomSource
	inner     gsync.Fallback
	interval  time.Duration
	mx        sync.Mutex
	backlog   map[gomel.Hash]gomel.Preunit
	required  map[gomel.Hash]int
	neededFor map[gomel.Hash][]*gomel.Hash
	missing   []*gomel.Hash
	quit      int32
	wg        sync.WaitGroup
	log       zerolog.Logger
}

// NewRetrying wraps the given fallback with a retrying routine that keeps trying to add problematic units.
func NewRetrying(inner gsync.Fallback, dag gomel.Dag, rs gomel.RandomSource, interval time.Duration, log zerolog.Logger) *Retrying {
	return &Retrying{
		dag:       dag,
		rs:        rs,
		inner:     inner,
		mx:        sync.Mutex{},
		backlog:   make(map[gomel.Hash]gomel.Preunit),
		required:  make(map[gomel.Hash]int),
		neededFor: make(map[gomel.Hash][]*gomel.Hash),
		log:       log,
	}
}

// Run executes the fallback and memorizes the preunit for later retries.
func (f *Retrying) Run(pu gomel.Preunit) {
	if f.addToBacklog(pu) {
		f.inner.Run(pu)
	}
}

// Start runs a goroutine that attempts to add units from the backlog in set intervals.
func (f *Retrying) Start() {
	f.wg.Add(1)
	go f.work()
}

// Stop signals the adding goroutine to halt and blocks until it does.
func (f *Retrying) Stop() {
	atomic.StoreInt32(&f.quit, 1)
	f.wg.Wait()
}

func (f *Retrying) addToBacklog(pu gomel.Preunit) bool {
	hashes := pu.Parents()
	parents := f.dag.Get(hashes)
	missing := []*gomel.Hash{}
	for i, h := range hashes {
		if parents[i] == nil {
			missing = append(missing, h)
		}
	}
	if len(missing) == 0 {
		// we got the parents in the meantime, all is fine
		f.addUnit(pu)
		return false
	}
	ourHash := *pu.Hash()
	f.mx.Lock()
	defer f.mx.Unlock()
	if _, ok := f.backlog[ourHash]; ok {
		// unit already in backlog
		return false
	}
	f.backlog[ourHash] = pu
	f.required[ourHash] = len(missing)
	for _, h := range missing {
		neededFor := f.neededFor[*h]
		if len(neededFor) == 0 {
			// this is the first time we need this hash
			f.missing = append(f.missing, h)
		}
		f.neededFor[*h] = append(neededFor, &ourHash)
	}
	return true
}

func (f *Retrying) work() {
	defer f.wg.Done()
	for atomic.LoadInt32(&f.quit) != 1 {
		time.Sleep(f.interval)
		f.update()
	}
}

func (f *Retrying) update() {
	f.mx.Lock()
	defer f.mx.Unlock()
	units := f.dag.Get(f.missing)
	newMissing := make([]*gomel.Hash, 0, len(f.missing))
	for i, h := range f.missing {
		if units[i] != nil {
			f.gotHash(h)
		} else {
			newMissing = append(newMissing, h)
		}
	}
	f.missing = newMissing
}

func (f *Retrying) gotHash(h *gomel.Hash) {
	toUpdate := f.neededFor[*h]
	hashesAdded := []*gomel.Hash{}
	for _, hh := range toUpdate {
		f.required[*hh]--
		if f.required[*hh] == 0 {
			f.addUnit(f.backlog[*hh])
			delete(f.required, *hh)
			delete(f.backlog, *hh)
			hashesAdded = append(hashesAdded, hh)
		}
	}
	delete(f.neededFor, *h)
	for _, hh := range hashesAdded {
		f.gotHash(hh)
	}
}

func (f *Retrying) addUnit(pu gomel.Preunit) {
	err := add.Unit(f.dag, f.rs, pu, gomel.NopCallback(), gsync.NopFallback(), f.log)
	if err != nil {
		log.Error().Str("where", "retryingFallback.addUnit").Msg(err.Error())
	}
}
