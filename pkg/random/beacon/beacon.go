package beacon

import (
	"errors"
	"math/big"
	"sort"

	"gitlab.com/alephledger/consensus-go/pkg/crypto/tcoin"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/random"
	"golang.org/x/crypto/sha3"
)

// Beacon code assumes that:
// (1) there are no forks
// (2) every unit is a prime unit
// (3) level = height for each unit
// (4) each unit has at least 2f + 1 parents

const (
	dealingLevel   = 0
	votingLevel    = 3
	multicoinLevel = 6
	sharesLevel    = 8
)

type beacon struct {
	pid        int
	dag        gomel.Dag
	multicoins []*tcoin.ThresholdCoin
	tcoins     []*tcoin.ThresholdCoin
	// vote[i][j] is the vote of i-th process on the j-th tcoin
	// it is nil when j-th dealing unit is not below
	// the unit of i-th process on votingLevel
	votes [][]*vote
	// shareProviders[i] is the set of the processes which have a unit on
	// votingLevel below the unit created by the i-th process on multicoinLevel
	shareProviders []map[int]bool
	// subcoins[i] is the set of coins which forms the i-th multicoin
	subcoins []map[int]bool
	// shares[i] is a map of the form
	// hash of a unit => the share for the i-th tcoin contained in the unit
	shares       []*random.SyncCSMap
	polyVerifier tcoin.PolyVerifier
}

// vote is a vote for a tcoin
// proof = nil means that the tcoin is correct
// proof != nil means gives proof that the tcoin is incorrect
type vote struct {
	proof *big.Int
}

func (v *vote) isCorrect() bool {
	return v.proof == nil
}

// NewBeacon returns a RandomSource based on a beacon
// It is meant to be used in the setup stage only.
func NewBeacon(dag gomel.Dag, pid int) gomel.RandomSource {
	n := dag.NProc()
	b := &beacon{
		pid:            pid,
		dag:            dag,
		multicoins:     make([]*tcoin.ThresholdCoin, n),
		votes:          make([][]*vote, n),
		shareProviders: make([]map[int]bool, n),
		subcoins:       make([]map[int]bool, n),
		tcoins:         make([]*tcoin.ThresholdCoin, n),
		shares:         make([]*random.SyncCSMap, n),
		polyVerifier:   tcoin.NewPolyVerifier(n, n/3+1),
	}
	for i := 0; i < dag.NProc(); i++ {
		b.votes[i] = make([]*vote, dag.NProc())
		b.shares[i] = random.NewSyncCSMap()
		b.subcoins[i] = make(map[int]bool)
	}
	return b
}

// GetCRP returns a random permutation of processes on a given level.
// Proceses which haven't produce a unit on the requested level,
// form a sufix of the permutation.
// It returns nil when
// (1) there are no units on the given level
// or
// (2) the level is too low (i.e. (level+3) < sharesLevel)
// or
// (3) there are no enough shares on level+3 yet to generate the priority
// of some unit on a given level.
func (b *beacon) GetCRP(level int) []int {
	nProc := b.dag.NProc()
	permutation := make([]int, nProc)
	priority := make([][]byte, nProc)
	for i := 0; i < nProc; i++ {
		permutation[i] = i
	}

	units := unitsOnLevel(b.dag, level)
	if len(units) == 0 {
		return nil
	}

	for _, u := range units {
		priority[u.Creator()] = make([]byte, 32)

		rBytes := b.RandomBytes(u, level+3)
		if rBytes == nil {
			return nil
		}
		rBytes = append(rBytes, u.Hash()[:]...)
		sha3.ShakeSum128(priority[u.Creator()], rBytes)
	}

	sort.Slice(permutation, func(i, j int) bool {
		if priority[permutation[j]] == nil {
			return true
		}
		if priority[permutation[i]] == nil {
			return false
		}
		for x := 0; x < 32; x++ {
			if priority[permutation[i]][x] < priority[permutation[j]][x] {
				return true
			}
			if priority[permutation[i]][x] > priority[permutation[j]][x] {
				return false
			}
		}
		panic("hash collision")
		return (permutation[i] < permutation[j])
	})

	return permutation
}

