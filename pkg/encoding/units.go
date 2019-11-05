package encoding

import (
	"bytes"
	"io"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

// EncodeUnit encodes a unit to a slice of bytes.
func EncodeUnit(unit gomel.Unit) ([]byte, error) {
	var buf bytes.Buffer
	encoder := newEncoder(&buf)
	err := encoder.encodeUnit(unit)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// DecodePreunit checks decodes the given data into preunit. Complementary to EncodeUnit.
func DecodePreunit(data []byte) (gomel.Preunit, error) {
	decoder := newDecoder(bytes.NewReader(data))
	return decoder.decodePreunit()
}

// EncodeCrown encodes a crown to a slice of bytes.
func EncodeCrown(crown *gomel.Crown) ([]byte, error) {
	var buf bytes.Buffer
	encoder := newEncoder(&buf)
	err := encoder.encodeCrown(crown)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// DecodeCrown checks decodes the given data into crown. Complementary to EncodeCrown.
func DecodeCrown(data []byte) (*gomel.Crown, error) {
	decoder := newDecoder(bytes.NewReader(data))
	return decoder.decodeCrown()
}

// SendUnit writes encoded unit to writer.
func SendUnit(unit gomel.Unit, w io.Writer) error {
	return newEncoder(w).encodeUnit(unit)
}

// ReceivePreunit decodes a preunit from reader.
func ReceivePreunit(r io.Reader) (gomel.Preunit, error) {
	return newDecoder(r).decodePreunit()
}

// SendChunk encodes units and writes them to writer.
func SendChunk(units []gomel.Unit, w io.Writer) error {
	return newEncoder(w).encodeChunk(units)
}

// ReceiveChunk decodes slice of preunit antichains from reader.
func ReceiveChunk(r io.Reader) ([]gomel.Preunit, error) {
	return newDecoder(r).decodeChunk()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func computeLayer(u gomel.Unit, layers map[gomel.Unit]int) int {
	if layers[u] == -1 {
		maxParentLayer := 0
		for _, v := range u.Parents() {
			maxParentLayer = max(maxParentLayer, computeLayer(v, layers))
		}
		layers[u] = maxParentLayer + 1
	}
	return layers[u]
}

// topSort sorts the given slice of units in the topological order.
func topSort(units []gomel.Unit) []gomel.Unit {
	layers := map[gomel.Unit]int{}
	for _, u := range units {
		layers[u] = -1
	}
	for _, u := range units {
		layers[u] = computeLayer(u, layers)
	}
	maxLayer := -1
	for _, u := range units {
		maxLayer = max(maxLayer, layers[u])
	}
	result := make([]gomel.Unit, 0, len(units))
	for layer := 0; layer <= maxLayer; layer++ {
		for _, u := range units {
			if layers[u] == layer {
				result = append(result, u)
			}
		}
	}
	return result
}
