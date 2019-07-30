package sync

import (
	"time"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/network"
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
	servers []sync.Server
	log     zerolog.Logger
}

// Checks if all fallbacks have their corresponding configurations.
func valid(configs []*process.Sync) error {
	if len(configs) == 0 {
		return gomel.NewConfigError("empty sync configuration")
	}
	for i, c := range configs {
		if c.Fallback != "" {
			f := c.Fallback
			if f == "retrying" {
				switch c.Params["fallback"] {
				case 0:
					f = "gossip"
				case 1:
					f = "fetch"
				default:
					gomel.NewConfigError("defined " + f + " as fallback, but didn't specify correct fallback for it")
				}
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
	}
	return nil
}

// Builds fallback for process.Sync configuration
func getFallback(c *process.Sync, s *service, dag gomel.Dag, randomSource gomel.RandomSource, log zerolog.Logger) (sync.Fallback, chan uint16, chan fetch.Request, error) {
	var fbk sync.Fallback
	switch c.Fallback {
	case "gossip":
		reqChan := make(chan uint16)
		fbk = fallback.NewGossip(reqChan)
		return fbk, reqChan, nil, nil
	case "fetch":
		reqChan := make(chan fetch.Request)
		fbk = fallback.NewFetch(dag, reqChan)
		return fbk, nil, reqChan, nil
	case "retrying":
		var baseFbk sync.Fallback
		log := log.With().Int(logging.Service, logging.RetryingService).Logger()
		ri := time.Duration(float64(c.Params["retryingInterval"]))
		switch c.Params["fallback"] {
		case 0:
			reqChan := make(chan uint16)
			baseFbk = fallback.NewGossip(reqChan)
			fbk = fallback.NewRetrying(baseFbk, dag, randomSource, ri, log)
			return fbk, reqChan, nil, nil
		case 1:
			reqChan := make(chan fetch.Request)
			baseFbk = fallback.NewFetch(dag, reqChan)
			fbk = fallback.NewRetrying(baseFbk, dag, randomSource, ri, log)
			return fbk, nil, reqChan, nil
		default:
			return nil, nil, nil, gomel.NewConfigError("fallback param for retrying cannot be empty")
		}
	default:
		fbk = sync.Noop()
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
func NewService(dag gomel.Dag, randomSource gomel.RandomSource, configs []*process.Sync, attemptTiming chan<- int, log zerolog.Logger) (process.Service, func(gomel.Unit), error) {
	if err := valid(configs); err != nil {
		return nil, nil, err
	}
	pid := uint16(configs[0].Pid)
	nProc := uint16(dag.NProc())
	s := &service{log: log.With().Int(logging.Service, logging.SyncService).Logger()}
	fallbacks := make(map[string]sync.Fallback)
	callback := func(gomel.Unit) {}

	for i, c := range configs {
		var (
			dialer   network.Dialer
			listener network.Listener
			server   sync.Server
			fbk      sync.Fallback
		)
		t := time.Duration(float64(c.Params["Timeout"])) * time.Second
		switch c.Type {
		case "multicast":
			log = log.With().Int(logging.Service, logging.MCService).Logger()
			var err error
			switch c.Params["McType"] {
			case 0:
				dialer, listener, err = tcp.NewNetwork(c.LocalAddress, c.RemoteAddresses, log)
			case 1:
				dialer, listener, err = udp.NewNetwork(c.LocalAddress, c.RemoteAddresses, log)
			}
			if err != nil {
				return nil, nil, err
			}
			fbk = fallbacks[c.Fallback]
			if fbk == nil {
				fbk = sync.Noop()
			}
			server, callback = multicast.NewServer(pid, dag, randomSource, dialer, listener, t, fbk, log)
		case "gossip":
			log = log.With().Int(logging.Service, logging.GossipService).Logger()

			dialer, listener, err := tcp.NewNetwork(c.LocalAddress, c.RemoteAddresses, log)
			if err != nil {
				return nil, nil, err
			}

			var peerSource gossip.PeerSource
			if id := isFallback("fetch", configs[i+1:]); id != -1 {
				fbk, reqChan, _, err := getFallback(configs[i+1+id], s, dag, randomSource, log)
				fallbacks["gossip"] = fbk
				if err != nil {
					return nil, nil, err
				}
				peerSource = gossip.NewMixedPeerSource(nProc, pid, reqChan)
			} else {
				peerSource = gossip.NewDefaultPeerSource(nProc, pid)
			}
			server = gossip.NewServer(pid, dag, randomSource, dialer, listener, peerSource, t, attemptTiming, log, c.Params["nOut"], c.Params["nIn"])
		case "fetch":
			log = log.With().Int(logging.Service, logging.FetchService).Logger()
			dialer, listener, err := tcp.NewNetwork(c.LocalAddress, c.RemoteAddresses, log)
			if err != nil {
				return nil, nil, err
			}

			var reqChan chan fetch.Request
			if id := isFallback("fetch", configs[i+1:]); id != -1 || c.Fallback == "fetch" {
				fbk, _, reqChan, err = getFallback(configs[i+1+id], s, dag, randomSource, log)
				fallbacks["fetch"] = fbk
				if err != nil {
					return nil, nil, err
				}
			}

			fbk = fallbacks[c.Fallback]
			server = fetch.NewServer(pid, dag, randomSource, reqChan, dialer, listener, t, fbk, attemptTiming, log, c.Params["nOut"], c.Params["nIn"])
		}
		s.servers = append(s.servers, server)
	}

	return s, callback, nil
}

func (s *service) Start() error {
	for _, server := range s.servers {
		server.Start()
	}
	s.log.Info().Msg(logging.ServiceStarted)
	return nil
}

func (s *service) Stop() {
	for _, server := range s.servers {
		server.StopOut()
	}
	// let other processes sync with us some more
	time.Sleep(5 * time.Second)
	for _, server := range s.servers {
		server.StopIn()
	}
	s.log.Info().Msg(logging.ServiceStopped)
}
