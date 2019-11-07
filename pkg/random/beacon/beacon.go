// Package beacon implements the beacon random source.
// This source is meant to be used in the setup stage only.
// After the setup phase is done, it shall return a coin random source for use in the main part of the protocol.
//
// Beacon assumes the following about the dag:
//  (1) there are no forks,
//  (2) level = height for each unit,
package beacon

import (
	"errors"

	"gitlab.com/alephledger/consensus-go/pkg/crypto/bn256"
	"gitlab.com/alephledger/consensus-go/pkg/crypto/encrypt"
	"gitlab.com/alephledger/consensus-go/pkg/crypto/p2p"
	"gitlab.com/alephledger/consensus-go/pkg/crypto/tcoin"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/random"
	"gitlab.com/alephledger/consensus-go/pkg/random/coin"
)

const (
	dealingLevel   = 0
	votingLevel    = 3
	multicoinLevel = 6
	sharesLevel    = 8
)

// Beacon is a struct representing the beacon random source.
type Beacon struct {
	pid        uint16
	dag        gomel.Dag
	multicoins []*tcoin.ThresholdCoin
	tcoins     []*tcoin.ThresholdCoin
	// vote[i][j] is the vote of i-th process on the j-th tcoin
	// it is nil when j-th dealing unit is not below
	// the unit of i-th process on votingLevel
	votes [][]*vote
	// shareProviders[i] is the set of the processes which have a unit on
	// votingLevel below the unit created by the i-th process on multicoinLevel
	shareProviders []map[uint16]bool
	// subcoins[i] is the set of coins which forms the i-th multicoin
	subcoins []map[uint16]bool
	// shares[i] is a map of the form
	// hash of a unit => the share for the i-th tcoin contained in the unit
	shares       []*random.SyncCSMap
	polyVerifier bn256.PolyVerifier
	pKeys        []*p2p.PublicKey
	sKey         *p2p.SecretKey
	p2pKeys      []encrypt.SymmetricKey
}

// vote is a vote for a tcoin
// proof = nil means that the tcoin is correct
// proof != nil means gives proof that the tcoin is incorrect
type vote struct {
	proof *p2p.SharedSecret
}

func (v *vote) isCorrect() bool {
	return v.proof == nil
}

// New returns a RandomSource using a beacon.
func New(pid uint16, pKeys []*p2p.PublicKey, sKey *p2p.SecretKey) (*Beacon, error) {
	p2pKeys, err := p2p.Keys(sKey, pKeys, pid)
	if err != nil {
		return nil, err
	}

	return &Beacon{
		pid:     pid,
		pKeys:   pKeys,
		sKey:    sKey,
		p2pKeys: p2pKeys,
	}, nil
}

// Bind the beacon with the given dag.
func (b *Beacon) Bind(dag gomel.Dag) {
	n := dag.NProc()
	b.dag = dag
	b.multicoins = make([]*tcoin.ThresholdCoin, n)
	b.votes = make([][]*vote, n)
	b.shareProviders = make([]map[uint16]bool, n)
	b.subcoins = make([]map[uint16]bool, n)
	b.tcoins = make([]*tcoin.ThresholdCoin, n)
	b.shares = make([]*random.SyncCSMap, n)
	b.polyVerifier = bn256.NewPolyVerifier(int(n), int(gomel.MinimalTrusted(n)))

	for i := uint16(0); i < dag.NProc(); i++ {
		b.votes[i] = make([]*vote, dag.NProc())
		b.shares[i] = random.NewSyncCSMap()
		b.subcoins[i] = make(map[uint16]bool)
	}
	dag.AddCheck(b.checkCompliance)
	dag.BeforeInsert(b.update)
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

	shares := []*tcoin.CoinShare{}
	units := unitsOnLevel(b.dag, level)
	for _, u := range units {
		if b.shareProviders[pid][u.Creator()] {
			uShares := []*tcoin.CoinShare{}
			for sc := range b.subcoins[pid] {
				uShares = append(uShares, b.shares[sc].Get(u.Hash()))
			}
			shares = append(shares, tcoin.SumShares(uShares))
		}
	}
	coin, ok := b.multicoins[pid].CombineCoinShares(shares)
	if !ok {
		// Not enough shares
		return nil
	}
	return coin.RandomBytes()
}

