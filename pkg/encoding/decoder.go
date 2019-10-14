package encoding

import (
	"encoding/binary"
	"io"
	"math"

	"gitlab.com/alephledger/consensus-go/pkg/creating"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type dec struct {
	io.Reader
	nProc uint16
}

// newDecoder creates a new encoding.Decoder that is threadsafe.
// It assumes the data encodes units in the following format:
//  1. Creator id, 2 bytes.
//  2. Signature, 64 bytes.
//  3. Parent heights, 4 bytes each.
//  4. Control hash 32 bytes.
//  5. Size of the unit data in bytes, 4 bytes.
//  6. The unit data, as much as declared in 5.
//  7. Size of the random source data in bytes, 4 bytes.
//  8. The random source data, as much as declared in 7.
// All integer values are encoded as 16 or 32 bit unsigned ints.
// It is guaranteed to read only as much data as needed.
func newDecoder(r io.Reader, nProc uint16) decoder {
	return &dec{r, nProc}
}

// decodeCrown reads encoded data from the io.Reader and tries to decode it as a crown.
func (d *dec) decodeCrown() (*gomel.Crown, error) {
	uint32Buf := make([]byte, 4)
	heights := make([]int, d.nProc)
	for i := range heights {
		_, err := io.ReadFull(d, uint32Buf)
		if err != nil {
			return nil, err
		}
		h := uint32(binary.LittleEndian.Uint32(uint32Buf))
		if h == math.MaxUint32 {
			heights[i] = -1
		} else {
			heights[i] = int(h)
		}
	}
	controlHash := &gomel.Hash{}
	_, err := io.ReadFull(d, controlHash[:])
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
