package gossip

import (
	"encoding/binary"
	"errors"
	"io"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/encoding/custom"
)

func encodeUint32(w io.Writer, i uint32) error {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, i)
	_, err := w.Write(buf)
	return err
}

func decodeUint32(r io.Reader) (uint32, error) {
	buf := make([]byte, 4)
	_, err := io.ReadFull(r, buf)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(buf), nil
}

func encodeUnitInfo(w io.Writer, ui *unitInfo) error {
	err := encodeUint32(w, ui.height)
	if err != nil {
		return err
	}
	_, err = w.Write(ui.hash[:])
	return err
}

func decodeUnitInfo(r io.Reader) (*unitInfo, error) {
	result := &unitInfo{}
	var err error
	result.height, err = decodeUint32(r)
	if err != nil {
		return nil, err
	}
	result.hash = &gomel.Hash{}
	_, err = io.ReadFull(r, result.hash[:])
	return result, err
}

func encodeProcessInfo(w io.Writer, pi *processInfo) error {
	k := uint32(len(*pi))
	err := encodeUint32(w, k)
	if err != nil {
		return err
	}
	for _, ui := range *pi {
		err = encodeUnitInfo(w, ui)
		if err != nil {
			return err
		}
	}
	return nil
}

func decodeProcessInfo(r io.Reader) (*processInfo, error) {
	k, err := decodeUint32(r)
	if err != nil {
		return nil, err
	}
	result := make(processInfo, k)
	for i := range result {
		result[i], err = decodeUnitInfo(r)
		if err != nil {
			return nil, err
		}
	}
	return &result, nil
}

func encodeRequests(w io.Writer, r *requests, theirPosetInfo posetInfo) error {
	k := 0
	for _, processInfo := range theirPosetInfo {
		k += len(processInfo)
	}
	err := encodeUint32(w, uint32(k))
	if err != nil {
		return err
	}
	if k != 0 {
		bs := newBitSet(k)
		position := 0

		for pid := 0; pid < len(*r); pid++ {
			hashSet := newStaticHashSet((*r)[pid])
			for _, uInfo := range theirPosetInfo[pid] {
				if hashSet.contains((*uInfo).hash) {
					bs.set(position)
				}
				position++
			}
		}
		_, err := w.Write(bs.toSlice()[:])
		if err != nil {
			return err
		}
	}
	return nil
}

func decodeRequests(r io.Reader, myPosetInfo posetInfo) (*requests, error) {
	nProc := len(myPosetInfo)
	myK := 0
	for _, processInfo := range myPosetInfo {
		myK += len(processInfo)
	}
	k, err := decodeUint32(r)
	if err != nil {
		return nil, err
	}
	if k != uint32(myK) {
		return nil, errors.New("received wrong length of requests bitset")
	}
	result := make(requests, nProc)
	if k != 0 {
		array := make([]byte, (k+7)>>3)
		_, err := io.ReadFull(r, array[:])
		if err != nil {
			return nil, err
		}
		bs := bitSetFromSlice(array)

		position := 0
		for pid, processInfo := range myPosetInfo {
			for _, uInfo := range processInfo {
				if bs.test(position) {
					result[pid] = append(result[pid], (*uInfo).hash)
				}
				position++
			}
		}
	}
	return &result, nil
}

func encodeLayer(w io.Writer, layer []gomel.Unit) error {
	err := encodeUint32(w, uint32(len(layer)))
	if err != nil {
		return err
	}
	encoder := custom.NewEncoder(w)
	for _, u := range layer {
		err = encoder.EncodeUnit(u)
		if err != nil {
			return err
		}
	}
	return nil
}

func decodeLayer(r io.Reader) ([]gomel.Preunit, error) {
	k, err := decodeUint32(r)
	if err != nil {
		return nil, err
	}
	decoder := custom.NewDecoder(r)
	result := make([]gomel.Preunit, k)
	for i := range result {
		result[i], err = decoder.DecodePreunit()
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func encodeUnits(w io.Writer, units [][]gomel.Unit) error {
	err := encodeUint32(w, uint32(len(units)))
	if err != nil {
		return err
	}
	for _, layer := range units {
		err := encodeLayer(w, layer)
		if err != nil {
			return err
		}
	}
	return nil
}

func decodeUnits(r io.Reader) ([][]gomel.Preunit, int, error) {
	k, err := decodeUint32(r)
	if err != nil {
		return nil, 0, err
	}
	result := make([][]gomel.Preunit, k)
	nUnits := 0
	for i := range result {
		layer, err := decodeLayer(r)
		if err != nil {
			return nil, 0, err
		}
		result[i] = layer
		nUnits += len(layer)
	}
	return result, nUnits, nil
}
