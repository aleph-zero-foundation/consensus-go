package beacon

import (
	"errors"
	"math/big"
	"sort"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/crypto/tcoin"
	"gitlab.com/alephledger/consensus-go/pkg/random"
	"golang.org/x/crypto/sha3"
)

// Beacon code assumes that:
// (1) there are no forks
// (2) every unit is a prime unit
// (3) level = height for each unit

const (
	dealingLevel   = 0
	votingLevel    = 3
	multicoinLevel = 6
	sharesLevel    = 8
)

type beacon struct {
	pid        int
	poset      gomel.Poset
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
	return v.proof != nil
}

// NewBeacon returns a RandomSource based on a beacon
func NewBeacon(poset gomel.Poset, pid int) gomel.RandomSource {
	n := poset.NProc()
	b := &beacon{
		pid:            pid,
		poset:          poset,
		multicoins:     make([]*tcoin.ThresholdCoin, n),
		votes:          make([][]*vote, n),
		shareProviders: make([]map[int]bool, n),
		subcoins:       make([]map[int]bool, n),
		tcoins:         make([]*tcoin.ThresholdCoin, n),
		shares:         make([]*random.SyncCSMap, n),
		polyVerifier:   tcoin.NewPolyVerifier(n, n/3+1),
	}
	for i := 0; i < poset.NProc(); i++ {
		b.votes[i] = make([]*vote, poset.NProc())
		b.shares[i] = random.NewSyncCSMap()
		b.subcoins[i] = make(map[int]bool)
	}
	return b
}

// GetCRP returns a random permutation of processes on a given level.
// Proceses which haven't produce a unit on the requested level,
// form a sufix of the permutation.
func (b *beacon) GetCRP(level int) []int {
	nProc := b.poset.NProc()
	permutation := make([]int, nProc)
	priority := make([][]byte, nProc)
	for i := 0; i < nProc; i++ {
		permutation[i] = i
	}

	units := unitsOnLevel(b.poset, level)
	su := b.poset.PrimeUnits(level + 3)
	if su == nil {
		return nil
	}

	for _, u := range units {
		priority[u.Creator()] = make([]byte, 32)

		if len(su.Get(u.Creator())) == 0 {
			// unit on level + 3 has not been created yet
			return nil
		}

		rBytes := b.RandomBytes(su.Get(u.Creator())[0])
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
		return (permutation[i] < permutation[j])
	})

	return permutation
}

