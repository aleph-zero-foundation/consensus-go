package gossip

import (
	"encoding/binary"
	"io"

	"gitlab.com/alephledger/consensus-go/pkg/encoding/custom"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
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

func encodeUnitInfo(w io.Writer, ui unitInfo) error {
	err := encodeUint32(w, ui.height)
	if err != nil {
		return err
	}
	_, err = w.Write(ui.hash[:])
	return err
}

func decodeUnitInfo(r io.Reader) (unitInfo, error) {
	var result unitInfo
	var err error
	result.height, err = decodeUint32(r)
	if err != nil {
		return result, err
	}
	result.hash = &gomel.Hash{}
	_, err = io.ReadFull(r, result.hash[:])
	return result, err
}

func encodeProcessInfo(w io.Writer, pi processInfo) error {
	k := uint32(len(pi))
	err := encodeUint32(w, k)
	if err != nil {
		return err
	}
	for _, ui := range pi {
		err = encodeUnitInfo(w, ui)
		if err != nil {
			return err
		}
	}
	return nil
}

func decodeProcessInfo(r io.Reader) (processInfo, error) {
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
	return result, nil
}

func encodeRequests(w io.Writer, r requests, theirDagInfo dagInfo) error {
	k := uint32(0)
	nReq := uint32(0)
	for pid, processInfo := range theirDagInfo {
		k += uint32(len(processInfo))
		nReq += uint32(len(r[pid]))
	}
	err := encodeUint32(w, nReq)
	if err != nil {
		return err
	}
	if nReq != 0 {
		bs := newBitSet(k)
		position := uint32(0)

		for pid := range theirDagInfo {
			hashSet := newStaticHashSet((r)[pid])
			for _, uInfo := range theirDagInfo[pid] {
				if hashSet.contains(uInfo.hash) {
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

func decodeRequests(r io.Reader, myDagInfo dagInfo) (requests, error) {
	nProc := len(myDagInfo)
	k := uint32(0)
	for _, processInfo := range myDagInfo {
		k += uint32(len(processInfo))
	}
	nReq, err := decodeUint32(r)
	if err != nil {
		return nil, err
	}
	result := make(requests, nProc)
	if nReq != 0 {
		array := make([]byte, (k+7)>>3)
		_, err := io.ReadFull(r, array[:])
		if err != nil {
			return nil, err
		}
		bs := bitSetFromSlice(array)

		position := uint32(0)
		for pid, processInfo := range myDagInfo {
			for _, uInfo := range processInfo {
				if bs.test(position) {
					result[pid] = append(result[pid], uInfo.hash)
				}
				position++
			}
		}
	}
	return result, nil
}
