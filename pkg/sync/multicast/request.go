package multicast

import (
	"bytes"
	"math/rand"

	"gitlab.com/alephledger/consensus-go/pkg/encoding/custom"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

//MCRequest represents a request to send the encoded unit to the committee member indicated by pid.
type MCRequest struct {
	encUnit []byte
	height  int
	pid     uint16
}

//Request encodes the given unit and pushes to the provided channel MCRequests to send that unit to every committee member other than pid.
func Request(unit gomel.Unit, requests chan<- MCRequest, pid, nProc int) error {
	buffer := &bytes.Buffer{}
	encoder := custom.NewEncoder(buffer)
	err := encoder.EncodeUnit(unit)
	if err != nil {
		return err
	}
	encUnit := buffer.Bytes()[:]
	perm := rand.Perm(nProc)
	for i := 0; i < nProc; i++ {
		if perm[i] == pid {
			continue
		}
		requests <- MCRequest{encUnit, unit.Height(), uint16(perm[i])}
	}
	return nil
}
