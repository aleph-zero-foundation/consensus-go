package encoding

import (
	"encoding/binary"
	"io"

	"gitlab.com/alephledger/consensus-go/pkg/creating"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type decoder struct {
	io.Reader
}

// NewDecoder creates a new encoding.Decoder that is threadsafe.
// It assumes the data encodes units in the following format:
//  1. Creator id, 2 bytes.
//  2. Signature, 64 bytes.
//  3. Number of parents, 2 bytes.
//  4. Parent hashes, as many as declared in 3., 32 bytes each.
//  5. Size of the unit data in bytes, 4 bytes.
//  6. The unit data, as much as declared in 5.
//  7. Size of the random source data in bytes, 4 bytes.
//  8. The random source data, as much as declared in 7.
// All integer values are encoded as 16 or 32 bit unsigned ints.
// It is guaranteed to read only as much data as needed.
func NewDecoder(r io.Reader) Decoder {
	return &decoder{r}
}

// DecodePreunit reads encoded data from the io.Reader and tries to decode it
// as a preunit.
func (d *decoder) DecodePreunit() (gomel.Preunit, error) {
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
	_, err = io.ReadFull(d, uint16Buf)
	if err != nil {
		return nil, err
	}
	nParents := binary.LittleEndian.Uint16(uint16Buf)
	parents := make([]*gomel.Hash, nParents)
	for i := range parents {
		parents[i] = &gomel.Hash{}
		_, err = io.ReadFull(d, parents[i][:])
		if err != nil {
			return nil, err
		}
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

	result := creating.NewPreunit(creator, parents, unitData, rsData)
	result.SetSignature(signature)
	return result, nil
}

func (d *decoder) decodeAntichain() ([]gomel.Preunit, error) {
	k, err := d.decodeUint32()
	if err != nil {
		return nil, err
	}
	result := make([]gomel.Preunit, k)
	for i := range result {
		result[i], err = d.DecodePreunit()
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (d *decoder) DecodePreunits() ([][]gomel.Preunit, int, error) {
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

func (d *decoder) decodeUint32() (uint32, error) {
	buf := make([]byte, 4)
	_, err := io.ReadFull(d, buf)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(buf), nil
}
