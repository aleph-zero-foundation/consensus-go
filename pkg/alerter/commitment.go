package alerter

import (
	"encoding/binary"
	"errors"
	"io"
	"sync"

	"gitlab.com/alephledger/consensus-go/pkg/encoding"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type commitment interface {
	setUnit(u gomel.Unit)
	getUnit() gomel.Unit
	getHash() *gomel.Hash
	commitmentForParent(u gomel.Unit) commitment
	commitmentForPreparent(pu gomel.Preunit, encoded []byte) commitment
	checkProof(fp *forkingProof) error
	rmcID() uint64
	Marshal() []byte
}

type baseCommitment struct {
	sync.RWMutex
	unit    gomel.Unit
	id      uint64
	pu      gomel.Preunit
	encoded []byte
}

func (comm *baseCommitment) Marshal() []byte {
	if comm.encoded == nil {
		comm.encoded = make([]byte, 8)
		binary.LittleEndian.PutUint64(comm.encoded, comm.id)
		unitEncoded, _ := encoding.EncodeUnit(comm.unit)
		comm.encoded = append(comm.encoded, unitEncoded...)
	}
	return comm.encoded
}

func (comm *baseCommitment) rmcID() uint64 {
	return comm.id
}

func (comm *baseCommitment) setUnit(u gomel.Unit) {
	comm.Lock()
	defer comm.Unlock()
	if comm.unit == nil {
		comm.unit = u
	}
}

func (comm *baseCommitment) getUnit() gomel.Unit {
	comm.RLock()
	defer comm.RUnlock()
	return comm.unit
}

func (comm *baseCommitment) getHash() *gomel.Hash {
	if comm.pu != nil {
		return comm.pu.Hash()
	}
	if u := comm.getUnit(); u != nil {
		return u.Hash()
	}
	return nil
}

func (comm *baseCommitment) checkProof(fp *forkingProof) error {
	if h := comm.getHash(); h != nil {
		if *h != *fp.pcommit.Hash() {
			return errors.New("wrong proof for commit")
		}
		return nil
	}
	return errors.New("unitless commitment")
}

func (comm *baseCommitment) commitmentForParent(u gomel.Unit) commitment {
	comm.RLock()
	defer comm.RUnlock()
	if comm.unit == nil {
		// Cannot create commitments for parents of units we don't yet have.
		return nil
	}
	if predecessor := gomel.Predecessor(comm.unit); predecessor == nil || *u.Hash() != *predecessor.Hash() {
		return nil
	}
	return &inferredCommitment{
		unit:            u,
		childCommitment: comm,
	}
}

func (comm *baseCommitment) commitmentForPreparent(pu gomel.Preunit, encoded []byte) commitment {
	if comm.pu == nil {
		return nil
	}
	parents := comm.pu.Parents()
	if len(parents) < 1 || *pu.Hash() != *parents[0] {
		return nil
	}
	return &inferredCommitment{
		pu:              pu,
		childCommitment: comm,
		encoded:         encoded,
	}
}

type inferredCommitment struct {
	unit            gomel.Unit
	childCommitment commitment
	pu              gomel.Preunit
	encoded         []byte
}

func (comm *inferredCommitment) Marshal() []byte {
	if comm.encoded == nil {
		comm.encoded = comm.childCommitment.Marshal()
		unitEncoded, _ := encoding.EncodeUnit(comm.unit)
		comm.encoded = append(comm.encoded, unitEncoded...)
	}
	return comm.encoded
}

func (comm *inferredCommitment) rmcID() uint64 {
	return comm.childCommitment.rmcID()
}

func (comm *inferredCommitment) setUnit(gomel.Unit) {}

func (comm *inferredCommitment) getUnit() gomel.Unit {
	return comm.unit
}

func (comm *inferredCommitment) getHash() *gomel.Hash {
	if comm.pu != nil {
		return comm.pu.Hash()
	}
	if u := comm.getUnit(); u != nil {
		return u.Hash()
	}
	return nil
}

func (comm *inferredCommitment) checkProof(fp *forkingProof) error {
	return comm.childCommitment.checkProof(fp)
}

func (comm *inferredCommitment) commitmentForParent(u gomel.Unit) commitment {
	if comm.unit == nil {
		return nil
	}
	if predecessor := gomel.Predecessor(comm.unit); predecessor == nil || *u.Hash() != *predecessor.Hash() {
		return nil
	}
	return &inferredCommitment{
		unit:            u,
		childCommitment: comm,
	}
}

func (comm *inferredCommitment) commitmentForPreparent(pu gomel.Preunit, encoded []byte) commitment {
	if comm.pu == nil {
		return nil
	}
	parents := comm.pu.Parents()
	if len(parents) < 1 || *pu.Hash() != *parents[0] {
		return nil
	}
	return &inferredCommitment{
		pu:              pu,
		childCommitment: comm,
		encoded:         encoded,
	}
}

type commitBase struct {
	sync.RWMutex
	toUnit   map[gomel.Hash]commitment
	byMember map[uint16]map[uint16]commitment
}

func newCommitBase() *commitBase {
	return &commitBase{
		toUnit:   map[gomel.Hash]commitment{},
		byMember: map[uint16]map[uint16]commitment{},
	}
}

func (cb *commitBase) add(c commitment, commiter, forker uint16) {
	h := c.getHash()
	cb.Lock()
	defer cb.Unlock()
	if h != nil {
		if cb.toUnit[*h] == nil {
			cb.toUnit[*h] = c
		}
	}
	if cb.byMember[forker] == nil {
		cb.byMember[forker] = map[uint16]commitment{}
	}
	if cb.byMember[forker][commiter] == nil {
		cb.byMember[forker][commiter] = c
	}
}

func (cb *commitBase) getByHash(h *gomel.Hash) commitment {
	cb.RLock()
	defer cb.RUnlock()
	return cb.toUnit[*h]
}

func (cb *commitBase) getByParties(commiter, forker uint16) commitment {
	cb.RLock()
	defer cb.RUnlock()
	if cb.byMember[forker] == nil {
		return nil
	}
	return cb.byMember[forker][commiter]
}

func (cb *commitBase) isForker(forker uint16) bool {
	cb.RLock()
	defer cb.RUnlock()
	return cb.byMember[forker] != nil
}

func (cb *commitBase) addBatch(comms []commitment, proof *forkingProof, commiter uint16) error {
	if err := comms[0].checkProof(proof); err != nil {
		return err
	}
	forker := proof.pcommit.Creator()
	for _, c := range comms {
		cb.add(c, commiter, forker)
	}
	return nil
}

type memorizingReader struct {
	io.Reader
	memory []byte
}

func (r *memorizingReader) Read(b []byte) (int, error) {
	n, err := r.Reader.Read(b)
	if err == nil {
		r.memory = append(r.memory, b...)
	}
	return n, err
}

func (r *memorizingReader) getMemory() []byte {
	return r.memory
}

func acquireCommitments(r io.Reader) ([]commitment, error) {
	mr := &memorizingReader{
		Reader: r,
	}
	buf := make([]byte, 8)
	_, err := io.ReadFull(mr, buf)
	if err != nil {
		return nil, err
	}
	rmcID := binary.LittleEndian.Uint64(buf)
	pu, err := encoding.ReceivePreunit(mr)
	var comm commitment = &baseCommitment{
		id:      rmcID,
		pu:      pu,
		encoded: mr.getMemory(),
	}
	result := []commitment{comm}
	pu, err = encoding.ReceivePreunit(mr)
	if err != nil {
		return nil, err
	}
	for pu != nil {
		comm = comm.commitmentForPreparent(pu, mr.getMemory())
		result = append(result, comm)
		pu, err = encoding.ReceivePreunit(mr)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}
