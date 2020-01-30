package tcoin

import (
	"encoding/binary"
	"errors"
	"math/big"
	"sync"

	"gitlab.com/alephledger/core-go/pkg/crypto/bn256"
)

// Marshal returns byte representation of the given coin share in the following form
// (1) owner, 2 bytes as uint16
// (2) signature
func (cs *CoinShare) Marshal() []byte {
	data := make([]byte, 2)
	binary.LittleEndian.PutUint16(data[:2], cs.owner)
	data = append(data, cs.sgn.Marshal()...)
	return data
}

// Unmarshal reads a coin share from its byte representation.
func (cs *CoinShare) Unmarshal(data []byte) error {
	if len(data) < 2 {
		return errors.New("given data is too short")
	}
	owner := binary.LittleEndian.Uint16(data[:2])
	sgn := data[2:]
	cs.owner = owner
	decSgn, err := new(bn256.Signature).Unmarshal(sgn)
	if err != nil {
		return err
	}
	cs.sgn = decSgn
	return nil
}

// Unmarshal creates a coin from its byte representation.
func (c *Coin) Unmarshal(data []byte) error {
	if len(data) != bn256.SignatureLength {
		return errors.New("unmarshalling of coin failed. Wrong data length")
	}
	sgn := new(bn256.Signature)
	sgn, err := sgn.Unmarshal(data)
	if err != nil {
		return err
	}
	c.sgn = sgn
	return nil
}

// RandomBytes returns randomBytes from the coin.
func (c *Coin) RandomBytes() []byte {
	return c.sgn.Marshal()
}

// Toss returns a pseduorandom bit from a coin.
func (c *Coin) Toss() int {
	return int(c.sgn.Marshal()[0] & 1)
}

// CreateCoinShare creates a CoinShare for given process and nonce.
func (tc *ThresholdCoin) CreateCoinShare(nonce int) *CoinShare {
	return &CoinShare{
		owner: tc.owner,
		sgn:   tc.sk.Sign(big.NewInt(int64(nonce)).Bytes()),
	}
}

// CombineCoinShares combines the given shares into a Coin.
// It returns a Coin and a bool value indicating whether combining was successful or not.
func (tc *ThresholdCoin) CombineCoinShares(shares []*CoinShare) (*Coin, bool) {
	if uint16(len(shares)) > tc.threshold {
		shares = shares[:tc.threshold]
	}
	if tc.threshold != uint16(len(shares)) {
		return nil, false
	}
	var points []int64
	for _, sh := range shares {
		points = append(points, int64(sh.owner))
	}

	var sum *bn256.Signature
	summands := make(chan *bn256.Signature)

	var wg sync.WaitGroup
	for _, sh := range shares {
		wg.Add(1)
		go func(ch *CoinShare) {
			defer wg.Done()
			summands <- bn256.MulSignature(ch.sgn, lagrange(points, int64(ch.owner)))
		}(sh)
	}
	go func() {
		wg.Wait()
		close(summands)
	}()

	for elem := range summands {
		sum = bn256.AddSignatures(sum, elem)
	}

	return &Coin{sgn: sum}, true
}
