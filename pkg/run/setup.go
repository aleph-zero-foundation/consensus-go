package run

import (
	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/orderer"
	"gitlab.com/alephledger/consensus-go/pkg/random/beacon"
	"gitlab.com/alephledger/consensus-go/pkg/sync/syncer"
	"gitlab.com/alephledger/core-go/pkg/crypto/tss"
)

func setup(conf config.Config, wtkchan chan *tss.WeakThresholdKey, seed int64) (func(), error) {
	if seed != 0 {
		wtkchan <- tss.SeededWTK(conf.NProc, conf.Pid, seed, nil)
		return func() {}, nil
	}

	log, err := logging.NewLogger(conf)
	if err != nil {
		return nil, err
	}

	rsf, err := beacon.New(conf)
	if err != nil {
		return nil, err
	}

	ch := make(chan uint16)
	extractHead := func(units []gomel.Unit) {
		head := units[len(units)-1]
		if head.Level() == conf.OrderStartLevel {
			ch <- units[len(units)-1].Creator()
		}
		panic("Setup phase: wrong level")
	}
	makeWTK := func() {
		if head, ok := <-ch; ok {
			wtkchan <- rsf.GetWTK(head)
		}
	}

	ord := orderer.New(conf, rsf, nil, extractHead, log)
	syn, err := syncer.New(conf, ord, log)
	if err != nil {
		return nil, err
	}

	go makeWTK()
	ord.Start(syn, gomel.NopAlerter())

	return func() {
		close(ch)
		ord.Stop()
	}, nil
}
