package beacon

import (
	"errors"
	"math/big"
	"sort"
	"sync"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/crypto/tcoin"
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

// TODO: Consider more sophisticated thread safety (replace the global lock)
type beacon struct {
	sync.RWMutex
	pid        int
	poset      gomel.Poset
	multicoins []*tcoin.ThresholdCoin
	tcoins     []*tcoin.ThresholdCoin
	// vote[i][j] is the vote of i-th process on the j-th tcoin
	// it is nil when j-th dealing unit is not below
	// the unit of i-th process on votingLevel
	votes [][]*vote
	// shareProviders[i] is the set of the processes that voted "yes"
	// for all the tcoins which forms the i-th multicion
	// those processes should provide shares for the i-th multicoin
	shareProviders []map[int]bool
	// shares[i] is a map of the form
	// hash of a unit => the share for the i-th multicoin contained in the unit
	shares []map[gomel.Hash]*tcoin.CoinShare
}

type vote struct {
	isCorrect bool
	// proof != nil only when isCorrect = false
	proof *big.Int
}

// NewBeacon returns a RandomSource based on a beacon
func NewBeacon(poset gomel.Poset, pid int) gomel.RandomSource {
	b := &beacon{
		pid:            pid,
		poset:          poset,
		multicoins:     make([]*tcoin.ThresholdCoin, poset.NProc()),
		votes:          make([][]*vote, poset.NProc()),
		shareProviders: make([]map[int]bool, poset.NProc()),
		tcoins:         make([]*tcoin.ThresholdCoin, poset.NProc()),
		shares:         make([]map[gomel.Hash]*tcoin.CoinShare, poset.NProc()),
	}
	for i := 0; i < poset.NProc(); i++ {
		b.votes[i] = make([]*vote, poset.NProc())
		b.shares[i] = make(map[gomel.Hash]*tcoin.CoinShare)
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
		return (permutation[i] < permutation[j])
	})

	return permutation
}

// RandomBytes returns a sequence of random bits for a given process and nonce
// in the case of fail it returns nil
func (b *beacon) RandomBytes(uTossing gomel.Unit, level int) []byte {
	b.Lock()
	defer b.Unlock()

	shares := []*tcoin.CoinShare{}
	units := unitsOnLevel(b.poset, level)
	for _, u := range units {
		if cs, ok := b.shares[uTossing.Creator()][*u.Hash()]; ok {
			shares = append(shares, cs)
		}
	}
	coin, ok := b.multicoins[uTossing.Creator()].CombineCoinShares(shares)
	if !ok {
		return nil
	}
	return coin.RandomBytes()
}

// Update updates the RandomSource with data included in the preunit
func (b *beacon) Update(pu gomel.Preunit) error {
	b.Lock()
	defer b.Unlock()

	if u := b.poset.Get([]*gomel.Hash{pu.Hash()})[0]; u != nil {
		return gomel.NewDuplicateUnit(u)
	}

	level, err := level(pu, b.poset)
	if err != nil {
		return err
	}
	if level == dealingLevel {
		tcEncoded := pu.RandomSourceData()
		tc, err := tcoin.Decode(tcEncoded, b.pid)
		if err != nil {
			return err
		}
		b.tcoins[pu.Creator()] = tc
		b.votes[b.pid][pu.Creator()] = verifyTCoin(tc)
	}
	if level == votingLevel {
		votes, err := unmarshallVotes(pu.RandomSourceData(), b.poset.NProc())
		if err != nil {
			return err
		}
		err = validateVotes(b, pu, votes)
		if err != nil {
			return err
		}
		b.votes[pu.Creator()] = make([]*vote, b.poset.NProc())
		for pid, vote := range votes {
			b.votes[pu.Creator()][pid] = vote
		}
	}
	if level == multicoinLevel {
		coinsApprovedBy := make([]int, b.poset.NProc())
		nBelowPuOnVotingLevel := 0
		providers := make(map[int]bool)
		votingUnits := unitsOnLevel(b.poset, votingLevel)

		for _, u := range votingUnits {
			if gomel.BelowAny(u, b.poset.Get(pu.Parents())) {
				providers[u.Creator()] = true
				nBelowPuOnVotingLevel++
				for pid := 0; pid < b.poset.NProc(); pid++ {
					if b.votes[u.Creator()][pid] != nil && b.votes[u.Creator()][pid].isCorrect {
						coinsApprovedBy[pid]++
					}
				}
			}
		}
		coinsToMerge := []*tcoin.ThresholdCoin{}
		for pid := 0; pid < b.poset.NProc(); pid++ {
			if coinsApprovedBy[pid] == nBelowPuOnVotingLevel {
				coinsToMerge = append(coinsToMerge, b.tcoins[pid])
			}
		}
		b.multicoins[pu.Creator()] = tcoin.CreateMulticoin(coinsToMerge, b.pid)
		b.shareProviders[pu.Creator()] = providers
	}
	if level >= sharesLevel {
		shares, err := unmarshallShares(pu.RandomSourceData(), b.poset.NProc())
		if err != nil {
			return err
		}
		for pid := 0; pid < b.poset.NProc(); pid++ {
			if b.shareProviders[pid][pu.Creator()] {
				if shares[pid] == nil {
					return errors.New("Preunit without a share it should contain ")
				}
				// This verification is slow
				if !b.multicoins[pid].VerifyCoinShare(shares[pid], level) {
					return errors.New("Preunit contains wrong coin share")
				}
				b.shares[pid][*pu.Hash()] = shares[pid]
			}
		}
	}
	return nil
}

