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

type dec struct {
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
func newDecoder(r io.Reader) decoder {
	return &dec{r}
}

// decodeCrown reads encoded data from the io.Reader and tries to decode it as a crown.
func (d *dec) decodeCrown() (*gomel.Crown, error) {
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

// decodePreunit reads encoded data from the io.Reader and tries to decode it
// as a preunit.
func (d *dec) decodePreunit() (gomel.Preunit, error) {
	uint16Buf := make([]byte, 2)
	uint32Buf := make([]byte, 4)
	_, err := io.ReadFull(d, uint16Buf)
	if err != nil {
		return nil, err
	}
	creator := binary.LittleEndian.Uint16(uint16Buf)
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

	result := creating.NewPreunit(creator, crown, unitData, rsData)
	result.SetSignature(signature)
	return result, nil
}

func (d *dec) decodeAntichain() ([]gomel.Preunit, error) {
	k, err := d.decodeUint32()
	if err != nil {
		return nil, err
	}
	// The maximal possible antichain size is bounded by nProc^2
	// becuase each process may create only up to nProc forks.
	if k > uint32(d.nProc)*uint32(d.nProc) {
		return nil, errors.New("antichain length too long")
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

func (d *dec) decodeChunk() ([][]gomel.Preunit, int, error) {
	k, err := d.decodeUint32()
	if err != nil {
		return nil, 0, err
	}
	if k > config.MaxAntichainsInChunk {
		return nil, 0, errors.New("chunk contains too many antichains")
	}
	result := make([][]gomel.Preunit, k)
	nUnits := 0
	for i := range result {
		layer, err := d.decodeAntichain()
		if err != nil {
			return nil, 0, err
		}
		result[i] = layer
		nUnits += len(layer)
	}
	return result, nUnits, nil
}

func (d *dec) decodeUint32() (uint32, error) {
	buf := make([]byte, 4)
	_, err := io.ReadFull(d, buf)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(buf), nil
}
