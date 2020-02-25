// Package run defines a function for running the whole protocol, using services defined in other packages.
package run

import (
	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/forking"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/orderer"
	"gitlab.com/alephledger/consensus-go/pkg/random/coin"
	"gitlab.com/alephledger/consensus-go/pkg/sync/syncer"
	"gitlab.com/alephledger/core-go/pkg/core"
	"gitlab.com/alephledger/core-go/pkg/crypto/tss"
)

func consensus(conf config.Config, wtkchan chan *tss.WeakThresholdKey, ds core.DataSource, ps core.PreblockSink) error {
	log, err := logging.NewLogger(conf)
	if err != nil {
		return err
	}

	wtkey, ok := <-wtkchan
	if !ok {
		// received termination signal from outside
		return nil
	}
	log.Info().Msg(logging.GotWeakThresholdKey)
	conf.WTKey = wtkey
	rsf := coin.NewFactory(conf.Pid, wtkey)

	makePreblock := func(units []gomel.Unit) {
		ps <- gomel.ToPreblock(units)
	}

	ord := orderer.New(conf, rsf, ds, makePreblock, log)
	syn, err := syncer.New(conf, ord, log)
	if err != nil {
		return err
	}
	alrt, err := forking.NewAlerter(conf, ord, log)
	if err != nil {
		return err
	}
	ord.Start(syn, alrt)
	return nil
}

func Process(conf, setupConf config.Config, ds core.DataSource, ps core.PreblockSink) error {
	wtkchan := make(chan *tss.WeakThresholdKey, 1)
	stopSetup, err := setup(setupConf, wtkchan, 0)
	if err != nil {
		return err
	}
	defer stopSetup()
	err = consensus(conf, wtkchan, ds, ps)
	if err != nil {
		return err
	}
	return nil
}

func NoBeacon(conf config.Config, ds core.DataSource, ps core.PreblockSink) error {
	wtkchan := make(chan *tss.WeakThresholdKey, 1)
	setup(conf, wtkchan, 2137)
	err := consensus(conf, wtkchan, ds, ps)
	if err != nil {
		return err
	}
	return nil
}
