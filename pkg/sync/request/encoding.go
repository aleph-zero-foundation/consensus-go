package request

import (
	"encoding/binary"
	"errors"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
)

func (ui *unitInfo) MarshalBinary() ([]byte, error) {
	result := make([]byte, 4, 68)
	binary.LittleEndian.PutUint32(result[0:], ui.height)
	result = append(result, ui.hash[:]...)
	return result, nil
}

func (ui *unitInfo) UnmarshalBinary(data []byte) error {
	if len(data) != 68 {
		return errors.New("invalid unit info decoded")
	}
	ui.height = binary.LittleEndian.Uint32(data[0:])
	for i := range ui.hash {
		ui.hash[i] = data[4+i]
	}
	return nil
}

func (pi *processInfo) MarshalBinary() ([]byte, error) {
	k := uint32(len(*pi))
	result := make([]byte, 4, 4+k*68)
	binary.LittleEndian.PutUint32(result[0:], k)
	for i := uint32(0); i < k; i++ {
		unitInfo, err := (*pi)[i].MarshalBinary()
		if err != nil {
			return nil, err
		}
		result = append(result, unitInfo...)
	}
	return result, nil
}

func (pi *processInfo) UnmarshalBinary(data []byte) error {
	if len(data) < 4 {
		return errors.New("invalid process info decoded")
	}
	k := binary.LittleEndian.Uint32(data[0:])
	start := 4
	for i := uint32(0); i < k; i++ {
		(*pi) = append(*pi, &unitInfo{})
		err := (*pi)[i].UnmarshalBinary(data[start : start+68])
		if err != nil {
			return err
		}
		start += 68
	}
	return nil
}

func encodeRequest(i uint32, h gomel.Hash) []byte {
	result := make([]byte, 4, 68)
	binary.LittleEndian.PutUint32(result[0:], i)
	result = append(result, h[:]...)
	return result
}

func (r *requests) MarshalBinary() ([]byte, error) {
	result := make([]byte, 4)
	k := uint32(0)
	for i, hs := range *r {
		for _, h := range hs {
			result = append(result, encodeRequest(uint32(i), h)...)
			k++
		}
	}
	binary.LittleEndian.PutUint32(result[0:], k)
	return result, nil
}

func (r *requests) UnmarshalBinary(data []byte) error {
	if len(data) < 4 {
		return errors.New("invalid process info decoded")
	}
	k := binary.LittleEndian.Uint32(data[0:])
	start := 4
	for i := uint32(0); i < k; i++ {
		j := binary.LittleEndian.Uint32(data[start:])
		if j > uint32(len(*r)) {
			return errors.New("invalid process id in requests")
		}
		start += 4
		h := gomel.Hash{}
		for l := range h {
			h[l] = data[start+l]
		}
		(*r)[j] = append((*r)[j], h)
		start += 64
	}
	return nil
}
