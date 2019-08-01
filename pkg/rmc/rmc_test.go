package rmc_test

import (
	"io"
	"sync"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"gitlab.com/alephledger/consensus-go/pkg/crypto/bn256"
	. "gitlab.com/alephledger/consensus-go/pkg/rmc"
)

var _ = Describe("Rmc", func() {
	var (
		protos  []*Protocol
		data    []byte
		readers [][]io.Reader
		writers [][]io.Writer
		n       int
	)
	BeforeEach(func() {
		data = []byte("19890604")
		n = 10
		pubs := make([]*bn256.VerificationKey, n)
		privs := make([]*bn256.SecretKey, n)
		protos = make([]*Protocol, n)
		readers = make([][]io.Reader, n)
		writers = make([][]io.Writer, n)
		for i := range pubs {
			var err error
			pubs[i], privs[i], err = bn256.GenerateKeys()
			Expect(err).NotTo(HaveOccurred())
		}
		for i := range protos {
			protos[i] = New(pubs, privs[i])
			readers[i] = make([]io.Reader, n)
			writers[i] = make([]io.Writer, n)
			for j := range readers[i] {
				readers[i][j], writers[i][j] = io.Pipe()
			}
		}
	})
	CorrectCast := func(myPid uint16, id uint64) {
		defer GinkgoRecover()
		proto := protos[myPid]
		for i := range protos {
			if uint16(i) == myPid {
				continue
			}
			Expect(proto.SendData(id, data, writers[myPid][i])).To(Succeed())
			_, err := proto.AcceptSignature(id, uint16(i), readers[i][myPid])
			Expect(err).NotTo(HaveOccurred())
		}
		for i := range protos {
			if uint16(i) == myPid {
				continue
			}
			Expect(proto.SendProof(id, writers[myPid][i])).To(Succeed())
		}
	}
	CorrectReceive := func(myPid, otherPid uint16, id uint64) {
		defer GinkgoRecover()
		proto := protos[myPid]
		locData, err := proto.AcceptData(id, otherPid, readers[otherPid][myPid])
		Expect(err).NotTo(HaveOccurred())
		Expect(locData).To(Equal(data))
		Expect(proto.SendSignature(id, writers[myPid][otherPid])).To(Succeed())
		Expect(proto.AcceptProof(id, readers[otherPid][myPid])).To(Succeed())
	}
	It("Should successfully multicast", func() {
		var wg sync.WaitGroup
		id := uint64(21037)
		wg.Add(1)
		go func() {
			defer wg.Done()
			CorrectCast(0, id)
		}()
		for i := range protos {
			if i == 0 {
				continue
			}
			wg.Add(1)
			go func(i uint16) {
				defer wg.Done()
				CorrectReceive(i, 0, id)
			}(uint16(i))
		}
		wg.Wait()
	})
})
