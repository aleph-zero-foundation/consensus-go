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

func encodeRequests(w io.Writer, r *requests) error {
	k := uint32(0)
	for _, hs := range *r {
		for range hs {
			k++
		}
	}
	err := encodeUint32(w, k)
	if err != nil {
		return err
	}
	for i, hs := range *r {
		for _, h := range hs {
			err := encodeUint32(w, uint32(i))
			if err != nil {
				return err
			}
			_, err = w.Write(h[:])
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func decodeRequests(r io.Reader, nProc int) (*requests, error) {
	k, err := decodeUint32(r)
	if err != nil {
		return nil, err
	}
	result := make(requests, nProc)
	for i := uint32(0); i < k; i++ {
		j, err := decodeUint32(r)
		if err != nil {
			return nil, err
		}
		if j > uint32(nProc) {
			return nil, errors.New("invalid process id in requests")
		}
		h := &gomel.Hash{}
		_, err = io.ReadFull(r, h[:])
		if err != nil {
			return nil, err
		}
		result[j] = append(result[j], h)
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
