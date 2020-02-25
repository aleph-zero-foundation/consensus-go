// Package beacon implements the beacon random source.
// This source is meant to be used in the setup stage only.
// After the setup phase is done, it shall return a coin random source for use in the main part of the protocol.
//
// Beacon assumes the following about the dag:
//  (1) there are no forks,
//  (2) level = height for each unit,
package beacon

import (
	"encoding/binary"
	"errors"

	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/random"
	"gitlab.com/alephledger/core-go/pkg/crypto/bn256"
	"gitlab.com/alephledger/core-go/pkg/crypto/encrypt"
	"gitlab.com/alephledger/core-go/pkg/crypto/p2p"
	"gitlab.com/alephledger/core-go/pkg/crypto/tss"
)

const (
	dealingLevel  = 0
	votingLevel   = 3
	multikeyLevel = 6
	sharesLevel   = 8
)

func nonce(level int) []byte {
	data := make([]byte, 2)
	binary.LittleEndian.PutUint16(data, uint16(level))
	return data
}

// Beacon is a struct representing the beacon random source.
type Beacon struct {
	pid  uint16
	conf config.Config
	dag  gomel.Dag
	wtk  []*tss.WeakThresholdKey
	tks  []*tss.ThresholdKey
	// vote[i][j] is the vote of i-th process on the j-th tss
	// it is nil when j-th dealing unit is not below
	// the unit of i-th process on votingLevel
	votes [][]*vote
	// shareProviders[i] is the set of the processes which have a unit on
	// votingLevel below the unit created by the i-th process on multikeyLevel
	shareProviders []map[uint16]bool
	// subcoins[i] is the set of coins which forms the i-th multicoin
	subcoins []map[uint16]bool
	// shares[i] is a map of the form
	// hash of a unit => the share for the i-th tss contained in the unit
	shares       []*random.SyncCSMap
	polyVerifier bn256.PolyVerifier
	p2pKeys      []encrypt.SymmetricKey
}

// vote is a vote for a tss
// proof = nil means that the tss is correct
// proof != nil means gives proof that the tss is incorrect
type vote struct {
	proof *p2p.SharedSecret
}

func (v *vote) isCorrect() bool {
	return v.proof == nil
}

// New returns a RandomSource using a beacon.
func New(conf config.Config) (*Beacon, error) {
	p2pKeys, err := p2p.Keys(conf.P2PSecretKey, conf.P2PPublicKeys, conf.Pid)
	if err != nil {
		return nil, err
	}
	b := &Beacon{
		pid:            conf.Pid,
		conf:           conf,
		wtk:            make([]*tss.WeakThresholdKey, conf.NProc),
		tks:            make([]*tss.ThresholdKey, conf.NProc),
		votes:          make([][]*vote, conf.NProc),
		shareProviders: make([]map[uint16]bool, conf.NProc),
		subcoins:       make([]map[uint16]bool, conf.NProc),
		shares:         make([]*random.SyncCSMap, conf.NProc),
		polyVerifier:   bn256.NewPolyVerifier(int(conf.NProc), int(gomel.MinimalTrusted(conf.NProc))),
		p2pKeys:        p2pKeys,
	}
	for i := 0; i < int(conf.NProc); i++ {
		b.votes[i] = make([]*vote, conf.NProc)
		b.shares[i] = random.NewSyncCSMap()
		b.subcoins[i] = make(map[uint16]bool)
	}
	return b, nil
}

// NewRandomSource allows using this instance of Beacon as a RandomSource by binding the provided dag to it.
func (b *Beacon) NewRandomSource(dag gomel.Dag) gomel.RandomSource {
	dag.AddCheck(b.checkCompliance)
	dag.BeforeInsert(b.update)
	b.dag = dag
	return b
}

