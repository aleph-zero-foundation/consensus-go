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
