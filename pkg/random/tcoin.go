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
	coinShares *syncCSMap
}

// NewTcSource returns a RandomSource based on threshold coins
func NewTcSource(poset gomel.Poset, pid int) gomel.RandomSource {
	return &tcRandomSource{
		pid:        pid,
		poset:      poset,
		tcs:        newSyncTCMap(),
		coinShares: newSyncCSMap(),
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

// RandomBytes returns a sequence of random bits for a given process and nonce
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
func (rs *tcRandomSource) Update(pu gomel.Preunit) error {
	tc, cs, err := unmarshall(pu.RandomSourceData(), rs.pid)
	if err != nil {
		return err
	}
	if len(pu.Parents()) == 0 && tc == nil {
		return errors.New("Dealing unit without threshold coin machine")
	}
	// TODO: checking if pu is a primeUnit
	// and if so checking if it contains a coinShare
	if cs != nil {
		rs.coinShares.add(pu.Hash(), cs)
	}
	if tc != nil {
		rs.tcs.add(pu.Hash(), tc)
	}
	return nil
}

// Rollback rolls back an update
func (rs *tcRandomSource) Rollback(pu gomel.Preunit) {
	rs.coinShares.remove(pu.Hash())
	rs.tcs.remove(pu.Hash())
}

// ToInclude returns data which should be included in the unit under creation
// with given creator and set of parents.
func (rs *tcRandomSource) DataToInclude(creator int, parents []gomel.Unit, level int) []byte {
	// dealing unit
	if len(parents) == 0 {
		nProc := rs.poset.NProc()
		return marshall(tcoin.Deal(nProc, nProc/3+1), nil)
	}
	// prime non-dealing unit
	if parents[0].Level() != level {
		return marshall(nil, rs.createCoinShare(parents, level))
	}
	return nil
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
					u = heighest
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
