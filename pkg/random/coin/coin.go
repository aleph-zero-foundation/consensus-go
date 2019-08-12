package coin

import (
	"errors"
	"math/big"
	"math/rand"

	"gitlab.com/alephledger/consensus-go/pkg/crypto/bn256"
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
func New(nProc, pid int, tcoin *tcoin.ThresholdCoin, shareProvider map[int]bool) gomel.RandomSource {
	return &coin{
		pid:           pid,
		tc:            tcoin,
		coinShares:    random.NewSyncCSMap(),
		shareProvider: shareProvider,
	}
}

// NewFixedCoin returns a Coin random source generated using the given seed.
// This function should be used only for testing, as it is not safe,
// because all the secrets could be revealed knowing the seed.
func NewFixedCoin(nProc, pid, seed int) gomel.RandomSource {
	rnd := rand.New(rand.NewSource(int64(seed)))
	threshold := nProc/3 + 1

	shareProviders := make(map[int]bool)
	for i := 0; i < nProc; i++ {
		shareProviders[i] = true
	}

	coeffs := make([]*big.Int, threshold)
	for i := 0; i < threshold; i++ {
		coeffs[i] = big.NewInt(0).Rand(rnd, bn256.Order)
	}

	return New(nProc, pid, tcoin.New(nProc, pid, coeffs), shareProviders)
}

// Init initialize the coin with given dag
func (c *coin) Init(dag gomel.Dag) {
	c.dag = dag
}

// RandomBytes returns a sequence of random bits for a given level.
// The first argument is irrelevant for this random source.
// If there are not enough shares on the level it returns nil.
// If the dag reached level+1 the existence of enough shares is guaranteed.
func (c *coin) RandomBytes(pid int, level int) []byte {
	shares := []*tcoin.CoinShare{}
	shareCollected := make(map[int]bool)

	su := c.dag.PrimeUnits(level)
	if su == nil {
		return nil
	}
	su.Iterate(func(units []gomel.Unit) bool {
		for _, v := range units {
			if !c.shareProvider[v.Creator()] || shareCollected[v.Creator()] {
				continue
			}
			cs := c.coinShares.Get(v.Hash())
			if cs != nil && c.tc.VerifyCoinShare(cs, level) {
				shares = append(shares, cs)
				shareCollected[v.Creator()] = true
				if len(shares) == c.tc.Threshold {
					return false
				}
				return true
			}
		}
		return true
	})
	if len(shares) < c.tc.Threshold {
		// no enough shares
		return nil
	}

	// As the shares are already verified we have guarantee that combining
	// shares will be successful
	coin, _ := c.tc.CombineCoinShares(shares)
	return coin.RandomBytes()
}

// Update updates the RandomSource with data included in the preunit
func (c *coin) Update(u gomel.Unit) {
	if gomel.Prime(u) && c.shareProvider[u.Creator()] {
		cs := new(tcoin.CoinShare)
		cs.Unmarshal(u.RandomSourceData())
		c.coinShares.Add(u.Hash(), cs)
	}
}

// CheckCompliance checks if the random source data included in the unit
// is correct
func (c *coin) CheckCompliance(u gomel.Unit) error {
	if gomel.Prime(u) && c.shareProvider[u.Creator()] {
		cs := new(tcoin.CoinShare)
		err := cs.Unmarshal(u.RandomSourceData())
		if err != nil {
			return err
		}
		if !c.tc.VerifyCoinShare(cs, u.Level()) {
			return errors.New("invalid share")
		}
	} else if u.RandomSourceData() != nil {
		return errors.New("random source data should be empty")
	}
	return nil
}

// DataToInclude returns data which should be included in the unit under creation
// with given creator and set of parents.
func (c *coin) DataToInclude(creator int, parents []gomel.Unit, level int) []byte {
	if (len(parents) == 0 || parents[0].Level() != level) && c.shareProvider[creator] {
		return c.tc.CreateCoinShare(level).Marshal()
	}
	return nil
}
