package multicast

import (
	"bytes"
	"errors"

	"gitlab.com/alephledger/consensus-go/pkg/encoding/custom"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

const (
	sendData byte = iota
	sendFinished
)

type category byte

const (
	unit category = iota
	alert
)

// Request represents a request to a multicast server
type Request struct {
	msgType byte
	id      uint64
	pid     uint16
	data    []byte
}

// NewRequest returns a request with given parameters
func NewRequest(id uint64, pid uint16, data []byte, msgType byte) *Request {
	return &Request{
		msgType: msgType,
		id:      id,
		pid:     pid,
		data:    data,
	}
}

// NewUnitSendRequest creates multicast request to send given unit to the process with given id
func NewUnitSendRequest(u gomel.Unit, pid uint16, nProc int) (*Request, error) {
	data, err := encodeUnit(u)
	if err != nil {
		return nil, err
	}
	return &Request{
		msgType: sendData,
		id:      unitID(u, nProc),
		pid:     pid,
		data:    data,
	}, nil
}

func unitID(u gomel.Unit, nProc int) uint64 {
	return uint64(u.Creator()) + uint64(nProc)*uint64(u.Height())
}

func encodeUnit(u gomel.Unit) ([]byte, error) {
	var buf bytes.Buffer

	buf.Write([]byte{byte(unit)})

	encoder := custom.NewEncoder(&buf)
	err := encoder.EncodeUnit(u)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// DecodePreunit checks wheather the given data is representing a unit
// and decodes it using the method from custom package
func DecodePreunit(data []byte) (gomel.Preunit, error) {
	if data[0] != byte(unit) {
		return nil, errors.New("given data doesn't represent a unit")
	}
	decoder := custom.NewDecoder(bytes.NewReader(data[1:]))
	return decoder.DecodePreunit()
}
