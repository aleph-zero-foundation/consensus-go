package urn

import (
	"errors"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/crypto/tcoin"
	"gitlab.com/alephledger/consensus-go/pkg/random"
)

type urn struct {
	pid        int
	poset      gomel.Poset
	tcs        *random.SyncTCMap
	coinShares *random.SyncCSMap
}

// NewUrn returns a RandomSource based on multiple threshold coins
// as explained in the first version of the whitepaper.
// (i.e. we choose the dealer using the random permutation which is defined
// as pseudo-random function of processes public keys.
// The permutation is known to the adversary in advance and this knowledge
// can be used in a potential attack).
func NewUrn(poset gomel.Poset, pid int) gomel.RandomSource {
	return &urn{
		pid:        pid,
		poset:      poset,
		tcs:        random.NewSyncTCMap(),
		coinShares: random.NewSyncCSMap(),
	}
}

// GetCRP is a dummy implementation of a common random permutation
func (rs *urn) GetCRP(nonce int) []int {
	nProc := rs.poset.NProc()
	permutation := make([]int, nProc)
	for i := 0; i < nProc; i++ {
		permutation[i] = (i + nonce) % nProc
	}
	return permutation
}

// RandomBytes returns a sequence of random bits for a given process and nonce
// in the case of fail it returns nil
func (rs *urn) RandomBytes(uTossing gomel.Unit, nonce int) []byte {
	level := uTossing.Level() - 1
	var dealer gomel.Unit
	var tc *tcoin.ThresholdCoin
	shares := []*tcoin.CoinShare{}
	shareCollected := make(map[int]bool)

	rs.poset.PrimeUnits(level).Iterate(func(units []gomel.Unit) bool {
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
				tc = rs.tcs.Get(dealer.Hash())
			}
			if dealer != fduV {
				continue
			}
			cs := rs.coinShares.Get(v.Hash())
			if cs != nil {
				if tc.VerifyCoinShare(cs, level) {
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
	if !ok || !tc.VerifyCoin(coin, level) {
		return nil
	}
	return coin.RandomBytes()
}

// Update updates the RandomSource with data included in the preunit
func (rs *urn) Update(u gomel.Unit) {
	if gomel.Dealing(u) {
		tc, _ := tcoin.Decode(u.RandomSourceData(), rs.pid)
		rs.tcs.Add(u.Hash(), tc)
	} else if gomel.Prime(u) {
		cs := new(tcoin.CoinShare)
		cs.Unmarshal(u.RandomSourceData())
		rs.coinShares.Add(u.Hash(), cs)
	}
}

// CheckCompliance checks if the random source data included in the unit
// is correct
func (rs *urn) CheckCompliance(u gomel.Unit) error {
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
		tc := rs.tcs.Get(fdu.Hash())
		if !tc.VerifyCoinShare(cs, u.Level()) {
			return errors.New("Invalid coin share")
		}
	}
	return nil
}

// DataToInclude returns data which should be included in the unit under creation
// with given creator and set of parents.
func (rs *urn) DataToInclude(creator int, parents []gomel.Unit, level int) []byte {
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

func (rs *urn) createCoinShare(parents []gomel.Unit, level int) *tcoin.CoinShare {
	fdu := rs.firstDealingUnitFromParents(parents, level)
	tc := rs.tcs.Get(fdu.Hash())
	return tc.CreateCoinShare(level)
}

// firstDealingUnitFromParents takes parents of the unit under construction
// and calculates the first (sorted with respect to CRP on level of the unit) dealing unit
// that is below the unit under construction
func (rs *urn) firstDealingUnitFromParents(parents []gomel.Unit, level int) gomel.Unit {
	dealingUnits := rs.poset.PrimeUnits(0)
	for _, dealer := range rs.GetCRP(level) {
		// We are only checking if there are forked dealing units created by the dealer
		// below the unit under construction.
		// We could check if we have evidence that the dealer is forking
		// but this is expensive without access to floors.
		var result gomel.Unit
		for _, u := range dealingUnits.Get(dealer) {
			if gomel.BelowAny(u, parents) {
				if result != nil {
					// we see forked dealing unit
					result = nil
					break
				} else {
					result = u
				}
			}
		}
		if result != nil {
			return result
		}
	}
	return nil
}

func (rs *urn) firstDealingUnit(u gomel.Unit) gomel.Unit {
	dealingUnits := rs.poset.PrimeUnits(0)
	for _, dealer := range rs.GetCRP(u.Level()) {
		var result gomel.Unit
		// We are only checking if there are forked dealing units created by the dealer below u.
		// We can change it to hasForkingEvidence, but we would have to also implement
		// this in creating.
		for _, v := range dealingUnits.Get(dealer) {
			if v.Below(u) {
				if result != nil {
					// we see forked dealing unit
					result = nil
					break
				} else {
					result = v
				}
			}
		}
		if result != nil {
			return result
		}
	}
	return nil
}
