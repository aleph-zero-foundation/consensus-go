package random

import (
	"errors"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/crypto/tcoin"
)

type tcRandomSource struct {
	pid        int
	poset      gomel.Poset
	tcs        *syncTCMap
	coinShares *SyncCSMap
}

// NewTcSource returns a RandomSource based on threshold coins
func NewTcSource(poset gomel.Poset, pid int) gomel.RandomSource {
	return &tcRandomSource{
		pid:        pid,
		poset:      poset,
		tcs:        newSyncTCMap(),
		coinShares: NewSyncCSMap(),
	}
}

// GetCRP is a dummy implementation of a common random permutation
func (rs *tcRandomSource) GetCRP(nonce int) []int {
	nProc := rs.poset.NProc()
	permutation := make([]int, nProc)
	for i := 0; i < nProc; i++ {
		permutation[i] = (i + nonce) % nProc
	}
	return permutation
}

// RandomBytes returns a sequence of random bits for a given process and nonce
// in the case of fail it returns nil
func (rs *tcRandomSource) RandomBytes(uTossing gomel.Unit, level int) []byte {
	if level != uTossing.Level() {
		return nil
	}
	var dealer gomel.Unit
	var tc *tcoin.ThresholdCoin
	shares := []*tcoin.CoinShare{}
	shareCollected := make(map[int]bool)

	rs.poset.PrimeUnits(level - 1).Iterate(func(units []gomel.Unit) bool {
		for _, v := range units {
			if !v.Below(uTossing) {
				continue
			}
			if shareCollected[v.Creator()] {
				continue
			}
			fduV := rs.firstDealingUnit(v)
			if dealer == nil {
				dealer = fduV
				tc = rs.tcs.get(dealer.Hash())
			}
			if dealer != fduV {
				continue
			}
			cs := rs.coinShares.Get(v.Hash())
			if cs != nil {
				if tc.VerifyCoinShare(cs, level-1) {
					shares = append(shares, cs)
					shareCollected[v.Creator()] = true
					if len(shares) == tc.Threshold {
						return false
					}
					return true
				}
			}
		}
		return true
	})

	coin, ok := tc.CombineCoinShares(shares)
	if !ok || !tc.VerifyCoin(coin, level-1) {
		return nil
	}
	return coin.RandomBytes()
}

// Update updates the RandomSource with data included in the preunit
func (rs *tcRandomSource) Update(u gomel.Unit) {
	if gomel.Dealing(u) {
		tc, _ := tcoin.Decode(u.RandomSourceData(), rs.pid)
		rs.tcs.add(u.Hash(), tc)
	} else if gomel.Prime(u) {
		cs := new(tcoin.CoinShare)
		cs.Unmarshal(u.RandomSourceData())
		rs.coinShares.Add(u.Hash(), cs)
	}
}

// CheckCompliance checks if the random source data included in the unit
// is correct
func (rs *tcRandomSource) CheckCompliance(u gomel.Unit) error {
	if gomel.Dealing(u) {
		_, err := tcoin.Decode(u.RandomSourceData(), rs.pid)
		if err != nil {
			return err
		}
	} else if gomel.Prime(u) {
		cs := new(tcoin.CoinShare)
		err := cs.Unmarshal(u.RandomSourceData())
		if err != nil {
			return err
		}
		fdu := rs.firstDealingUnit(u)
		tc := rs.tcs.get(fdu.Hash())
		if !tc.VerifyCoinShare(cs, u.Level()) {
			return errors.New("Invalid coin share")
		}
	}
	return nil
}

// DataToInclude returns data which should be included in the unit under creation
// with given creator and set of parents.
func (rs *tcRandomSource) DataToInclude(creator int, parents []gomel.Unit, level int) []byte {
	// dealing unit
	if len(parents) == 0 {
		nProc := rs.poset.NProc()
		return tcoin.Deal(nProc, nProc/3+1)
	}
	// prime non-dealing unit
	if parents[0].Level() != level {
		return rs.createCoinShare(parents, level).Marshal()
	}
	return nil
}

func (rs *tcRandomSource) createCoinShare(parents []gomel.Unit, level int) *tcoin.CoinShare {
	fdu := rs.firstDealingUnitFromParents(parents, level)
	tc := rs.tcs.get(fdu.Hash())
	return tc.CreateCoinShare(level)
}

// hasForkingEvidenceFromParents checks whether parents have evidence that
// the creator is forking.
func hasForkingEvidenceFromParents(parents []gomel.Unit, creator int) bool {
	var heighest gomel.Unit
	for _, p := range parents {
		if p.HasForkingEvidence(creator) {
			return true
		}
		if len(p.Floor()[creator]) == 1 {
			u := p.Floor()[creator][0]
			if heighest == nil {
				heighest = u
			} else {
				if heighest.Height() <= u.Height() {
					if !heighest.Below(u) {
						return true
					}
					heighest = u
				} else {
					if !u.Below(heighest) {
						return true
					}
				}
			}
		}
	}
	return false
}

// firstDealingUnitFromParents takes parents of the unit under construction
// and calculates the first (sorted with respect to CRP on level of the unit) dealing unit
// that is below the unit under construction
func (rs *tcRandomSource) firstDealingUnitFromParents(parents []gomel.Unit, level int) gomel.Unit {
	dealingUnits := rs.poset.PrimeUnits(0)
	for _, dealer := range rs.GetCRP(level) {
		if hasForkingEvidenceFromParents(parents, dealer) {
			continue
		}
		for _, u := range dealingUnits.Get(dealer) {
			if gomel.BelowAny(u, parents) {
				return u
			}
		}
	}
	return nil
}

func (rs *tcRandomSource) firstDealingUnit(u gomel.Unit) gomel.Unit {
	dealingUnits := rs.poset.PrimeUnits(0)
	for _, dealer := range rs.GetCRP(u.Level()) {
		if u.HasForkingEvidence(dealer) {
			continue
		}
		for _, v := range dealingUnits.Get(dealer) {
			if v.Below(u) {
				return v
			}
		}
	}
	return nil
}
