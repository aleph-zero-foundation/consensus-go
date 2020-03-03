// Package run defines API for running the whole consensus protocol.
package run

import (
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
func Process(setupConf, conf config.Config, ds core.DataSource, ps core.PreblockSink) (func(), func(), error) {
	wtkchan := make(chan *tss.WeakThresholdKey, 1)
	startSetup, stopSetup, err := setup(setupConf, wtkchan)
	if err != nil {
		return nil, nil, err
	}
	startConsensus, stopConsensus, err := consensus(conf, wtkchan, ds, ps)
	if err != nil {
		return nil, nil, err
	}
	start := func() {
		startSetup()
		go startConsensus()
	}
	stop := func() {
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
	}

	ord := orderer.New(conf, ds, makePreblock, log)
	syn, err := syncer.New(conf, ord, log)
	if err != nil {
		return nil, nil, err
	}
	netserv, err := tcp.NewServer(conf.RMCAddresses[conf.Pid], conf.RMCAddresses)
	if err != nil {
		return nil, nil, err
	}
	alrt, err := forking.NewAlerter(conf, ord, netserv, log)
	if err != nil {
		return nil, nil, err
	}

	start := func() {
		wtkey, ok := <-wtkchan
		if !ok {
			// received termination signal from outside
			return
		}
		log.Info().Msg(logging.GotWTK)
		conf.WTKey = wtkey
		ord.Start(coin.NewFactory(conf.Pid, wtkey), syn, alrt)
	}
	stop := func() {
		close(wtkchan)
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

	ch := make(chan uint16)
	extractHead := func(units []gomel.Unit) {
		head := units[len(units)-1]
		if head.Level() == conf.OrderStartLevel {
			ch <- units[len(units)-1].Creator()
			return
		}
		panic("Setup phase: wrong level")
	}
	makeWTK := func() {
		if head, ok := <-ch; ok {
			wtkchan <- rsf.GetWTK(head)
		}
	}

	ord := orderer.New(conf, nil, extractHead, log)
	syn, err := syncer.New(conf, ord, log)
	if err != nil {
		return nil, nil, err
	}

	start := func() {
		go makeWTK()
		ord.Start(rsf, syn, gomel.NopAlerter())
	}
	stop := func() {
		close(ch)
		ord.Stop()
	}
	return start, stop, nil
}
