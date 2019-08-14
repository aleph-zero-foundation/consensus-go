package coin

import (
	"crypto/subtle"
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
	randomBytes   *random.SyncBytesSlice
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
		randomBytes:   random.NewSyncBytesSlice(),
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
// It returns nil when the dag haven't reached level+1 level yet.
func (c *coin) RandomBytes(_ int, level int) []byte {
	return c.randomBytes.Get(level)
}

// Update updates the RandomSource with data included in the preunit
func (c *coin) Update(u gomel.Unit) {
	if gomel.Prime(u) && c.shareProvider[u.Creator()] {
		cs := new(tcoin.CoinShare)
		offset := bn256.SignatureLength
		if gomel.Dealing(u) {
			// dealing units doesn't contain random data from previous level
			offset = 0
		}
		cs.Unmarshal(u.RandomSourceData()[offset:])
		c.coinShares.Add(u.Hash(), cs)
	}
	if gomel.Prime(u) && !gomel.Dealing(u) {
		c.randomBytes.AppendOrIgnore(u.Level()-1, u.RandomSourceData()[:bn256.SignatureLength])
	}
}

// CheckCompliance checks if the random source data included in the unit
// is correct. The following rules should be satisfied:
// (1) dealing unit created by a share provider should contain marshalled share
// (2) non-dealing prime unit should start with random bytes from previous level,
// then if the creator is a share provider marshalled coin share should follow
// (3) every other unit should have empty random source data
func (c *coin) CheckCompliance(u gomel.Unit) error {
	if gomel.Dealing(u) {
		if c.shareProvider[u.Creator()] {
			cs := new(tcoin.CoinShare)
			err := cs.Unmarshal(u.RandomSourceData())
			if err != nil {
				return err
			}
			return nil
		} else if u.RandomSourceData() != nil {
			return errors.New("random source data should be empty")
		}
		return nil
	}

	if gomel.Prime(u) && c.shareProvider[u.Creator()] {
		if len(u.RandomSourceData()) < bn256.SignatureLength {
			return errors.New("random source data too short")
		}
		cs := new(tcoin.CoinShare)
		err := cs.Unmarshal(u.RandomSourceData()[bn256.SignatureLength:])
		if err != nil {
			return err
		}
	}
	if gomel.Prime(u) {
		if len(u.RandomSourceData()) < bn256.SignatureLength {
			return errors.New("random source data too short")
		}
		uRandomBytes := u.RandomSourceData()[:bn256.SignatureLength]
		if rb := c.randomBytes.Get(u.Level() - 1); rb != nil {
			if subtle.ConstantTimeCompare(rb, uRandomBytes) != 1 {
				return errors.New("incorrect random bytes")
			}
		} else {
			coin := new(tcoin.Coin)
			err := coin.Unmarshal(uRandomBytes)
			if err != nil {
				return err
			}
			if !c.tc.VerifyCoin(coin, u.Level()-1) {
				return errors.New("incorrect random bytes")
			}
		}
	} else if u.RandomSourceData() != nil {
		return errors.New("random source data should be empty")
	}
	return nil
}

// DataToInclude returns data which should be included in the unit under creation
// with given creator and set of parents.
// If the unit under creation is the first unit on its level (>0) the coin shares
// from previous level are being combined.
// If the shares don't combine to the correct random bytes for previous level
// it returns an error. This means that someone had put a wrong coin share
// and we should start an alert.
func (c *coin) DataToInclude(creator int, parents []gomel.Unit, level int) ([]byte, error) {
	if len(parents) == 0 {
		if c.shareProvider[creator] {
			return c.tc.CreateCoinShare(level).Marshal(), nil
		}
		return nil, nil
	}
	if parents[0].Level() != level {
		var rb []byte
		if c.randomBytes.Get(level-1) != nil {
			rb = make([]byte, bn256.SignatureLength)
			copy(rb, c.randomBytes.Get(level-1))
		} else {
			var err error
			rb, err = c.combineShares(level - 1)
			if err != nil {
				return nil, err
			}
			c.randomBytes.AppendOrIgnore(level-1, rb)
		}
		if c.shareProvider[creator] {
			rb = append(rb, c.tc.CreateCoinShare(level).Marshal()...)
		}
		return rb, nil
	}
	return nil, nil
}

func (c *coin) combineShares(level int) ([]byte, error) {
	shares := []*tcoin.CoinShare{}
	shareCollected := make(map[int]bool)

	su := c.dag.PrimeUnits(level)
	if su == nil {
		return nil, errors.New("no primes on a given level")
	}
	su.Iterate(func(units []gomel.Unit) bool {
		for _, v := range units {
			if !c.shareProvider[v.Creator()] || shareCollected[v.Creator()] {
				continue
			}
			cs := c.coinShares.Get(v.Hash())
			if cs != nil {
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

	coin, ok := c.tc.CombineCoinShares(shares)
	if !ok {
		return nil, errors.New("combining shares failed")
	}
	if !c.tc.VerifyCoin(coin, level) {
		return nil, errors.New("verification of coin failed")
	}
	return coin.RandomBytes(), nil
}
