package encoding

import (
	"bytes"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

//EncodeUnit encodes a unit to a slice of bytes.
func EncodeUnit(unit gomel.Unit) ([]byte, error) {
	var buf bytes.Buffer
	encoder := NewEncoder(&buf)
	err := encoder.EncodeUnit(unit)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// DecodePreunit checks decodes the given data into preunit. Complementary to EncodeUnit.
func DecodePreunit(data []byte) (gomel.Preunit, error) {
	decoder := NewDecoder(bytes.NewReader(data))
	return decoder.DecodePreunit()
}

func toLayers([]gomel.Unit) [][]gomel.Unit {
	return nil
}
