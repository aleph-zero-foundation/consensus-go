package coin

import (
	"errors"

	"gitlab.com/alephledger/consensus-go/pkg/crypto/tcoin"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/random"
)

type coin struct {
	pid           int
	dag           gomel.Dag
	tc            *tcoin.ThresholdCoin
	coinShares    *random.SyncCSMap
	shareProvider map[int]bool
}

// New returns a Coin RandomSource based on fixed thresholdCoin with given
// set of share providers.
// It is meant to be used in the main process.
// The result of the setup phase should be a consensus on this random source.
func New(dag gomel.Dag, pid int, tcoin *tcoin.ThresholdCoin, shareProvider map[int]bool) gomel.RandomSource {
	return &coin{
		pid:           pid,
		dag:           dag,
		tc:            tcoin,
		coinShares:    random.NewSyncCSMap(),
		shareProvider: shareProvider,
	}
}

// GetCRP implements a common random permutation
func (rs *coin) GetCRP(level int) []int {
	return random.CRP(rs, rs.dag, level)
}

// RandomBytes returns a sequence of random bits for a given level.
// The first argument is irrelevant for this random source.
// If there are not enough shares on the level it returns nil.
// If the dag reached level+1 the existence of enough shares is guaranteed.
func (rs *coin) RandomBytes(_ gomel.Unit, level int) []byte {
	shares := []*tcoin.CoinShare{}
	shareCollected := make(map[int]bool)

	su := rs.dag.PrimeUnits(level)
	if su == nil {
		return nil
	}
	su.Iterate(func(units []gomel.Unit) bool {
		for _, v := range units {
			if !rs.shareProvider[v.Creator()] || shareCollected[v.Creator()] {
				continue
			}
			cs := rs.coinShares.Get(v.Hash())
			if cs != nil && rs.tc.VerifyCoinShare(cs, level) {
				shares = append(shares, cs)
				shareCollected[v.Creator()] = true
				if len(shares) == rs.tc.Threshold {
					return false
				}
				return true
			}
		}
		return true
	})
	if len(shares) < rs.tc.Threshold {
		// no enough shares
		return nil
	}

	// As the shares are already verified we have guarantee that combining
	// shares will be successful
	coin, _ := rs.tc.CombineCoinShares(shares)
	return coin.RandomBytes()
}

// Update updates the RandomSource with data included in the preunit
func (rs *coin) Update(u gomel.Unit) {
	if gomel.Prime(u) && rs.shareProvider[u.Creator()] {
		cs := new(tcoin.CoinShare)
		cs.Unmarshal(u.RandomSourceData())
		rs.coinShares.Add(u.Hash(), cs)
	}
}

// CheckCompliance checks if the random source data included in the unit
// is correct
func (rs *coin) CheckCompliance(u gomel.Unit) error {
	if gomel.Prime(u) && rs.shareProvider[u.Creator()] {
		cs := new(tcoin.CoinShare)
		err := cs.Unmarshal(u.RandomSourceData())
		if err != nil {
			return err
		}
		if !rs.tc.VerifyCoinShare(cs, u.Level()) {
			return errors.New("invalid share")
		}
	} else if u.RandomSourceData() != nil {
		return errors.New("random source data should be empty")
	}
	return nil
}

// DataToInclude returns data which should be included in the unit under creation
// with given creator and set of parents.
func (rs *coin) DataToInclude(creator int, parents []gomel.Unit, level int) []byte {
	if (len(parents) == 0 || parents[0].Level() != level) && rs.shareProvider[creator] {
		return rs.tc.CreateCoinShare(level).Marshal()
	}
	return nil
}
