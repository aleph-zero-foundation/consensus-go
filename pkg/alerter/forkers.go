package alerter

import (
	"bytes"
	"encoding/binary"
	"errors"

	"gitlab.com/alephledger/consensus-go/pkg/encoding"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type forkingProof struct {
	u, v, commit    gomel.Unit
	pu, pv, pcommit gomel.Preunit
	encoded         []byte
}

func newForkingProof(u, max gomel.Unit) *forkingProof {
	v := max
	for v.Height() > u.Height() {
		v = gomel.Predecessor(v)
	}
	uPred := gomel.Predecessor(u)
	vPred := gomel.Predecessor(v)
	for uPred != vPred {
		u = uPred
		v = vPred
		uPred = gomel.Predecessor(u)
		vPred = gomel.Predecessor(v)
	}
	if *u.Hash() == *v.Hash() {
		return nil
	}
	return &forkingProof{
		u:      u,
		v:      v,
		commit: max,
	}
}

func (fp *forkingProof) Marshal() []byte {
	if fp.encoded == nil {
		ue, _ := encoding.EncodeUnit(fp.u)
		ve, _ := encoding.EncodeUnit(fp.v)
		comme, _ := encoding.EncodeUnit(fp.commit)
		fp.encoded = append(ue, ve...)
		fp.encoded = append(fp.encoded, comme...)
	}
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
	if fp.u != nil {
		return fp.u.Creator()
	}
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
	fp.commit = commit
	fp.pcommit = nil
	if fp.encoded == nil {
		return
	}
	proofOnly, _ := fp.splitEncoding()
	comme, _ := encoding.EncodeUnit(fp.commit)
	fp.encoded = append([]byte{}, proofOnly...)
	fp.encoded = append(fp.encoded, comme...)
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
	if (fp.pu.Parents() == nil && fp.pv.Parents() != nil) || (fp.pv.Parents() == nil && fp.pu.Parents() != nil) {
		return errors.New("only one of the units has a predecessor")
	}
	if fp.pu.Parents() != nil && fp.pv.Parents() != nil && fp.pu.Parents()[expectedPid] != fp.pv.Parents()[expectedPid] {
		return errors.New("the units have different predecessors")
	}
	if *fp.pu.Hash() == *fp.pv.Hash() {
		return errors.New("two copies of a unit are not a fork")
	}
	return nil
}

func (fp *forkingProof) extractCommitment(rmcID uint64) commitment {
	if fp.u != nil {
		return &baseCommitment{
			id:   rmcID,
			unit: fp.commit,
			pu:   fp.pcommit,
		}
	}
	encoded := make([]byte, 8)
	binary.LittleEndian.PutUint64(encoded, rmcID)
	_, commitOnly := fp.splitEncoding()
	return &baseCommitment{
		id:      rmcID,
		pu:      fp.pcommit,
		unit:    fp.commit,
		encoded: append(encoded, commitOnly...),
	}
}
