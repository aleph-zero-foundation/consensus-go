package rmc

import (
	"bytes"

	"gitlab.com/alephledger/consensus-go/pkg/encoding/custom"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type category byte

const (
	unit category = iota
	alert
)

func EncodeUnit(u gomel.Unit) ([]byte, error) {
	var buf bytes.Buffer

	buf.Write([]byte{byte(unit)})

	encoder := custom.NewEncoder(&buf)
	err := encoder.EncodeUnit(u)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func DecodePreunit(data []byte) (gomel.Preunit, error) {
	if data[0] != byte(unit) {
		return nil, nil
	}
	decoder := custom.NewDecoder(bytes.NewReader(data[1:]))
	return decoder.DecodePreunit()
}
