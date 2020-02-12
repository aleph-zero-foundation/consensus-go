package tcoin_test

import (
	"gitlab.com/alephledger/consensus-go/pkg/crypto/encrypt"
	"gitlab.com/alephledger/consensus-go/pkg/crypto/p2p"
	. "gitlab.com/alephledger/consensus-go/pkg/crypto/tcoin"
	"gitlab.com/alephledger/core-go/pkg/crypto/bn256"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Tcoin", func() {
	var (
		n, t, dealer uint16
		nonce        int64
		tcs          []*ThresholdCoin
		coinShares   []*CoinShare
		sKeys        []*p2p.SecretKey
		pKeys        []*p2p.PublicKey
		p2pKeys      [][]encrypt.SymmetricKey
	)
	Context("Between small number of processes", func() {
		Describe("Coin shares", func() {
			BeforeEach(func() {
				n, t, dealer = 10, 3, 5

				gtc := NewRandomGlobal(n, t)
				tcs = make([]*ThresholdCoin, n)
				sKeys = make([]*p2p.SecretKey, n)
				pKeys = make([]*p2p.PublicKey, n)
				p2pKeys = make([][]encrypt.SymmetricKey, n)
				for i := uint16(0); i < n; i++ {
					pKeys[i], sKeys[i], _ = p2p.GenerateKeys()
				}
				for i := uint16(0); i < n; i++ {
					p2pKeys[i], _ = p2p.Keys(sKeys[i], pKeys, i)
				}
				tc, err := gtc.Encrypt(p2pKeys[dealer])
				Expect(err).NotTo(HaveOccurred())
				tcEncoded := tc.Encode()
				for i := uint16(0); i < n; i++ {
					tcs[i], _, err = Decode(tcEncoded, dealer, i, p2pKeys[i][dealer])
					Expect(err).NotTo(HaveOccurred())
				}
				nonce = 123
				coinShares = make([]*CoinShare, n)
				for i := uint16(0); i < n; i++ {
					coinShares[i] = tcs[i].CreateCoinShare(nonce)
				}
			})
			It("should be verified correctly", func() {
				Expect(tcs[2].VerifyCoinShare(coinShares[1], nonce)).To(BeTrue())
				Expect(tcs[2].VerifyCoinShare(coinShares[1], nonce+1)).To(BeFalse())
			})
			It("Should be correctly combined by t-parties", func() {
				c, ok := tcs[0].CombineCoinShares(coinShares[:t])
				Expect(ok).To(BeTrue())
				Expect(tcs[0].VerifyCoin(c, nonce)).To(BeTrue())
				Expect(tcs[0].VerifyCoin(c, nonce+1)).To(BeFalse())
			})
			It("Should be correctly combined by more than t-parties", func() {
				c, ok := tcs[0].CombineCoinShares(coinShares)
				Expect(ok).To(BeTrue())
				Expect(tcs[0].VerifyCoin(c, nonce)).To(BeTrue())
				Expect(tcs[0].VerifyCoin(c, nonce+1)).To(BeFalse())
			})
			It("Should be combined to the same coin by two different sets of t-parties", func() {
				c1, ok := tcs[0].CombineCoinShares(coinShares[:t])
				Expect(ok).To(BeTrue())
				c2, ok := tcs[0].CombineCoinShares(coinShares[(n - t):])
				Expect(ok).To(BeTrue())
				Expect(c1.RandomBytes()).To(Equal(c2.RandomBytes()))
				Expect(c1.Toss()).To(Equal(c2.Toss()))
				Expect(c1.Toss()).To(SatisfyAny(Equal(0), Equal(1)))
			})
			It("Shouldn't be correctly combined by t-1-parties", func() {
				_, ok := tcs[0].CombineCoinShares(coinShares[:(t - 1)])
				Expect(ok).To(BeFalse())
			})
			It("Should be marshalled and unmarshalled correctly", func() {
				for i := uint16(0); i < n; i++ {
					csMarshalled := coinShares[i].Marshal()
					var cs = new(CoinShare)
					err := cs.Unmarshal(csMarshalled)
					Expect(err).NotTo(HaveOccurred())
					Expect(tcs[0].VerifyCoinShare(cs, nonce)).To(BeTrue())
				}
			})
		})
	})
	Context("Coin unmarshal", func() {
		Context("On an empty slice", func() {
			It("Should return an error", func() {
				c := new(Coin)
				err := c.Unmarshal([]byte{})
				Expect(err).To(HaveOccurred())
			})
		})
		Context("On a incorrect slice having correct length", func() {
			It("Should return an error", func() {
				c := new(Coin)
				data := make([]byte, bn256.SignatureLength)
				data[0] = 1
				err := c.Unmarshal(data)
				Expect(err).To(HaveOccurred())
			})
		})
		Context("On a correctly marshalled coin", func() {
			It("Should work without errors", func() {
				c := new(Coin)

				_, priv, err := bn256.GenerateKeys()
				Expect(err).NotTo(HaveOccurred())
				data := []byte{1, 2, 3}

				err = c.Unmarshal(priv.Sign(data).Marshal())
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Context("Multicoin", func() {
		var (
			tcs1 []*ThresholdCoin
			tcs2 []*ThresholdCoin
		)
		BeforeEach(func() {
			n, t = 10, 4

			tcs = make([]*ThresholdCoin, n)
			tcs1 = make([]*ThresholdCoin, n)
			tcs2 = make([]*ThresholdCoin, n)
			sKeys = make([]*p2p.SecretKey, n)
			pKeys = make([]*p2p.PublicKey, n)
			p2pKeys = make([][]encrypt.SymmetricKey, n)
			for i := uint16(0); i < n; i++ {
				pKeys[i], sKeys[i], _ = p2p.GenerateKeys()
			}
			for i := uint16(0); i < n; i++ {
				p2pKeys[i], _ = p2p.Keys(sKeys[i], pKeys, i)
			}

			gtc1 := NewRandomGlobal(n, t)
			tc1, _ := gtc1.Encrypt(p2pKeys[0])
			gtc2 := NewRandomGlobal(n, t)
			tc2, _ := gtc2.Encrypt(p2pKeys[1])

			tc1Encoded := tc1.Encode()
			tc2Encoded := tc2.Encode()
			for i := uint16(0); i < n; i++ {
				tcs1[i], _, _ = Decode(tc1Encoded, 0, i, p2pKeys[i][0])
				tcs2[i], _, _ = Decode(tc2Encoded, 1, i, p2pKeys[i][1])
				tcs[i] = CreateMulticoin([]*ThresholdCoin{tcs1[i], tcs2[i]})
			}
			nonce = 123
			coinShares = make([]*CoinShare, n)
			for i := uint16(0); i < n; i++ {
				coinShares[i] = tcs[i].CreateCoinShare(nonce)
			}
		})
		Describe("coin shares", func() {
			It("should be the sum of coin shares among single coins", func() {
				cs := SumShares([]*CoinShare{tcs1[0].CreateCoinShare(nonce), tcs2[0].CreateCoinShare(nonce)})
				Expect(cs.Marshal()).To(Equal(coinShares[0].Marshal()))
			})
		})
	})
})
