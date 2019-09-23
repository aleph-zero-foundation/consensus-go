// Package retrying implements several algorithms for acquiring unknown parents of units received in syncs.
package retrying

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/process"
	gsync "gitlab.com/alephledger/consensus-go/pkg/sync"
	"gitlab.com/alephledger/consensus-go/pkg/sync/add"
)

type server struct {
	dag      gomel.Dag
	adder    gomel.Adder
	interval time.Duration
	inner    gsync.Fallback
	backlog  *backlog
	deps     *dependencies
	quit     int32
	wg       sync.WaitGroup
	log      zerolog.Logger
}

// NewService creates a service that continuously tries to add problematic units using provided Fallback.
func NewService(dag gomel.Dag, adder gomel.Adder, fallback gsync.Fallback, interval time.Duration, log zerolog.Logger) (process.Service, gsync.Fallback) {
	s := &server{
		dag:     dag,
		adder:   adder,
		inner:   fallback,
		backlog: newBacklog(),
		deps:    newDeps(),
		log:     log,
	}
	return s, s
}

func (f *server) FindOut(preunit gomel.Preunit) {
	if f.addToBacklog(preunit) {
		f.log.Info().Str(logging.Hash, gomel.Nickname(preunit)).Msg(logging.AddedToBacklog)
		f.inner.FindOut(preunit)
	}
}

func (f *server) Start() error {
	f.wg.Add(1)
	go f.work()
	return nil
}

func (f *server) Stop() {
	atomic.StoreInt32(&f.quit, 1)
	f.wg.Wait()
}

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
		add.Unit(f.adder, pu, f.inner, "retrying.addToBacklog", f.log)
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
			add.Unit(f.adder, pu, f.inner, "retrying.update", f.log)
			f.backlog.del(h)
			f.log.Info().Str(logging.Hash, gomel.Nickname(pu)).Msg(logging.RemovedFromBacklog)
		}
		presentHashes = addableHashes
	}
}