func validateVotes(b *beacon, pu gomel.Preunit, votes []*vote) error {
	dealingUnits := unitsOnLevel(b.poset, dealingLevel)
	createdDealing := make([]bool, b.poset.NProc())
	for _, u := range dealingUnits {
		parents := b.poset.Get(pu.Parents())
		shouldVote := gomel.BelowAny(u, parents)
		if shouldVote && votes[u.Creator()] == nil {
			return errors.New("Missing vote")
		}
		if !shouldVote && votes[u.Creator()] != nil {
			return errors.New("Vote on dealing unit not below the preunit")
		}
		if shouldVote && votes[u.Creator()].isCorrect == false {
			proof := votes[u.Creator()].proof
			if !b.tcoins[u.Creator()].VerifyWrongSecretKeyProof(pu.Creator(), proof) {
				return errors.New("The provided proof is incorrect")
			}
		}
		createdDealing[u.Creator()] = true
	}
	for pid := range createdDealing {
		if votes[pid] != nil && !createdDealing[pid] {
			return errors.New("Vote on non-existing dealing units")
		}
	}
	return nil
}

func verifyTCoin(tc *tcoin.ThresholdCoin) *vote {
	// TODO: proper implementation
	return &vote{
		isCorrect: true,
		proof:     nil,
	}
}

// Rollback rolls back an update
func (b *beacon) Rollback(pu gomel.Preunit) {
	b.Lock()
	defer b.Unlock()

	level, _ := level(pu, b.poset)
	if level == dealingLevel {
		b.tcoins[pu.Creator()] = nil
		b.votes[b.pid][pu.Creator()] = nil
		return
	}
	if level == votingLevel {
		b.votes[pu.Creator()] = nil
		return
	}
	if level == multicoinLevel {
		b.multicoins[pu.Creator()] = nil
		b.shareProviders[pu.Creator()] = nil
		return
	}
	if level == sharesLevel {
		for pid := 0; pid < b.poset.NProc(); pid++ {
			delete(b.shares[pid], *pu.Hash())
		}
	}
}

func (b *beacon) DataToInclude(creator int, parents []gomel.Unit, level int) []byte {
	b.Lock()
	defer b.Unlock()

	if level == dealingLevel {
		nProc := b.poset.NProc()
		return tcoin.Deal(nProc, nProc/3+1)
	}
	if level == votingLevel {
		votes := make([]*vote, b.poset.NProc())
		dealingUnits := unitsOnLevel(b.poset, dealingLevel)
		for _, u := range dealingUnits {
			if gomel.BelowAny(u, parents) {
				votes[u.Creator()] = b.votes[creator][u.Creator()]
			}
		}
		return marshallVotes(votes)
	}
	if level >= sharesLevel {
		cses := make([]*tcoin.CoinShare, b.poset.NProc())
		for pid := 0; pid < b.poset.NProc(); pid++ {
			if b.shareProviders[pid][creator] {
				cses[pid] = b.multicoins[pid].CreateCoinShare(level)
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
