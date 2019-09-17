// Package sync implements a service that creates and runs all the necessary syncing servers.
package sync

import (
	"strconv"
	"time"

	"github.com/rs/zerolog"
	chdag "gitlab.com/alephledger/consensus-go/pkg/dag"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/network"
	"gitlab.com/alephledger/consensus-go/pkg/network/persistent"
	"gitlab.com/alephledger/consensus-go/pkg/network/tcp"
	"gitlab.com/alephledger/consensus-go/pkg/network/udp"
	"gitlab.com/alephledger/consensus-go/pkg/process"
	"gitlab.com/alephledger/consensus-go/pkg/sync"
	"gitlab.com/alephledger/consensus-go/pkg/sync/fallback"
	"gitlab.com/alephledger/consensus-go/pkg/sync/fetch"
	"gitlab.com/alephledger/consensus-go/pkg/sync/gossip"
	"gitlab.com/alephledger/consensus-go/pkg/sync/multicast"
)

// Note: it is required that every server type that is used as a fallback is initialized
// before a server type that uses it as a fallback server.

type service struct {
	servers     []sync.Server
	subservices []process.Service
	log         zerolog.Logger
}

// Checks if all fallbacks have their corresponding configurations.
func valid(configs []*process.Sync) error {
	if len(configs) == 0 {
		return gomel.NewConfigError("empty sync configuration")
	}
	definedFbks := map[string]bool{}
	for i, c := range configs {
		if c.Fallback == "" {
			continue
		}
		f := c.Fallback
		if definedFbks[f] {
			continue
		}
		definedFbks[f] = true
		if f == "retrying" {
			f = c.Params["retryingFallback"]
		}
		found := false
		if f == "fetch" {
			found = c.Type == "fetch"
		}
		for _, cPrev := range configs[:i] {
			if cPrev.Type == f {
				found = true
				break
			}
		}
		if !found {
			return gomel.NewConfigError("defined " + f + " as fallback, but there is no configuration for it")
		}
	}
	return nil
}

// getFallback builds a fallback for process.Sync configuration.
func getFallback(c *process.Sync, s *service, dag gomel.Dag, adder gomel.Adder, log zerolog.Logger) (sync.Fallback, chan uint16, chan fetch.Request, error) {
	var fbk sync.Fallback
	nProc := dag.NProc()
	switch c.Fallback {
	case "gossip":
		reqChan := make(chan uint16, nProc)
		fbk = fallback.NewGossip(reqChan)
		return fbk, reqChan, nil, nil
	case "fetch":
		reqChan := make(chan fetch.Request, nProc)
		fbk = fallback.NewFetch(dag, reqChan)
		return fbk, nil, reqChan, nil
	case "retrying":
		var baseFbk sync.Fallback
		log := log.With().Int(logging.Service, logging.RetryingService).Logger()
		rif, err := strconv.ParseFloat(c.Params["retryingInterval"], 64)
		ri := time.Duration(rif)
		if err != nil {
			return nil, nil, nil, err
		}
		switch c.Params["retryingFallback"] {
		case "gossip":
			reqChan := make(chan uint16, nProc)
			baseFbk = fallback.NewGossip(reqChan)
			fbk = fallback.NewRetrying(baseFbk, dag, adder, ri, log)
			return fbk, reqChan, nil, nil
		case "fetch":
			reqChan := make(chan fetch.Request, nProc)
			baseFbk = fallback.NewFetch(dag, reqChan)
			retrying := fallback.NewRetrying(baseFbk, dag, adder, ri, log)
			s.subservices = append(s.subservices, retrying)
			fbk = retrying
			return fbk, nil, reqChan, nil
		default:
			return nil, nil, nil, gomel.NewConfigError("fallback param for retrying cannot be empty")
		}
	default:
		fbk = sync.NopFallback()
	}
	return fbk, nil, nil, nil
}

func isFallback(name string, configs []*process.Sync) int {
	for i, c := range configs {
		if c.Fallback == name {
			return i
		}
	}
	return -1
}

