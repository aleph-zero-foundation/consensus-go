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

type server struct {
	dag      gomel.Dag
	rs       gomel.RandomSource
	interval time.Duration
	inner    gsync.QueryServer
	backlog  *backlog
	deps     *dependencies
	quit     int32
	wg       sync.WaitGroup
	log      zerolog.Logger
}

// NewRetrying wraps the given fallback with a retrying routine that keeps trying to add problematic units.
func NewRetrying(dag gomel.Dag, rs gomel.RandomSource, interval time.Duration, log zerolog.Logger) gsync.QueryServer {
	return &server{
		dag:     dag,
		rs:      rs,
		backlog: newBacklog(),
		deps:    newDeps(),
		log:     log,
	}
}

func (f *server) FindOut(preunit gomel.Preunit) {
	if f.addToBacklog(preunit) {
		f.log.Info().Str(logging.Hash, gomel.Nickname(preunit)).Msg(logging.AddedToBacklog)
		f.inner.FindOut(preunit)
	}
}

// Start runs a goroutine that attempts to add units from the backlog in set intervals.
func (f *server) Start() {
	f.wg.Add(1)
	go f.work()
}

// Stop signals the adding goroutine to halt and blocks until it does.
func (f *server) StopIn() {
	atomic.StoreInt32(&f.quit, 1)
	f.wg.Wait()
}

func (f *server) StopOut() {}

func (f *server) addToBacklog(pu gomel.Preunit) bool {
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
		go f.addUnit(pu)
		return false
	}
	// The code below has the invariant that if a unit is in dependencies, then it is also in the backlog.
	if !f.backlog.add(pu) {
		return false
	}
	f.deps.add(pu.Hash(), missing)
	return true
}

func (f *server) work() {
	defer f.wg.Done()
	for atomic.LoadInt32(&f.quit) != 1 {
		time.Sleep(f.interval)
		f.update()
		f.backlog.refallback(f.inner.FindOut)
	}
}

func (f *server) update() {
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

func (f *server) addUnit(pu gomel.Preunit) {
	err := add.Unit(f.dag, f.rs, pu, f.log)
	if err != nil {
		log.Error().Str("where", "retryingFallback.addUnit").Msg(err.Error())
	}
}
