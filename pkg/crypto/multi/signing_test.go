package multi_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"gitlab.com/alephledger/consensus-go/pkg/crypto/bn256"
	. "gitlab.com/alephledger/consensus-go/pkg/crypto/multi"
)

var _ = Describe("Signing", func() {
	var (
		keys []*Keychain
		n    int
	)
	BeforeEach(func() {
		n = 10
		keys = make([]*Keychain, n)
		privs := make([]*bn256.SecretKey, n)
		pubs := make([]*bn256.VerificationKey, n)
		for i := range keys {
			var err error
			pubs[i], privs[i], err = bn256.GenerateKeys()
			Expect(err).NotTo(HaveOccurred())
		}
		for i := range keys {
			keys[i] = NewKeychain(pubs, privs[i])
		}
	})
	Describe("Data", func() {
		var (
			data []byte
		)
		BeforeEach(func() {
			data = []byte("19890604")
		})
		It("should sign and verify correctly", func() {
			ks := keys[0]
			sgn := ks.Sign(data)
			Expect(ks.Verify(0, append(data, sgn...))).To(BeTrue())
		})
		Context("With multisignatures", func() {
			var (
				multisig  *Signature
				threshold uint16
			)
			BeforeEach(func() {
				threshold = uint16(n - n/3)
				multisig = NewSignature(int(threshold), data)
			})
			It("should not verify without any signatures aggregated", func() {
				Expect(keys[0].MultiVerify(multisig)).To(BeFalse())
			})
			It("should not verify with only one signature aggregated", func() {
				done, err := multisig.Aggregate(0, keys[0].Sign(data))
				Expect(done).To(BeFalse())
				Expect(err).NotTo(HaveOccurred())
				Expect(keys[0].MultiVerify(multisig)).To(BeFalse())
			})
			It("should verify with threshold signatures aggregated", func() {
				for i := uint16(0); i < threshold-1; i++ {
					done, err := multisig.Aggregate(i, keys[i].Sign(data))
					Expect(err).NotTo(HaveOccurred())
					Expect(done).To(BeFalse())
				}
				done, err := multisig.Aggregate(threshold, keys[threshold].Sign(data))
				Expect(err).NotTo(HaveOccurred())
				Expect(done).To(BeTrue())
				Expect(keys[0].MultiVerify(multisig)).To(BeTrue())
			})
			It("should verify after all signatures aggregated", func() {
				for i := uint16(0); i < uint16(n); i++ {
					multisig.Aggregate(i, keys[i].Sign(data))
				}
				Expect(keys[0].MultiVerify(multisig)).To(BeTrue())
			})
			It("should verify after all signatures aggregated with marshaling/unmarshaling", func() {
				for i := uint16(0); i < uint16(n); i++ {
					multisig.Aggregate(i, keys[i].Sign(data))
				}
				mlsgn, err := NewSignature(int(threshold), data).Unmarshal(multisig.Marshal())
				Expect(err).NotTo(HaveOccurred())
				Expect(keys[0].MultiVerify(mlsgn)).To(BeTrue())
			})
			It("should not verify with one signature aggregated multiple times", func() {
				for i := uint16(0); i < uint16(n); i++ {
					multisig.Aggregate(0, keys[0].Sign(data))
				}
				Expect(keys[0].MultiVerify(multisig)).To(BeFalse())
			})
			It("should not verify with an incorrect signature aggregated", func() {
				for i := uint16(0); i < threshold-1; i++ {
					done, err := multisig.Aggregate(i, keys[i].Sign(data))
					Expect(err).NotTo(HaveOccurred())
					Expect(done).To(BeFalse())
				}
				sgn := keys[threshold].Sign(data)
				sgn[0] = sgn[1]
				done, err := multisig.Aggregate(threshold, sgn)
				Expect(err).To(HaveOccurred())
				Expect(done).To(BeFalse())
				Expect(keys[0].MultiVerify(multisig)).To(BeFalse())
			})
			It("should not verify when signatures aggregated with incorrect pids", func() {
				for i := uint16(0); i < threshold-1; i++ {
					done, err := multisig.Aggregate(i+1, keys[i].Sign(data))
					Expect(err).NotTo(HaveOccurred())
					Expect(done).To(BeFalse())
				}
				done, err := multisig.Aggregate(threshold+1, keys[threshold].Sign(data))
				Expect(err).NotTo(HaveOccurred())
				Expect(done).To(BeTrue())
				Expect(keys[0].MultiVerify(multisig)).To(BeFalse())
			})
		})
	})

})