// NewService creates a new syncing service for the given dag, with the given config.
// The returned dag will multicast units created by us, when they are added to it, when multicast is in use.
func NewService(dag gomel.Dag, adder gomel.Adder, configs []*process.Sync, log zerolog.Logger) (process.Service, gomel.Dag, error) {
	if err := valid(configs); err != nil {
		return nil, nil, err
	}
	resultDag := dag
	pid := configs[0].Pid
	nProc := dag.NProc()
	s := &service{log: log.With().Int(logging.Service, logging.SyncService).Logger()}
	fallbacks := make(map[string]sync.Fallback)

	for i, c := range configs {
		var (
			netserv network.Server
			server  sync.Server
			fbk     sync.Fallback
		)
		tf, err := strconv.ParseFloat(c.Params["timeout"], 64)
		if err != nil {
			return nil, nil, err
		}
		t := time.Duration(tf) * time.Second
		switch c.Type {
		case "multicast":
			log = log.With().Int(logging.Service, logging.MCService).Logger()
			var err error
			switch c.Params["mcType"] {
			case "tcp":
				netserv, err = tcp.NewServer(c.LocalAddress, c.RemoteAddresses, log)
			case "udp":
				netserv, err = udp.NewServer(c.LocalAddress, c.RemoteAddresses, log)
			case "pers":
				netserv, err = persistent.NewServer(c.LocalAddress, c.RemoteAddresses, log)
				s.subservices = append(s.subservices, netserv.(process.Service))
			default:
				return nil, nil, gomel.NewConfigError("wrong multicast type")
			}
			if err != nil {
				return nil, nil, err
			}
			fbk = fallbacks[c.Fallback]
			if fbk == nil {
				fbk = sync.NopFallback()
			}
			var cb func(gomel.Unit)
			server, cb = multicast.NewServer(pid, dag, adder, netserv, t, fbk, log)
			resultDag = chdag.AfterEmplace(dag, cb)
		case "gossip":
			log = log.With().Int(logging.Service, logging.GossipService).Logger()

			netserv, err := tcp.NewServer(c.LocalAddress, c.RemoteAddresses, log)
			if err != nil {
				return nil, nil, err
			}

			var peerSource gossip.PeerSource
			if id := isFallback("fetch", configs[i+1:]); id != -1 || (c.Fallback == "retrying" && c.Params["retryingFallback"] == "gossip") {
				fbk, reqChan, _, err := getFallback(configs[i+1+id], s, dag, adder, log)
				fallbacks["gossip"] = fbk
				if err != nil {
					return nil, nil, err
				}
				peerSource = gossip.NewMixedPeerSource(nProc, pid, reqChan)
			} else {
				peerSource = gossip.NewDefaultPeerSource(nProc, pid)
			}
			nOut, err := strconv.Atoi(c.Params["nOut"])
			if err != nil {
				return nil, nil, err
			}
			nIn, err := strconv.Atoi(c.Params["nIn"])
			if err != nil {
				return nil, nil, err
			}
			server = gossip.NewServer(pid, dag, adder, netserv, peerSource, t, log, nOut, nIn)
		case "fetch":
			log = log.With().Int(logging.Service, logging.FetchService).Logger()
			netserv, err := tcp.NewServer(c.LocalAddress, c.RemoteAddresses, log)
			if err != nil {
				return nil, nil, err
			}

			var reqChan chan fetch.Request
			if id := isFallback("fetch", configs[i+1:]); id != -1 || c.Fallback == "fetch" || (c.Fallback == "retrying" && c.Params["retryingFallback"] == "fetch") {
				fbk, _, reqChan, err = getFallback(configs[i+1+id], s, dag, adder, log)
				if id != 1 || c.Fallback == "fetch" {
					fallbacks["fetch"] = fbk
				}
				if c.Fallback == "retrying" && c.Params["retryingFallback"] == "fetch" {
					fallbacks["retrying"] = fbk
				}

				if err != nil {
					return nil, nil, err
				}
			}

			fbk = fallbacks[c.Fallback]
			nOut, err := strconv.Atoi(c.Params["nOut"])
			if err != nil {
				return nil, nil, err
			}
			nIn, err := strconv.Atoi(c.Params["nIn"])
			if err != nil {
				return nil, nil, err
			}
			server = fetch.NewServer(pid, dag, adder, reqChan, netserv, t, fbk, log, nOut, nIn)
		}
		s.servers = append(s.servers, server)
	}

	return s, resultDag, nil
}

func (s *service) Start() error {
	for _, service := range s.subservices {
		service.Start()
	}
	for _, server := range s.servers {
		server.Start()
	}
	s.log.Info().Msg(logging.ServiceStarted)
	return nil
}

func (s *service) Stop() {
	for i := len(s.servers) - 1; i >= 0; i-- {
		s.servers[i].StopOut()
	}

	// let other processes sync with us some more
	time.Sleep(5 * time.Second)
	for i := len(s.servers) - 1; i >= 0; i-- {
		s.servers[i].StopIn()
	}
	for _, service := range s.subservices {
		service.Stop()
	}
	s.log.Info().Msg(logging.ServiceStopped)
}
