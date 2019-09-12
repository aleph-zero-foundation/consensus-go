package tcoin_test

import (
	. "gitlab.com/alephledger/consensus-go/pkg/crypto/tcoin"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Tcoin", func() {
	var n, t uint16
	var nonce int
	var tcs []*ThresholdCoin
	var coinShares []*CoinShare
	Context("Between small number of processes", func() {
		Describe("Coin shares", func() {
			BeforeEach(func() {
				n, t = 10, 3
				dealt := Deal(n, t)
				tcs = make([]*ThresholdCoin, n)
				for i := uint16(0); i < n; i++ {
					tc, err := Decode(dealt, i)
					Expect(err).NotTo(HaveOccurred())
					tcs[i] = tc
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
})
