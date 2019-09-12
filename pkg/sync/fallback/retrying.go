// Package fallback implements several algorithms for acquiring unknown parents of units received in syncs.
package fallback

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	gsync "gitlab.com/alephledger/consensus-go/pkg/sync"
	"gitlab.com/alephledger/consensus-go/pkg/sync/add"
)

// Retrying is a wrapper for a fallback that continuously tries adding the problematic preunits to the dag.
type Retrying struct {
	dag      gomel.Dag
	rs       gomel.RandomSource
	inner    gsync.Fallback
	interval time.Duration
	backlog  *backlog
	deps     *dependencies
	quit     int32
	wg       sync.WaitGroup
	log      zerolog.Logger
}

// NewRetrying wraps the given fallback with a retrying routine that keeps trying to add problematic units.
func NewRetrying(inner gsync.Fallback, dag gomel.Dag, rs gomel.RandomSource, interval time.Duration, log zerolog.Logger) *Retrying {
	return &Retrying{
		dag:     dag,
		rs:      rs,
		inner:   inner,
		backlog: newBacklog(),
		deps:    newDeps(),
		log:     log,
	}
}

// Run executes the fallback and memorizes the preunit for later retries.
func (f *Retrying) Run(pu gomel.Preunit) {
	if f.addToBacklog(pu) {
		f.log.Info().Str(logging.Hash, gomel.Nickname(pu)).Msg(logging.AddedToBacklog)
		f.inner.Run(pu)
	}
}

// Start runs a goroutine that attempts to add units from the backlog in set intervals.
func (f *Retrying) Start() error {
	f.wg.Add(1)
	go f.work()
	return nil
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
	// The code below has the invariant that if a unit is in dependencies, then it is also in the backlog.
	if !f.backlog.add(pu) {
		return false
	}
	f.deps.add(pu.Hash(), missing)
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
	presentHashes := f.deps.scan(f.dag)
	for len(presentHashes) != 0 {
		addableHashes := f.deps.satisfy(presentHashes)
		for _, h := range addableHashes {
			// There is no need for nil checks, because of the invariant mentioned above.
			pu := f.backlog.get(h)
			f.addUnit(pu)
			f.backlog.del(h)
			f.log.Info().Str(logging.Hash, gomel.Nickname(pu)).Msg(logging.RemovedFromBacklog)
		}
		presentHashes = addableHashes
	}
}

func (f *Retrying) addUnit(pu gomel.Preunit) {
	err := add.Unit(f.dag, f.rs, pu, gomel.NopCallback, gsync.NopFallback(), f.log)
	if err != nil {
		log.Error().Str("where", "retryingFallback.addUnit").Msg(err.Error())
	}
}
