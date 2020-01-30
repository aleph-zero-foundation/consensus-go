package encoding

import (
	"bytes"
	"io"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

// EncodeUnit encodes a unit to a slice of bytes.
func EncodeUnit(unit gomel.BaseUnit) ([]byte, error) {
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

// SendDagInfos encodes a slice of daginfos to writer.
func SendDagInfos(infos []*gomel.DagInfo, w io.Writer) error {
	enc := newEncoder(w)
	err := enc.encodeUint32(uint32(len(infos)))
	if err != nil {
		return err
	}
	for _, info := range infos {
		err = enc.encodeDagInfo(info)
		if err != nil {
			return err
		}
	}
	return nil
}

// ReceiveDagInfos decodes daginfo from the given data.
func ReceiveDagInfos(r io.Reader) ([]*gomel.DagInfo, error) {
	dec := newDecoder(r)
	n, err := dec.decodeUint32()
	if err != nil {
		return nil, err
	}
	infos := make([]*gomel.DagInfo, n)
	for i := range infos {
		info, err := dec.decodeDagInfo()
		if err != nil {
			return nil, err
		}
		infos[i] = info
	}
	return infos, nil
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

func computeLayer(u gomel.Unit, layers map[gomel.Unit]int) int {
	if layers[u] == -1 {
		maxParentLayer := 0
		for _, v := range u.Parents() {
			if cl := computeLayer(v, layers); cl > maxParentLayer {
				maxParentLayer = cl
			}
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
		if layers[u] > maxLayer {
			maxLayer = layers[u]
		}
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
