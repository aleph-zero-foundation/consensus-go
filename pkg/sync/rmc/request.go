package rmc

import (
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

const (
	sendData byte = iota
	sendProof
)

// request represents a request to a multicast server
type request struct {
	msgType byte
	id      uint64
	data    []byte
}

// newRequest returns a request with given parameters
func newRequest(id uint64, data []byte, msgType byte) *request {
	return &request{
		msgType: msgType,
		id:      id,
		data:    data,
	}
}

func unitID(u gomel.Unit, nProc uint16) uint64 {
	return uint64(u.Creator()) + uint64(nProc)*uint64(u.Height())
}

func preunitID(pu gomel.Preunit, nProc uint16) uint64 {
	return uint64(pu.Creator()) + uint64(nProc)*uint64(pu.View().Heights[pu.Creator()]+1)
}