func (b *Beacon) checkCompliance(u gomel.Unit) error {
	if u.Level() == dealingLevel {
		tcEncoded := u.RandomSourceData()
		tc, _, err := tcoin.Decode(tcEncoded, u.Creator(), b.pid, b.p2pKeys[u.Creator()])
		if err != nil {
			return err
		}
		if !tc.PolyVerify(b.polyVerifier) {
			return errors.New("Tcoin does not come from a polynomial sequence")
		}
		return nil
	}
	if u.Level() == votingLevel {
		votes, err := unmarshallVotes(u.RandomSourceData(), b.dag.NProc())
		if err != nil {
			return err
		}

		err = validateVotes(b, u, votes)
		if err != nil {
			return err
		}
		return nil
	}
	if u.Level() >= sharesLevel {
		shares, err := unmarshallShares(u.RandomSourceData(), b.dag.NProc())
		if err != nil {
			return err
		}
		for pid := uint16(0); pid < b.dag.NProc(); pid++ {
			if b.votes[u.Creator()][pid] != nil && b.votes[u.Creator()][pid].isCorrect() {
				if shares[pid] == nil {
					return errors.New("missing share")
				}
				// This verification is slow
				if !b.tcoins[pid].VerifyCoinShare(shares[pid], u.Level()) {
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
		tc, okSecretKey, _ := tcoin.Decode(tcEncoded, u.Creator(), b.pid, b.p2pKeys[u.Creator()])
		if !okSecretKey {
			secret := p2p.NewSharedSecret(b.sKey, b.pKeys[u.Creator()])
			b.votes[b.pid][u.Creator()] = &vote{
				proof: &secret,
			}
		} else {
			b.votes[b.pid][u.Creator()] = &vote{}
		}
		b.tcoins[u.Creator()] = tc
	}
	if u.Level() == votingLevel {
		votes, _ := unmarshallVotes(u.RandomSourceData(), b.dag.NProc())
		for pid, vote := range votes {
			b.votes[u.Creator()][pid] = vote
		}
	}
	if u.Level() == multicoinLevel {
		coinsApprovedBy := make([]uint16, b.dag.NProc())
		nBelowUOnVotingLevel := uint16(0)
		providers := make(map[uint16]bool)
		votingUnits := unitsOnLevel(b.dag, votingLevel)

		for _, v := range votingUnits {
			if gomel.Above(u, v) {
				providers[v.Creator()] = true
				nBelowUOnVotingLevel++
				for pid := uint16(0); pid < b.dag.NProc(); pid++ {
					if b.votes[v.Creator()][pid] != nil && b.votes[v.Creator()][pid].isCorrect() {
						coinsApprovedBy[pid]++
					}
				}
			}
		}
		coinsToMerge := []*tcoin.ThresholdCoin{}
		for pid := uint16(0); pid < b.dag.NProc(); pid++ {
			if coinsApprovedBy[pid] == nBelowUOnVotingLevel {
				coinsToMerge = append(coinsToMerge, b.tcoins[pid])
				b.subcoins[u.Creator()][pid] = true
			}
		}
		b.multicoins[u.Creator()] = tcoin.CreateMulticoin(coinsToMerge)
		b.shareProviders[u.Creator()] = providers
	}
	if u.Level() >= sharesLevel {
		shares, _ := unmarshallShares(u.RandomSourceData(), b.dag.NProc())
		for pid := range shares {
			if shares[pid] != nil {
				b.shares[pid].Add(u.Hash(), shares[pid])
			}
		}
	}
}

func validateVotes(b *Beacon, u gomel.Unit, votes []*vote) error {
	dealingUnits := unitsOnLevel(b.dag, dealingLevel)
	createdDealing := make([]bool, b.dag.NProc())
	for _, v := range dealingUnits {
		shouldVote := gomel.Above(u, v)
		if shouldVote && votes[v.Creator()] == nil {
			return errors.New("missing vote")
		}
		if !shouldVote && votes[v.Creator()] != nil {
			return errors.New("vote on dealing unit not below the unit")
		}
		if shouldVote && !votes[v.Creator()].isCorrect() {
			if !verifyWrongSecretKeyProof(b, u.Creator(), v.Creator(), *votes[v.Creator()].proof) {
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

func verifyWrongSecretKeyProof(b *Beacon, prover, suspect uint16, proof p2p.SharedSecret) bool {
	if !p2p.VerifySharedSecret(b.pKeys[prover], b.pKeys[suspect], proof) {
		return false
	}
	key, err := p2p.Key(proof)
	if err != nil {
		return false
	}
	return !b.tcoins[suspect].CheckSecretKey(prover, key)
}

// DataToInclude returns data which should be included in a unit
// with the given creator and set of parents.
func (b *Beacon) DataToInclude(creator uint16, parents []gomel.Unit, level int) ([]byte, error) {
	if level == dealingLevel {
		nProc := b.dag.NProc()
		gtc := tcoin.NewRandomGlobal(nProc, gomel.MinimalTrusted(nProc))
		tc, err := gtc.Encrypt(b.p2pKeys)
		if err != nil {
			return nil, err
		}
		return tc.Encode(), nil
	}
	if level == votingLevel {
		return marshallVotes(b.votes[b.pid]), nil
	}
	if level >= sharesLevel {
		cses := make([]*tcoin.CoinShare, b.dag.NProc())
		for pid := uint16(0); pid < b.dag.NProc(); pid++ {
			if b.votes[creator][pid] != nil && b.votes[creator][pid].isCorrect() {
				cses[pid] = b.tcoins[pid].CreateCoinShare(level)
			}
		}
		return marshallShares(cses), nil
	}
	return []byte{}, nil
}

// GetCoin returns a coin random source obtained by using this beacon.
// Head should be the creator of a timing unit chosen on the 6th level.
func (b *Beacon) GetCoin(head uint16) gomel.RandomSource {
	return coin.New(b.dag.NProc(), b.pid, b.multicoins[head], b.shareProviders[head])
}

// unitsOnLevel returns all the prime units in the dag on a given level
func unitsOnLevel(dag gomel.Dag, level int) []gomel.Unit {
	result := []gomel.Unit{}
	su := dag.PrimeUnits(level)
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
