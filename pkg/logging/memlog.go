package logging

import (
	"runtime"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type service struct {
	ticker   <-chan time.Time
	exitChan chan struct{}
	log      zerolog.Logger
	wg       sync.WaitGroup
}

// NewService constructs a new service that logs current total memory consumption every n seconds.
func NewService(n int, log zerolog.Logger) gomel.Service {
	var ticker <-chan time.Time
	if n == 0 {
		ticker = make(<-chan time.Time)
	} else {
		ticker = time.Tick(time.Duration(n) * time.Second)
	}
	return &service{
		ticker:   ticker,
		exitChan: make(chan struct{}),
		log:      log,
	}
}

func (s *service) Start() error {
	s.wg.Add(1)
	var stats runtime.MemStats
	go func() {
		for {
			select {
			case <-s.exitChan:
				s.wg.Done()
				return
			case <-s.ticker:
				runtime.ReadMemStats(&stats)
				s.log.Info().Uint64(Memory, stats.Sys).Uint64(Size, stats.HeapAlloc).Msg(MemoryUsage)
			}
		}
	}()
	s.log.Info().Msg(ServiceStarted)
	return nil
}

func (s *service) Stop() {
	close(s.exitChan)
	s.wg.Wait()
	s.log.Info().Msg(ServiceStopped)
}