// RandomBytes returns a sequence of random bits for a given unit
// in the case of failure it returns nil.
// Becuase we are verifying coinShares before adding any unit to the poset,
// this function returns nil only when the tossing unit has too low level.
func (b *beacon) RandomBytes(uTossing gomel.Unit) []byte {
	if uTossing.Level() < sharesLevel {
		return nil
	}

	shares := []*tcoin.CoinShare{}
	for _, u := range uTossing.Parents() {
		if b.shareProviders[uTossing.Creator()][u.Creator()] {
			uShares := []*tcoin.CoinShare{}
			for sc := range b.subcoins[uTossing.Creator()] {
				uShares = append(uShares, b.shares[sc].Get(u.Hash()))
			}
			shares = append(shares, tcoin.SumShares(uShares))
		}
	}
	coin, ok := b.multicoins[uTossing.Creator()].CombineCoinShares(shares)
	if !ok {
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
		votes, err := unmarshallVotes(u.RandomSourceData(), b.poset.NProc())
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
		shares, err := unmarshallShares(u.RandomSourceData(), b.poset.NProc())
		if err != nil {
			return err
		}
		for pid := 0; pid < b.poset.NProc(); pid++ {
			if b.votes[u.Creator()][pid].isCorrect() {
				if shares[pid] == nil {
					return errors.New("Preunit without a share it should contain ")
				}
				// This verification is slow
				if !b.tcoins[pid].VerifyCoinShare(shares[pid], u.Level()) {
					return errors.New("Invalid coin share")
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
		votes, _ := unmarshallVotes(u.RandomSourceData(), b.poset.NProc())
		for pid, vote := range votes {
			b.votes[u.Creator()][pid] = vote
		}
	}
	if u.Level() == multicoinLevel {
		coinsApprovedBy := make([]int, b.poset.NProc())
		nBelowUOnVotingLevel := 0
		providers := make(map[int]bool)
		votingUnits := unitsOnLevel(b.poset, votingLevel)

		for _, v := range votingUnits {
			if v.Below(u) {
				providers[v.Creator()] = true
				nBelowUOnVotingLevel++
				for pid := 0; pid < b.poset.NProc(); pid++ {
					if b.votes[v.Creator()][pid].isCorrect() {
						coinsApprovedBy[pid]++
					}
				}
			}
		}
		coinsToMerge := []*tcoin.ThresholdCoin{}
		for pid := 0; pid < b.poset.NProc(); pid++ {
			if coinsApprovedBy[pid] == nBelowUOnVotingLevel {
				coinsToMerge = append(coinsToMerge, b.tcoins[pid])
				b.subcoins[u.Creator()][pid] = true
			}
		}
		b.multicoins[u.Creator()] = tcoin.CreateMulticoin(coinsToMerge)
		b.shareProviders[u.Creator()] = providers
	}
	if u.Level() >= sharesLevel {
		shares, _ := unmarshallShares(u.RandomSourceData(), b.poset.NProc())
		for pid := range shares {
			if shares[pid] != nil {
				b.shares[pid].Add(u.Hash(), shares[pid])
			}
		}
	}
}

func validateVotes(b *beacon, u gomel.Unit, votes []*vote) error {
	dealingUnits := unitsOnLevel(b.poset, dealingLevel)
	createdDealing := make([]bool, b.poset.NProc())
	for _, v := range dealingUnits {
		shouldVote := v.Below(u)
		if shouldVote && votes[v.Creator()] == nil {
			return errors.New("Missing vote")
		}
		if !shouldVote && votes[v.Creator()] != nil {
			return errors.New("Vote on dealing unit not below the unit")
		}
		if shouldVote && !votes[v.Creator()].isCorrect() {
			proof := votes[v.Creator()].proof
			if !b.tcoins[v.Creator()].VerifyWrongSecretKeyProof(u.Creator(), proof) {
				return errors.New("The provided proof is incorrect")
			}
		}
		createdDealing[v.Creator()] = true
	}
	for pid := range createdDealing {
		if votes[pid] != nil && !createdDealing[pid] {
			return errors.New("Vote on non-existing dealing unit")
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
		nProc := b.poset.NProc()
		return tcoin.Deal(nProc, nProc/3+1)
	}
	if level == votingLevel {
		votes := make([]*vote, b.poset.NProc())
		dealingUnits := unitsOnLevel(b.poset, dealingLevel)
		for _, u := range dealingUnits {
			if gomel.BelowAny(u, parents) {
				votes[u.Creator()] = verifyTCoin(b.tcoins[u.Creator()])
			}
		}
		return marshallVotes(votes)
	}
	if level >= sharesLevel {
		cses := make([]*tcoin.CoinShare, b.poset.NProc())
		for pid := 0; pid < b.poset.NProc(); pid++ {
			if b.votes[creator][pid].isCorrect() {
				cses[pid] = b.tcoins[pid].CreateCoinShare(level)
			}
		}
		return marshallShares(cses)
	}
	return []byte{}
}

func unitsOnLevel(p gomel.Poset, level int) []gomel.Unit {
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

func level(pu gomel.Preunit, poset gomel.Poset) (int, error) {
	if len(pu.Parents()) == 0 {
		return 0, nil
	}
	predecessor := poset.Get(pu.Parents()[:1])
	if predecessor[0] == nil {
		return 0, errors.New("Predecessor doesn't exist in the poset")
	}
	return predecessor[0].Level() + 1, nil
}