// RandomBytes returns a sequence of random bits for a given unit.
// It returns nil when
// (1) asked on too low level i.e. level < sharesLevel
// or
// (2) there are no enough shares. i.e.
// The number of units on a given level created by share providers
// to the multicoin of uTossing.Creator() is less than f+1
//
// When there is at least one unit of level+1 in the dag
// then the (2) condition doesn't hold.
func (b *beacon) RandomBytes(uTossing gomel.Unit, level int) []byte {
	if level < sharesLevel {
		// RandomBytes asked on too low level
		return nil
	}

	mcID := uTossing.Creator()
	shares := []*tcoin.CoinShare{}
	units := unitsOnLevel(b.dag, level)
	for _, u := range units {
		if b.shareProviders[mcID][u.Creator()] {
			uShares := []*tcoin.CoinShare{}
			for sc := range b.subcoins[mcID] {
				uShares = append(uShares, b.shares[sc].Get(u.Hash()))
			}
			shares = append(shares, tcoin.SumShares(uShares))
		}
	}
	coin, ok := b.multicoins[mcID].CombineCoinShares(shares)
	if !ok {
		// Not enough shares
		return nil
	}
	return coin.RandomBytes()
}

func (b *beacon) CheckCompliance(u gomel.Unit) error {
	if u.Level() == dealingLevel {
		tcEncoded := u.RandomSourceData()
		tc, err := tcoin.Decode(tcEncoded, b.pid)
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
		for pid := 0; pid < b.dag.NProc(); pid++ {
			if b.votes[u.Creator()][pid].isCorrect() {
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

// Update updates the RandomSource with data included in the preunit
func (b *beacon) Update(u gomel.Unit) {
	if u.Level() == dealingLevel {
		tcEncoded := u.RandomSourceData()
		tc, _ := tcoin.Decode(tcEncoded, b.pid)
		b.tcoins[u.Creator()] = tc
	}
	if u.Level() == votingLevel {
		votes, _ := unmarshallVotes(u.RandomSourceData(), b.dag.NProc())
		for pid, vote := range votes {
			b.votes[u.Creator()][pid] = vote
		}
	}
	if u.Level() == multicoinLevel {
		coinsApprovedBy := make([]int, b.dag.NProc())
		nBelowUOnVotingLevel := 0
		providers := make(map[int]bool)
		votingUnits := unitsOnLevel(b.dag, votingLevel)

		for _, v := range votingUnits {
			if v.Below(u) {
				providers[v.Creator()] = true
				nBelowUOnVotingLevel++
				for pid := 0; pid < b.dag.NProc(); pid++ {
					if b.votes[v.Creator()][pid].isCorrect() {
						coinsApprovedBy[pid]++
					}
				}
			}
		}
		coinsToMerge := []*tcoin.ThresholdCoin{}
		for pid := 0; pid < b.dag.NProc(); pid++ {
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

func validateVotes(b *beacon, u gomel.Unit, votes []*vote) error {
	dealingUnits := unitsOnLevel(b.dag, dealingLevel)
	createdDealing := make([]bool, b.dag.NProc())
	for _, v := range dealingUnits {
		shouldVote := v.Below(u)
		if shouldVote && votes[v.Creator()] == nil {
			return errors.New("missing vote")
		}
		if !shouldVote && votes[v.Creator()] != nil {
			return errors.New("vote on dealing unit not below the unit")
		}
		if shouldVote && !votes[v.Creator()].isCorrect() {
			proof := votes[v.Creator()].proof
			if !b.tcoins[v.Creator()].VerifyWrongSecretKeyProof(u.Creator(), proof) {
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

func verifyTCoin(tc *tcoin.ThresholdCoin) *vote {
	return &vote{
		proof: tc.VerifySecretKey(),
	}
}

func (b *beacon) DataToInclude(creator int, parents []gomel.Unit, level int) []byte {
	if level == dealingLevel {
		nProc := b.dag.NProc()
		return tcoin.Deal(nProc, nProc/3+1)
	}
	if level == votingLevel {
		votes := make([]*vote, b.dag.NProc())
		dealingUnits := unitsOnLevel(b.dag, dealingLevel)
		for _, u := range dealingUnits {
			if gomel.BelowAny(u, parents) {
				votes[u.Creator()] = verifyTCoin(b.tcoins[u.Creator()])
			}
		}
		return marshallVotes(votes)
	}
	if level >= sharesLevel {
		cses := make([]*tcoin.CoinShare, b.dag.NProc())
		for pid := 0; pid < b.dag.NProc(); pid++ {
			if b.votes[creator][pid].isCorrect() {
				cses[pid] = b.tcoins[pid].CreateCoinShare(level)
			}
		}
		return marshallShares(cses)
	}
	return []byte{}
}

func unitsOnLevel(p gomel.Dag, level int) []gomel.Unit {
	result := []gomel.Unit{}
	su := p.PrimeUnits(level)
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

func level(pu gomel.Preunit, dag gomel.Dag) (int, error) {
	if len(pu.Parents()) == 0 {
		return 0, nil
	}
	predecessor := dag.Get(pu.Parents()[:1])
	if predecessor[0] == nil {
		return 0, errors.New("predecessor doesn't exist in the dag")
	}
	return predecessor[0].Level() + 1, nil
}