// RandomBytes returns a sequence of random bits for a given unit.
// It returns nil when
// (1) asked on a level that is too low i.e. level < sharesLevel
// or
// (2) there are no enough shares, i.e.
// the number of units on a given level created by share providers
// to the multicoin of pid is less than f+1.
//
// When there is at least one unit of level+1 in the dag
// then condition (2) cannot hold.
func (b *Beacon) RandomBytes(pid uint16, level int) []byte {
	if level < sharesLevel {
		// RandomBytes asked on too low level
		return nil
	}
	shares := []*tss.Share{}
	units := unitsOnLevel(b.dag, level)
	for _, u := range units {
		if b.shareProviders[pid][u.Creator()] {
			uShares := []*tss.Share{}
			for sc := range b.subcoins[pid] {
				uShares = append(uShares, b.shares[sc].Get(u.Hash()))
			}
			shares = append(shares, tss.SumShares(uShares))
		}
	}
	coin, ok := b.wtk[pid].CombineShares(shares)
	if !ok {
		// Not enough shares
		return nil
	}
	return coin.Marshal()
}

func (b *Beacon) checkCompliance(u gomel.Unit, _ gomel.Dag) error {
	if u.Level() == dealingLevel {
		tcEncoded := u.RandomSourceData()
		tc, _, err := tss.Decode(tcEncoded, u.Creator(), b.pid, b.p2pKeys[u.Creator()])
		if err != nil {
			return err
		}
		if !tc.PolyVerify(b.polyVerifier) {
			return errors.New("Tcoin does not come from a polynomial sequence")
		}
		return nil
	}
	if u.Level() == votingLevel {
		votes, err := unmarshallVotes(u.RandomSourceData(), b.conf.NProc)
		if err != nil {
			return err
		}

		err = b.validateVotes(u, votes)
		if err != nil {
			return err
		}
		return nil
	}
	if u.Level() >= sharesLevel {
		shares, err := unmarshallShares(u.RandomSourceData(), b.conf.NProc)
		if err != nil {
			return err
		}
		for pid := uint16(0); pid < b.conf.NProc; pid++ {
			if b.votes[u.Creator()][pid] != nil && b.votes[u.Creator()][pid].isCorrect() {
				if shares[pid] == nil {
					return errors.New("missing share")
				}
				// This verification is slow
				if !b.tks[pid].VerifyShare(shares[pid], nonce(u.Level())) {
					return errors.New("invalid share")
				}
			}
		}
		return nil
	}
	return nil
}

func (b *Beacon) update(u gomel.Unit) {
	if u.Level() == dealingLevel {
		tcEncoded := u.RandomSourceData()
		tc, okSecretKey, _ := tss.Decode(tcEncoded, u.Creator(), b.pid, b.p2pKeys[u.Creator()])
		if !okSecretKey {
			secret := p2p.NewSharedSecret(b.conf.P2PSecretKey, b.conf.P2PPublicKeys[u.Creator()])
			b.votes[b.pid][u.Creator()] = &vote{
				proof: &secret,
			}
		} else {
			b.votes[b.pid][u.Creator()] = &vote{}
		}
		b.tks[u.Creator()] = tc
	}
	if u.Level() == votingLevel {
		votes, _ := unmarshallVotes(u.RandomSourceData(), b.conf.NProc)
		for pid, vote := range votes {
			b.votes[u.Creator()][pid] = vote
		}
	}
	if u.Level() == multikeyLevel {
		coinsApprovedBy := make([]uint16, b.conf.NProc)
		nBelowUOnVotingLevel := uint16(0)
		providers := make(map[uint16]bool)
		votingUnits := unitsOnLevel(b.dag, votingLevel)

		for _, v := range votingUnits {
			if gomel.Above(u, v) {
				providers[v.Creator()] = true
				nBelowUOnVotingLevel++
				for pid := uint16(0); pid < b.conf.NProc; pid++ {
					if b.votes[v.Creator()][pid] != nil && b.votes[v.Creator()][pid].isCorrect() {
						coinsApprovedBy[pid]++
					}
				}
			}
		}
		coinsToMerge := []*tss.ThresholdKey{}
		for pid := uint16(0); pid < b.conf.NProc; pid++ {
			if coinsApprovedBy[pid] == nBelowUOnVotingLevel {
				coinsToMerge = append(coinsToMerge, b.tks[pid])
				b.subcoins[u.Creator()][pid] = true
			}
		}
		b.wtk[u.Creator()] = tss.CreateWTK(coinsToMerge, providers)
		b.shareProviders[u.Creator()] = providers
	}
	if u.Level() >= sharesLevel {
		shares, _ := unmarshallShares(u.RandomSourceData(), b.conf.NProc)
		for pid := range shares {
			if shares[pid] != nil {
				b.shares[pid].Add(u.Hash(), shares[pid])
			}
		}
	}
}

