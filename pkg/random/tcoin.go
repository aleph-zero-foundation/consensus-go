package random

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/crypto/tcoin"
)

type tcRandomSource struct {
	poset      gomel.Poset
	tcs        *safeTCMap
	coinShares *safeCSMap
}

// NewTcRandomSource returns a RandomSource based on threshold coins
func NewTcRandomSource(poset gomel.Poset) gomel.RandomSource {
	return &tcRandomSource{
		poset:      poset,
		tcs:        newSafeTCMap(),
		coinShares: newSafeCSMap(),
	}
}

// GetCRP is a dummy implementation of a common random permutation
// TODO: implement
func (rs *tcRandomSource) GetCRP(nonce int) []int {
	nProc := rs.poset.NProc()
	permutation := make([]int, nProc)
	for i := 0; i < nProc; i++ {
		permutation[i] = (i + nonce) % nProc
	}
	return permutation
}

// RandomBits returns a sequence of random bits for a given process and nonce
// in the case of fail it returns nil
func (rs *tcRandomSource) RandomBytes(uTossing gomel.Unit, nonce int) []byte {
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
				tc = rs.tcs.get(dealer.Hash())
			}
			if dealer != fduV {
				continue
			}
			cs := rs.coinShares.get(v.Hash())
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
func (rs *tcRandomSource) Update(pu gomel.Preunit, data []byte) error {
	tc, cs, err := unmarshall(data, pu.Creator())
	if err != nil {
		return err
	}
	if cs != nil {
		rs.coinShares.add(pu.Hash(), cs)
	}
	if tc != nil {
		rs.tcs.add(pu.Hash(), tc)
	}
	return nil
}

// Rollback rolls back an update
func (rs *tcRandomSource) Rollback(pu gomel.Preunit) error {
	rs.coinShares.remove(pu.Hash())
	rs.tcs.remove(pu.Hash())
	return nil
}

// ToInclude returns data which should be included in the unit under creation
// with given creator and set of parents.
func (rs *tcRandomSource) DataToInclude(creator int, parents []gomel.Unit, level int) []byte {
	nProc := rs.poset.NProc()
	if len(parents) == 0 {
		return marshall(tcoin.Deal(nProc, nProc/3+1), nil)
	}
	return marshall(nil, rs.createCoinShare(parents, level))
}

func (rs *tcRandomSource) createCoinShare(parents []gomel.Unit, level int) *tcoin.CoinShare {
	fdu := rs.firstDealingUnitFromParents(parents, level)
	tc := rs.tcs.get(fdu.Hash())
	if tc == nil {
		// This is only needed for tests where we don't currently have threshold coins.
		// TODO: Add threshold coins to tests?
		return nil
	}
	return tc.CreateCoinShare(level)
}

// marshall returns marshalled ThresholdCoin and CoinShare in the following
// format:
// (1) mask indicating what type of data is included (1 byte)
//		 0 (empty), 1 (only tc), 2 (only cs)
// (2) the data marshalled
func marshall(tcEncoded []byte, cs *tcoin.CoinShare) []byte {
	if tcEncoded != nil {
		data := make([]byte, 1+len(tcEncoded))
		data[0] = 1
		copy(data[1:], tcEncoded)
		return data
	}
	if cs != nil {
		csData := cs.Marshal()
		data := make([]byte, 1+len(csData))
		data[0] = 2
		copy(data[1:], csData)
		return data
	}
	data := []byte{0}
	return data
}

func unmarshall(data []byte, pid int) (*tcoin.ThresholdCoin, *tcoin.CoinShare, error) {
	dataType := data[0]
	if dataType == 1 {
		tc, err := tcoin.Decode(data[1:], pid)
		if err != nil {
			return nil, nil, err
		}
		return tc, nil, nil
	}
	if dataType == 2 {
		cs := new(tcoin.CoinShare)
		err := cs.Unmarshal(data[1:])
		if err != nil {
			return nil, nil, err
		}
		return nil, cs, nil
	}
	return nil, nil, nil
}

// firstDealingUnitFromParents takes parents of the unit under construction
// and calculates the first (sorted with respect to CRP on level of the unit) dealing unit
// that is below the unit under construction
func (rs *tcRandomSource) firstDealingUnitFromParents(parents []gomel.Unit, level int) gomel.Unit {
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

func (rs *tcRandomSource) firstDealingUnit(u gomel.Unit) gomel.Unit {
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
