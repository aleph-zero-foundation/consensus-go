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
	getUnit() gomel.Preunit
	checkProof(fp *forkingProof) error
	rmcID() uint64
	setParentHashes(ph []byte)
	getParentHash(pid uint16) *gomel.Hash
	Marshal() []byte
}

type baseCommitment struct {
	sync.RWMutex
	pu           gomel.Preunit
	id           uint64
	encoded      []byte
	parentHashes []byte
}

func newBaseCommitment(pu gomel.Preunit, puEncoded []byte, rmcID uint64) commitment {
	comm := &baseCommitment{
		id: rmcID,
	}
	comm.encoded = make([]byte, 8)
	binary.LittleEndian.PutUint64(comm.encoded, comm.id)
	comm.encoded = append(comm.encoded, puEncoded...)
	comm.pu = pu
	return comm
}

func (comm *baseCommitment) Marshal() []byte {
	return comm.encoded
}

func (comm *baseCommitment) rmcID() uint64 {
	return comm.id
}

func (comm *baseCommitment) getUnit() gomel.Preunit {
	return comm.pu
}

func (comm *baseCommitment) checkProof(fp *forkingProof) error {
	if cu := comm.getUnit(); cu != nil {
		if *cu.Hash() != *fp.pcommit.Hash() {
			return errors.New("wrong proof for commit")
		}
		return nil
	}
	return errors.New("unitless commitment")
}

func (comm *baseCommitment) setParentHashes(ph []byte) {
	comm.Lock()
	defer comm.Unlock()
	comm.parentHashes = ph
}

func (comm *baseCommitment) getParentHash(pid uint16) *gomel.Hash {
	comm.RLock()
	defer comm.RUnlock()
	result := &gomel.Hash{}
	i := int(pid) * len(result)
	if i >= len(comm.parentHashes) {
		return nil
	}
	copy(result[:], comm.parentHashes[i:])
	return result
}

type inferredCommitment struct {
	sync.RWMutex
	pu              gomel.Preunit
	childCommitment commitment
	encoded         []byte
	parentHashes    []byte
}

func (comm *inferredCommitment) Marshal() []byte {
	return comm.encoded
}

func (comm *inferredCommitment) rmcID() uint64 {
	return comm.childCommitment.rmcID()
}

func (comm *inferredCommitment) getUnit() gomel.Preunit {
	return comm.pu
}

func (comm *inferredCommitment) checkProof(fp *forkingProof) error {
	return comm.childCommitment.checkProof(fp)
}

func (comm *inferredCommitment) setParentHashes(ph []byte) {
	comm.Lock()
	defer comm.Unlock()
	comm.parentHashes = ph
}

func (comm *inferredCommitment) getParentHash(pid uint16) *gomel.Hash {
	comm.RLock()
	defer comm.RUnlock()
	result := &gomel.Hash{}
	i := int(pid) * len(result)
	if i >= len(comm.parentHashes) {
		return nil
	}
	copy(result[:], comm.parentHashes[i:])
	return result
}

func commitmentForPreparent(comm commitment, pu gomel.Preunit, hashes []*gomel.Hash, encoded []byte) (commitment, error) {
	cu := comm.getUnit()
	if cu == nil {
		return nil, errors.New("empty commitment cannot justify parents")
	}
	pid := cu.Creator()
	if pid != pu.Creator() {
		return nil, errors.New("cannot justify unit created by a different process")
	}
	if *hashes[pid] != *pu.Hash() {
		return nil, errors.New("cannot justify unit with a mismatched hash")
	}
	if cu.View().ControlHash != *gomel.CombineHashes(hashes) {
		return nil, errors.New("control hash does not match hashes of parents")
	}
	parEncoded := []byte{}
	for _, h := range hashes {
		if h != nil {
			parEncoded = append(parEncoded, h[:]...)
		} else {
			parEncoded = append(parEncoded, gomel.ZeroHash[:]...)
		}
	}
	comm.setParentHashes(parEncoded)
	return &inferredCommitment{
		pu:              pu,
		childCommitment: comm,
		encoded:         encoded,
	}, nil
}

func commitmentForParent(comm commitment, u gomel.Unit) (commitment, error) {
	cu := comm.getUnit()
	if cu == nil {
		return nil, errors.New("empty commitment cannot justify parents")
	}
	if u == nil || *cu.Hash() != *u.Hash() {
		return nil, errors.New("incorrect commitment unit supplied")
	}
	pred := gomel.Predecessor(u)
	encoded := comm.Marshal()
	predEncoded, _ := encoding.EncodeUnit(pred)
	encoded = append(encoded, predEncoded...)
	parEncoded := []byte{}
	for _, par := range u.Parents() {
		if par != nil {
			parEncoded = append(parEncoded, par.Hash()[:]...)
		} else {
			parEncoded = append(parEncoded, gomel.ZeroHash[:]...)
		}
	}
	encoded = append(encoded, parEncoded...)
	pu, _ := encoding.DecodePreunit(predEncoded)
	comm.setParentHashes(parEncoded)
	return &inferredCommitment{
		pu:              pu,
		childCommitment: comm,
		encoded:         encoded,
	}, nil
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
	cu := c.getUnit()
	cb.Lock()
	defer cb.Unlock()
	if cu != nil {
		h := cu.Hash()
		if cb.toUnit[*h] == nil || cb.toUnit[*h].getParentHash(0) == nil {
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
		hashes := []*gomel.Hash{}
		for range pu.View().Heights {
			h := &gomel.Hash{}
			_, err := io.ReadFull(mr, h[:])
			if err != nil {
				return nil, err
			}
			hashes = append(hashes, h)
		}
		comm, err = commitmentForPreparent(comm, pu, hashes, mr.getMemory())
		if err != nil {
			return nil, err
		}
		result = append(result, comm)
		pu, err = encoding.ReceivePreunit(mr)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}
