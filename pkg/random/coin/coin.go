// Package coin implements the coin random source.
//
// It is meant to be used in the main process.
// The result of the setup phase should be a consensus on this random source.
package coin

import (
	"crypto/subtle"
	"encoding/binary"
	"errors"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/random"
	"gitlab.com/alephledger/core-go/pkg/crypto/bn256"
	"gitlab.com/alephledger/core-go/pkg/crypto/tss"
)

type coinFactory struct {
	pid   uint16
	wtkey *tss.WeakThresholdKey
}

// NewFactory creates a coin factory
func NewFactory(pid uint16, wtkey *tss.WeakThresholdKey) gomel.RandomSourceFactory {
	return &coinFactory{
		pid:   pid,
		wtkey: wtkey,
	}
}

// NewSeededFactory creates an unsafe coin factory. Should be used only for testing purposes
func NewSeededFactory(nProc, pid uint16, seed int64, shareProviders map[uint16]bool) gomel.RandomSourceFactory {
	wtk := tss.SeededWTK(nProc, pid, seed, shareProviders)
	return &coinFactory{pid, wtk}
}

func (cf *coinFactory) NewRandomSource(dag gomel.Dag) gomel.RandomSource {
	return newCoin(cf.pid, dag, cf.wtkey, cf.wtkey.ShareProviders())
}

func (cf *coinFactory) DealingData(epoch gomel.EpochID) ([]byte, error) {
	if cf.wtkey.ShareProviders()[cf.pid] {
		return cf.wtkey.CreateShare(nonce(0, epoch)).Marshal(), nil
	}
	return nil, nil
}

func nonce(level int, epoch gomel.EpochID) []byte {
	data := make([]byte, 8)
	binary.LittleEndian.PutUint64(data, uint64(epoch)<<16+uint64(level))
	return data
}

type coin struct {
	pid            uint16
	dag            gomel.Dag
	wtk            *tss.WeakThresholdKey
	coinShares     *random.SyncCSMap
	shareProviders map[uint16]bool
	randomBytes    *random.SyncBytesSlice
}

// newCoin returns a Coin RandomSource based on fixed thresholdCoin with the given set of share providers.
func newCoin(pid uint16, dag gomel.Dag, wtkey *tss.WeakThresholdKey, shareProviders map[uint16]bool) gomel.RandomSource {
	c := &coin{
		pid:            pid,
		dag:            dag,
		wtk:            wtkey,
		coinShares:     random.NewSyncCSMap(),
		shareProviders: shareProviders,
		randomBytes:    random.NewSyncBytesSlice(),
	}
	dag.AddCheck(c.checkCompliance)
	dag.BeforeInsert(c.update)
	return c
}

// RandomBytes returns a sequence of random bits for a given level.
// The first argument is irrelevant for this random source.
// It returns nil when the dag hasn't reached level+1 yet.
func (c *coin) RandomBytes(_ uint16, level int) []byte {
	return c.randomBytes.Get(level)
}

func (c *coin) update(u gomel.Unit) {
	if c.shareProviders[u.Creator()] {
		cs := new(tss.Share)
		offset := bn256.SignatureLength
		if gomel.Dealing(u) {
			// dealing units doesn't contain random data from previous level
			offset = 0
		}
		cs.Unmarshal(u.RandomSourceData()[offset:])
		c.coinShares.Add(u.Hash(), cs)
	}
	if !gomel.Dealing(u) {
		c.randomBytes.AppendOrIgnore(u.Level()-1, u.RandomSourceData()[:bn256.SignatureLength])
	}
}

// checkCompliance checks if the random source data included in the unit
// is correct. The following rules should be satisfied:
//  (1) A dealing unit created by a share providers should contain a marshalled share
//  (2) A non-dealing prime unit should start with random bytes from the previous level,
//  followed by a marshalled coin share, if the creator is a share provider.
//  (3) Every other unit's random source data should be empty.
func (c *coin) checkCompliance(u gomel.Unit, _ gomel.Dag) error {
	if gomel.Dealing(u) && c.shareProviders[u.Creator()] {
		return new(tss.Share).Unmarshal(u.RandomSourceData())
	}

	if !gomel.Dealing(u) {
		if len(u.RandomSourceData()) < bn256.SignatureLength {
			return errors.New("random source data too short")
		}

		uRandomBytes := u.RandomSourceData()[:bn256.SignatureLength]
		if rb := c.randomBytes.Get(u.Level() - 1); rb != nil {
			if subtle.ConstantTimeCompare(rb, uRandomBytes) != 1 {
				return errors.New("incorrect random bytes")
			}
		} else {
			coin := new(tss.Signature)
			err := coin.Unmarshal(uRandomBytes)
			if err != nil {
				return err
			}
			if !c.wtk.VerifySignature(coin, nonce(u.Level()-1, u.EpochID())) {
				return errors.New("incorrect random bytes")
			}
		}

		if c.shareProviders[u.Creator()] {
			err := new(tss.Share).Unmarshal(u.RandomSourceData()[bn256.SignatureLength:])
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
// with the given level and set of parents.
// The coin shares from the previous level will be combined.
// If the shares don't combine to the correct random bytes for previous level
// it returns an error. This means that someone had included a wrong coin share
// and we should start an alert.
func (c *coin) DataToInclude(parents []gomel.Unit, level int) ([]byte, error) {
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
	if c.shareProviders[c.pid] {
		rb = append(rb, c.wtk.CreateShare(nonce(level, c.dag.EpochID())).Marshal()...)
	}
	return rb, nil
}

func (c *coin) combineShares(level int) ([]byte, error) {
	shares := []*tss.Share{}
	shareCollected := make(map[uint16]bool)

	su := c.dag.UnitsOnLevel(level)
	if su == nil {
		return nil, errors.New("no primes on a given level")
	}
	su.Iterate(func(units []gomel.Unit) bool {
		for _, v := range units {
			if !c.shareProviders[v.Creator()] || shareCollected[v.Creator()] {
				continue
			}
			cs := c.coinShares.Get(v.Hash())
			if cs != nil {
				shares = append(shares, cs)
				shareCollected[v.Creator()] = true
				if len(shares) == int(c.wtk.Threshold()) {
					return false
				}
				return true
			}
		}
		return true
	})

	coin, ok := c.wtk.CombineShares(shares)
	if !ok {
		return nil, errors.New("combining shares failed")
	}
	if !c.wtk.VerifySignature(coin, nonce(level, c.dag.EpochID())) {
		return nil, errors.New("verification of coin failed")
	}
	return coin.Marshal(), nil
}
