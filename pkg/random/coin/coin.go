// Package coin implements the coin random source.
//
// It is meant to be used in the main process.
// The result of the setup phase should be a consensus on this random source.
package coin

import (
	"crypto/subtle"
	"errors"
	"math/big"
	"math/rand"

	"gitlab.com/alephledger/consensus-go/pkg/crypto/bn256"
	"gitlab.com/alephledger/consensus-go/pkg/crypto/p2p"
	"gitlab.com/alephledger/consensus-go/pkg/crypto/tcoin"
	chdag "gitlab.com/alephledger/consensus-go/pkg/dag"
	"gitlab.com/alephledger/consensus-go/pkg/dag/check"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/random"
)

type coin struct {
	pid           uint16
	dag           gomel.Dag
	tc            *tcoin.ThresholdCoin
	coinShares    *random.SyncCSMap
	shareProvider map[uint16]bool
	randomBytes   *random.SyncBytesSlice
}

// New returns a Coin RandomSource based on fixed thresholdCoin with the given
// set of share providers.
func New(nProc, pid uint16, tcoin *tcoin.ThresholdCoin, shareProvider map[uint16]bool) gomel.RandomSource {
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
func NewFixedCoin(nProc, pid uint16, seed int, shareProviders map[uint16]bool) gomel.RandomSource {
	rnd := rand.New(rand.NewSource(int64(seed)))
	threshold := gomel.MinimalTrusted(nProc)

	coeffs := make([]*big.Int, threshold)
	for i := uint16(0); i < threshold; i++ {
		coeffs[i] = big.NewInt(0).Rand(rnd, bn256.Order)
	}

	sKeys := make([]*p2p.SecretKey, nProc)
	pKeys := make([]*p2p.PublicKey, nProc)
	for i := uint16(0); i < nProc; i++ {
		pKeys[i], sKeys[i], _ = p2p.GenerateKeys()
	}
	dealer := uint16(0)

	p2pKeys, _ := p2p.Keys(sKeys[dealer], pKeys, dealer)

	gtc := tcoin.NewGlobal(nProc, coeffs)
	tc, _ := gtc.Encrypt(p2pKeys)
	myTC, _, _ := tcoin.Decode(tc.Encode(), dealer, pid, p2pKeys[pid])

	return New(nProc, pid, myTC, shareProviders)
}

// Bind the coin with the dag.
func (c *coin) Bind(dag gomel.Dag) gomel.Dag {
	c.dag = dag
	return chdag.BeforeEmplace(check.Units(dag, c.checkCompliance), c.update)
}

// RandomBytes returns a sequence of random bits for a given level.
// The first argument is irrelevant for this random source.
// It returns nil when the dag hasn't reached level+1 yet.
func (c *coin) RandomBytes(_ uint16, level int) []byte {
	return c.randomBytes.Get(level)
}

func (c *coin) update(u gomel.Unit) {
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

// checkCompliance checks if the random source data included in the unit
// is correct. The following rules should be satisfied:
//  (1) A dealing unit created by a share provider should contain a marshalled share
//  (2) A non-dealing prime unit should start with random bytes from the previous level,
//  followed by a marshalled coin share, if the creator is a share provider.
//  (3) Every other unit's random source data should be empty.
func (c *coin) checkCompliance(u gomel.Unit) error {
	if gomel.Dealing(u) && c.shareProvider[u.Creator()] {
		return new(tcoin.CoinShare).Unmarshal(u.RandomSourceData())
	}

	if gomel.Prime(u) && !gomel.Dealing(u) {
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

		if c.shareProvider[u.Creator()] {
			err := new(tcoin.CoinShare).Unmarshal(u.RandomSourceData()[bn256.SignatureLength:])
			if err != nil {
				return err
			}
		}
		return nil
	}

	if u.RandomSourceData() != nil {
		return errors.New("random source data should be empty")
	}
	return nil
}

// DataToInclude returns data which should be included in a unit
// with the given creator and set of parents.
// If the unit is the first unit on its level (>0) the coin shares
// from the previous level will be combined.
// If the shares don't combine to the correct random bytes for previous level
// it returns an error. This means that someone had included a wrong coin share
// and we should start an alert.
func (c *coin) DataToInclude(creator uint16, parents []gomel.Unit, level int) ([]byte, error) {
	if parents[creator] == nil {
		if c.shareProvider[creator] {
			return c.tc.CreateCoinShare(level).Marshal(), nil
		}
		return nil, nil
	}
	if parents[creator].Level() != level {
		var rb []byte
		if rbl := c.randomBytes.Get(level - 1); rbl != nil {
			rb = make([]byte, bn256.SignatureLength)
			copy(rb, rbl)
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
	shareCollected := make(map[uint16]bool)

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
				if len(shares) == int(c.tc.Threshold()) {
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
