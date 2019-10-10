// Package retrying implements several algorithms for acquiring unknown parents of units received in syncs.
package retrying

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	gsync "gitlab.com/alephledger/consensus-go/pkg/sync"
	"gitlab.com/alephledger/consensus-go/pkg/sync/add"
)

type server struct {
	dag       gomel.Dag
	adder     gomel.Adder
	interval  time.Duration
	inner     gsync.Fallback
	fetchData func(*gomel.Hash, uint16) error
	backlog   *backlog
	quit      int32
	wg        sync.WaitGroup
	log       zerolog.Logger
}

// NewService creates a service that continuously tries to add problematic units using provided Fallback.
func NewService(dag gomel.Dag, adder gomel.Adder, fallback gsync.Fallback, fetchData func(*gomel.Hash, uint16) error, interval time.Duration, log zerolog.Logger) (gomel.Service, gsync.Fallback) {
	s := &server{
		dag:       dag,
		adder:     adder,
		inner:     fallback,
		fetchData: fetchData,
		backlog:   newBacklog(),
		log:       log,
	}
	return s, s
}

func (f *server) Resolve(preunit gomel.Preunit) {
	if f.addToBacklog(preunit) {
		f.log.Info().Str(logging.Hash, gomel.Nickname(preunit)).Msg(logging.AddedToBacklog)
		f.inner.Resolve(preunit)
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
	if haveParents(pu, f.dag) {
		// we got the parents in the meantime, all is fine
		add.Unit(f.adder, pu, f.inner, f.fetchData, pu.Creator(), "retrying.addToBacklog", f.log)
		return false
	}
	return f.backlog.add(pu)
}

func (f *server) work() {
	defer f.wg.Done()
	for atomic.LoadInt32(&f.quit) != 1 {
		time.Sleep(f.interval)
		f.update()
		f.backlog.refallback(f.inner.Resolve)
	}
}

func (f *server) update() {
	toDelete := []*gomel.Hash{}
	f.backlog.refallback(func(pu gomel.Preunit) {
		if haveParents(pu, f.dag) {
			if add.Unit(f.adder, pu, f.inner, f.fetchData, pu.Creator(), "retrying.update", f.log) {
				toDelete = append(toDelete, pu.Hash())
			}
		}
	})
	for _, h := range toDelete {
		f.backlog.del(h)
	}
}

func haveParents(pu gomel.Preunit, dag gomel.Dag) bool {
	for i, h := range pu.View().Heights {
		if h == -1 {
			continue
		}

		if dh := dag.UnitsOnHeight(h); dh == nil || dh.Get(uint16(i)) == nil {
			return false
		}
	}
	return true
}
