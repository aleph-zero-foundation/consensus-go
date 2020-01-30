package encoding

import (
	"encoding/binary"
	"errors"
	"io"
	"math"

	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/creating"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type decoder struct {
	io.Reader
}

// newDecoder creates a new encoding.Decoder that is threadsafe.
// It assumes the data encodes units in the following format:
//  1. Creator id, 2 bytes.
//  2. Signature, 64 bytes.
//  3. Number of parents, 2 bytes.
//  4. Parent heights, 4 bytes each.
//  5. Control hash 32 bytes.
//  6. Size of the unit data in bytes, 4 bytes.
//  7. The unit data, as much as declared in 6.
//  8. Size of the random source data in bytes, 4 bytes.
//  9. The random source data, as much as declared in 8.
// All integer values are encoded as 16 or 32 bit unsigned ints.
// It is guaranteed to read only as much data as needed.
func newDecoder(r io.Reader) *decoder {
	return &decoder{r}
}

// decodeCrown reads encoded data from the io.Reader and tries to decode it as a crown.
func (d *decoder) decodeCrown() (*gomel.Crown, error) {
	uint16Buf := make([]byte, 2)
	uint32Buf := make([]byte, 4)

	_, err := io.ReadFull(d, uint16Buf)
	if err != nil {
		return nil, err
	}
	nProc := binary.LittleEndian.Uint16(uint16Buf)

	heights := make([]int, nProc)
	for i := range heights {
		_, err := io.ReadFull(d, uint32Buf)
		if err != nil {
			return nil, err
		}
		h := binary.LittleEndian.Uint32(uint32Buf)
		if h == math.MaxUint32 {
			heights[i] = -1
		} else {
			heights[i] = int(h)
		}
	}
	controlHash := &gomel.Hash{}
	_, err = io.ReadFull(d, controlHash[:])
	if err != nil {
		return nil, err
	}
	return gomel.NewCrown(heights, controlHash), nil
}

func (d *decoder) decodeDagInfo() (*gomel.DagInfo, error) {
	uint16Buf := make([]byte, 2)
	uint32Buf := make([]byte, 4)

	_, err := io.ReadFull(d, uint32Buf)
	if err != nil {
		return nil, err
	}
	epoch := binary.LittleEndian.Uint32(uint32Buf)

	_, err = io.ReadFull(d, uint16Buf)
	if err != nil {
		return nil, err
	}
	nProc := binary.LittleEndian.Uint16(uint16Buf)

	heights := make([]int, nProc)
	for i := range heights {
		_, err = io.ReadFull(d, uint32Buf)
		if err != nil {
			return nil, err
		}
		h := binary.LittleEndian.Uint32(uint32Buf)
		if h == math.MaxUint32 {
			heights[i] = -1
		} else {
			heights[i] = int(h)
		}
	}
	return &gomel.DagInfo{int(epoch), heights}, nil
}

// decodePreunit reads encoded data from the io.Reader and tries to decode it as a preunit.
func (d *decoder) decodePreunit() (gomel.Preunit, error) {
	uint64Buf := make([]byte, 8)
	uint32Buf := uint64Buf[:4]
	uint16Buf := uint32Buf[:2]
	_, err := io.ReadFull(d, uint16Buf)
	if err != nil {
		return nil, err
	}
	creator := binary.LittleEndian.Uint16(uint16Buf)
	if creator == math.MaxUint16 {
		return nil, nil
	}

	_, err = io.ReadFull(d, uint64Buf)
	if err != nil {
		return nil, err
	}
	epochID := binary.LittleEndian.Uint64(uint64Buf)

	signature := make([]byte, 64)
	_, err = io.ReadFull(d, signature)
	if err != nil {
		return nil, err
	}
	crown, err := d.decodeCrown()
	if err != nil {
		return nil, err
	}

	_, err = io.ReadFull(d, uint32Buf)
	if err != nil {
		return nil, err
	}
	unitDataLen := binary.LittleEndian.Uint32(uint32Buf)
	if unitDataLen > config.MaxDataBytesPerUnit {
		return nil, errors.New("maximal allowed data size in a preunit exceeded")
	}
	unitData := make([]byte, unitDataLen)
	_, err = io.ReadFull(d, unitData)
	if err != nil {
		return nil, err
	}
	_, err = io.ReadFull(d, uint32Buf)
	if err != nil {
		return nil, err
	}
	rsDataLen := binary.LittleEndian.Uint32(uint32Buf)
	if rsDataLen > config.MaxRandomSourceDataBytesPerUnit {
		return nil, errors.New("maximal allowed random source data size in a preunit exceeded")
	}
	rsData := make([]byte, rsDataLen)
	_, err = io.ReadFull(d, rsData)
	if err != nil {
		return nil, err
	}

	result := creating.NewPreunit(creator, gomel.EpochID(epochID), crown, unitData, rsData)
	result.SetSignature(signature)
	return result, nil
}

func (d *decoder) decodeChunk() ([]gomel.Preunit, error) {
	k, err := d.decodeUint32()
	if err != nil {
		return nil, err
	}
	if k > config.MaxUnitsInChunk {
		return nil, errors.New("chunk contains too many units")
	}
	result := make([]gomel.Preunit, k)
	for i := range result {
		result[i], err = d.decodePreunit()
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (d *decoder) decodeUint32() (uint32, error) {
	buf := make([]byte, 4)
	_, err := io.ReadFull(d, buf)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(buf), nil
}
