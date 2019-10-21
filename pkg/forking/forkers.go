package forking

import (
	"bytes"
	"errors"

	"gitlab.com/alephledger/consensus-go/pkg/encoding"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type forkingProof struct {
	pu, pv, pcommit gomel.Preunit
	encoded         []byte
}

func newForkingProof(u, max gomel.Unit) *forkingProof {
	v := max
	for v.Height() > u.Height() {
		v = gomel.Predecessor(v)
	}
	if *u.Hash() == *v.Hash() {
		return nil
	}
	ue, _ := encoding.EncodeUnit(u)
	ve, _ := encoding.EncodeUnit(v)
	comme, _ := encoding.EncodeUnit(max)
	encoded := append(ue, ve...)
	encoded = append(encoded, comme...)
	pu, _ := encoding.DecodePreunit(ue)
	pv, _ := encoding.DecodePreunit(ve)
	pcommit, _ := encoding.DecodePreunit(comme)
	return &forkingProof{
		pu:      pu,
		pv:      pv,
		pcommit: pcommit,
		encoded: encoded,
	}
}

func (fp *forkingProof) Marshal() []byte {
	return fp.encoded
}

func (fp *forkingProof) Unmarshal(data []byte) (*forkingProof, error) {
	reader := bytes.NewReader(data)
	var err error
	fp.pu, err = encoding.ReceivePreunit(reader)
	if err != nil {
		return nil, err
	}
	fp.pv, err = encoding.ReceivePreunit(reader)
	if err != nil {
		return nil, err
	}
	fp.pcommit, err = encoding.ReceivePreunit(reader)
	if err != nil {
		return nil, err
	}
	fp.encoded = append(make([]byte, 0, len(data)), data...)
	return fp, nil
}

func (fp *forkingProof) forkerID() uint16 {
	return fp.pu.Creator()
}

// splitEncoding returns the encoded proof in two parts, first the proof itself, then the commitment
func (fp *forkingProof) splitEncoding() ([]byte, []byte) {
	encoded := fp.Marshal()
	reader := bytes.NewReader(encoded)
	encoding.ReceivePreunit(reader)
	encoding.ReceivePreunit(reader)
	proofOnly := encoded[:len(encoded)-reader.Len()]
	commitOnly := encoded[len(encoded)-reader.Len():]
	return proofOnly, commitOnly
}

func (fp *forkingProof) replaceCommit(commit gomel.Unit) {
	proofOnly, _ := fp.splitEncoding()
	comme, _ := encoding.EncodeUnit(commit)
	fp.encoded = append([]byte{}, proofOnly...)
	fp.encoded = append(fp.encoded, comme...)
	fp.pcommit, _ = encoding.DecodePreunit(comme)
}

func (fp *forkingProof) checkCorrectness(expectedPid uint16, key gomel.PublicKey) error {
	if fp.pu == nil || fp.pv == nil {
		return errors.New("nil units do not prove forking")
	}
	if fp.pu.Creator() != expectedPid || fp.pv.Creator() != expectedPid || (fp.pcommit != nil && fp.pcommit.Creator() != expectedPid) {
		return errors.New("creator differs from expected")
	}
	if !key.Verify(fp.pu) || !key.Verify(fp.pv) || (fp.pcommit != nil && !key.Verify(fp.pcommit)) {
		return errors.New("improper signature")
	}
	if fp.pu.Height() != fp.pv.Height() {
		return errors.New("two units on different heights do not prove a fork")
	}
	if *fp.pu.Hash() == *fp.pv.Hash() {
		return errors.New("two copies of a unit are not a fork")
	}
	return nil
}

func (fp *forkingProof) extractCommitment(rmcID uint64) commitment {
	_, commitOnly := fp.splitEncoding()
	return newBaseCommitment(fp.pcommit, commitOnly, rmcID)
}
