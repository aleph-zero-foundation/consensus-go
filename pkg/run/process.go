// Package run defines API for running the whole consensus protocol.
package run

import (
	"errors"

	"github.com/rs/zerolog"

	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/forking"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/orderer"
	"gitlab.com/alephledger/consensus-go/pkg/random/beacon"
	"gitlab.com/alephledger/consensus-go/pkg/random/coin"
	"gitlab.com/alephledger/consensus-go/pkg/sync/syncer"
	"gitlab.com/alephledger/core-go/pkg/core"
	"gitlab.com/alephledger/core-go/pkg/crypto/tss"
	"gitlab.com/alephledger/core-go/pkg/network/tcp"
)

// Process is the main external API of consensus-go.
// Given two Config objects (one for the setup phase and one for the main consensus), data source and preblock sink,
// Process initializes two orderers and a channel between them used to pass the result of the setup phase.
// Returns two functions that can be used to, respectively, start and stop the whole system.
// The provided preblock sink gets closed after Process produces the last preblock.
func Process(setupConf, conf config.Config, ds core.DataSource, ps core.PreblockSink) (start func(), stop func(), err error) {
	wtkchan := make(chan *tss.WeakThresholdKey, 1)
	startSetup, stopSetup, setupErr := setup(setupConf, wtkchan)
	if setupErr != nil {
		return nil, nil, errors.New("an error occurred while initializing setup: " + setupErr.Error())
	}
	startConsensus, stopConsensus, consensusErr := consensus(conf, wtkchan, ds, ps)
	if consensusErr != nil {
		return nil, nil, errors.New("an error occurred while initializing consensus: " + consensusErr.Error())
	}
	start = func() {
		startSetup()
		startConsensus()
	}
	stop = func() {
		stopSetup()
		stopConsensus()
	}
	return start, stop, nil
}

// NoBeacon is a counterpart of Process that does not perform the setup phase.
// Instead, a fixed seeded WeakThresholdKey is used for the main consensus.
// NoBeacon should be used for testing purposes only! Returns start and stop functions.
func NoBeacon(conf config.Config, ds core.DataSource, ps core.PreblockSink) (func(), func(), error) {
	wtkchan := make(chan *tss.WeakThresholdKey, 1)
	wtkchan <- tss.SeededWTK(conf.NProc, conf.Pid, 2137, nil)
	start, stop, err := consensus(conf, wtkchan, ds, ps)
	if err != nil {
		return nil, nil, err
	}
	return start, stop, nil
}

func consensus(conf config.Config, wtkchan chan *tss.WeakThresholdKey, ds core.DataSource, ps core.PreblockSink) (func(), func(), error) {
	log, err := logging.NewLogger(conf)
	if err != nil {
		return nil, nil, err
	}

	makePreblock := func(units []gomel.Unit) {
		ps <- gomel.ToPreblock(units)
		timingUnit := units[len(units)-1]
		if timingUnit.Level() == conf.LastLevel && timingUnit.EpochID() == gomel.EpochID(conf.NumberOfEpochs-1) {
			// we have just sent the last preblock of the last epoch, it's safe to quit
			close(ps)
		}
	}

	ord := orderer.New(conf, ds, makePreblock, log)
	syn, err := syncer.New(conf, ord, log, false)
	if err != nil {
		return nil, nil, err
	}
	netserv, err := tcp.NewServer(conf.RMCAddresses[conf.Pid], conf.RMCAddresses, log)
	if err != nil {
		return nil, nil, err
	}
	alrt, err := forking.NewAlerter(conf, ord, netserv, log)
	if err != nil {
		return nil, nil, err
	}

	started := make(chan struct{})
	start := func() {
		go func() {
			defer func() { started <- struct{}{} }()
			wtkey, ok := <-wtkchan
			if !ok {
				// received termination signal from outside
				return
			}

			logWTK(log, wtkey)

			conf.WTKey = wtkey
			ord.Start(coin.NewFactory(conf.Pid, wtkey), syn, alrt)
		}()
	}
	stop := func() {
		<-started
		netserv.Stop()
		ord.Stop()
	}
	return start, stop, nil
}

func setup(conf config.Config, wtkchan chan *tss.WeakThresholdKey) (func(), func(), error) {
	log, err := logging.NewLogger(conf)
	if err != nil {
		return nil, nil, err
	}

	rsf, err := beacon.New(conf)
	if err != nil {
		return nil, nil, err
	}

	extractHead := func(units []gomel.Unit) {
		head := units[len(units)-1]
		if head.Level() == conf.OrderStartLevel {
			wtkchan <- rsf.GetWTK(head.Creator())
			return
		}
		panic("Setup phase: wrong level")
	}

	ord := orderer.New(conf, nil, extractHead, log)
	syn, err := syncer.New(conf, ord, log, true)
	if err != nil {
		return nil, nil, err
	}

	start := func() {
		ord.Start(rsf, syn, gomel.NopAlerter())
	}
	stop := func() {
		ord.Stop()
		close(wtkchan)
	}
	return start, stop, nil
}

func logWTK(log zerolog.Logger, wtkey *tss.WeakThresholdKey) {
	providers := make([]uint16, 0, len(wtkey.ShareProviders()))
	for provider := range wtkey.ShareProviders() {
		providers = append(providers, provider)
	}
	log.Log().
		Uint16(logging.WTKThreshold, wtkey.Threshold()).
		Uints16(logging.WTKShareProviders, providers).
		Msg(logging.GotWTK)

}
