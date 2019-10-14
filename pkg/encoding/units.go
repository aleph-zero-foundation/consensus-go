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
func DecodePreunit(data []byte, nProc uint16) (gomel.Preunit, error) {
	decoder := newDecoder(bytes.NewReader(data), nProc)
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
func DecodeCrown(data []byte, nProc uint16) (*gomel.Crown, error) {
	decoder := newDecoder(bytes.NewReader(data), nProc)
	return decoder.decodeCrown()
}

// SendUnit writes encoded unit to writer.
func SendUnit(unit gomel.Unit, w io.Writer) error {
	return newEncoder(w).encodeUnit(unit)
}

// ReceivePreunit decodes a preunit from reader.
func ReceivePreunit(r io.Reader, nProc uint16) (gomel.Preunit, error) {
	return newDecoder(r, nProc).decodePreunit()
}

// SendChunk encodes units and writes them to writer.
func SendChunk(units []gomel.Unit, w io.Writer) error {
	return newEncoder(w).encodeChunk(units)
}

// ReceiveChunk decodes slice of preunit antichains from reader.
func ReceiveChunk(r io.Reader, nProc uint16) ([][]gomel.Preunit, int, error) {
	return newDecoder(r, nProc).decodeChunk()
}

func computeLayer(u gomel.Unit, layer map[gomel.Unit]int) int {
	if layer[u] == -1 {
		maxParentLayer := 0
		for _, v := range u.Parents() {
			if computeLayer(v, layer) > maxParentLayer {
				maxParentLayer = computeLayer(v, layer)
			}
		}
		layer[u] = maxParentLayer + 1
	}
	return layer[u]
}

// toLayers divides the provided units into antichains, so that each antichain is
// maximal, and depends only on units from outside or from previous antichains.
func toLayers(units []gomel.Unit) [][]gomel.Unit {
	layer := map[gomel.Unit]int{}
	maxLayer := 0
	for _, u := range units {
		layer[u] = -1
	}
	for _, u := range units {
		layer[u] = computeLayer(u, layer)
		if layer[u] > maxLayer {
			maxLayer = layer[u]
		}
	}
	result := make([][]gomel.Unit, maxLayer)
	for _, u := range units {
		result[layer[u]-1] = append(result[layer[u]-1], u)
	}
	return result
}