func (b *Beacon) validateVotes(u gomel.Unit, votes []*vote) error {
	dealingUnits := unitsOnLevel(b.dag, dealingLevel)
	createdDealing := make([]bool, b.conf.NProc)
	for _, v := range dealingUnits {
		shouldVote := gomel.Above(u, v)
		if shouldVote && votes[v.Creator()] == nil {
			return errors.New("missing vote")
		}
		if !shouldVote && votes[v.Creator()] != nil {
			return errors.New("vote on dealing unit not below the unit")
		}
		if shouldVote && !votes[v.Creator()].isCorrect() {
			if !b.verifyWrongSecretKeyProof(u.Creator(), v.Creator(), *votes[v.Creator()].proof) {
				return errors.New("the provided proof is incorrect")
			}
		}
		createdDealing[v.Creator()] = true
	}
	for pid := range createdDealing {
		if votes[pid] != nil && !createdDealing[pid] {
			return errors.New("vote on non-existing dealing unit")
		}
	}
	return nil
}

func (b *Beacon) verifyWrongSecretKeyProof(prover, suspect uint16, proof p2p.SharedSecret) bool {
	if !p2p.VerifySharedSecret(b.conf.P2PPublicKeys[prover], b.conf.P2PPublicKeys[suspect], proof) {
		return false
	}
	key, err := p2p.Key(proof)
	if err != nil {
		return false
	}
	return !b.tks[suspect].CheckSecretKey(prover, key)
}

// DealingData returns random source data that should be included in the dealing unit.
func (b *Beacon) DealingData(epoch gomel.EpochID) ([]byte, error) {
	if epoch != 0 {
		return nil, errors.New("Beacon was asked for dealing data with non-zero epoch")
	}
	gtc := tss.NewRandom(b.conf.NProc, gomel.MinimalTrusted(b.conf.NProc))
	tc, err := gtc.Encrypt(b.p2pKeys)
	if err != nil {
		return nil, err
	}
	return tc.Encode(), nil
}

// DataToInclude returns data which should be included in a unit with given parents and level.
func (b *Beacon) DataToInclude(parents []gomel.Unit, level int) ([]byte, error) {
	if level == votingLevel {
		return marshallVotes(b.votes[b.pid]), nil
	}
	if level >= sharesLevel {
		shs := make([]*tss.Share, b.conf.NProc)
		for pid := uint16(0); pid < b.conf.NProc; pid++ {
			if b.votes[b.conf.Pid][pid] != nil && b.votes[b.conf.Pid][pid].isCorrect() {
				shs[pid] = b.tks[pid].CreateShare(nonce(level))
			}
		}
		return marshallShares(shs), nil
	}
	return []byte{}, nil
}

// GetWTK returns a weak threshold key obtained by using this beacon.
// Head should be the creator of the timing unit chosen on the 6th level.
func (b *Beacon) GetWTK(head uint16) *tss.WeakThresholdKey {
	return b.wtk[head]
}

// unitsOnLevel returns all the prime units in the dag on a given level
func unitsOnLevel(dag gomel.Dag, level int) []gomel.Unit {
	result := []gomel.Unit{}
	su := dag.UnitsOnLevel(level)
	if su != nil {
		su.Iterate(func(units []gomel.Unit) bool {
			if len(units) != 0 {
				result = append(result, units[0])
			}
			return true
		})
	}
	return result
}
