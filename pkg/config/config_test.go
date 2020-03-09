package config_test

import (
	"bufio"
	"io/ioutil"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "gitlab.com/alephledger/consensus-go/pkg/config"

	"bytes"
)

var _ = Describe("Config", func() {
	Describe("committee member", func() {
		Describe("When loaded from a test file ", func() {
			It("Should work without errors", func() {
				file, err := os.Open("../testdata/test_pk.txt")
				defer file.Close()
				Expect(err).NotTo(HaveOccurred())
				reader := bufio.NewReader(file)
				_, err = LoadMember(reader)
				Expect(err).NotTo(HaveOccurred())
			})
		})
		Describe("When loaded from a test file and stored in a buffer", func() {
			var member *Member
			var fileContent []byte
			BeforeEach(func() {
				file, err := os.Open("../testdata/test_pk.txt")
				defer file.Close()
				Expect(err).NotTo(HaveOccurred())

				reader := bufio.NewReader(file)
				member, err = LoadMember(reader)
				Expect(err).NotTo(HaveOccurred())

				fileContent, err = ioutil.ReadFile("../testdata/test_pk.txt")
				Expect(err).NotTo(HaveOccurred())
			})
			It("Should have the same content", func() {
				buf := bytes.NewBuffer([]byte{})
				err := StoreMember(buf, member)
				Expect(err).NotTo(HaveOccurred())
				Expect(buf.Bytes()).To(Equal(fileContent))
			})
		})
	})
	Describe("committee", func() {
		Describe("When loaded from a test file ", func() {
			It("Should work without errors", func() {
				file, err := os.Open("../testdata/test_committee.txt")
				defer file.Close()
				Expect(err).NotTo(HaveOccurred())
				reader := bufio.NewReader(file)
				_, err = LoadCommittee(reader)
				Expect(err).NotTo(HaveOccurred())
			})
		})
		Describe("When loaded from a test file and stored in a buffer", func() {
			var committee *Committee
			var fileContent []byte
			BeforeEach(func() {
				file, err := os.Open("../testdata/test_committee.txt")
				defer file.Close()
				Expect(err).NotTo(HaveOccurred())

				reader := bufio.NewReader(file)
				committee, err = LoadCommittee(reader)
				Expect(err).NotTo(HaveOccurred())

				fileContent, err = ioutil.ReadFile("../testdata/test_committee.txt")
				Expect(err).NotTo(HaveOccurred())
			})
			It("Should have the same content", func() {
				buf := bytes.NewBuffer([]byte{})
				err := StoreCommittee(buf, committee)
				Expect(err).NotTo(HaveOccurred())
				Expect(buf.Bytes()).To(Equal(fileContent))
			})
		})
	})
	Describe("defaults", func() {
		var (
			m   *Member
			c   *Committee
			cnf Config
		)
		BeforeEach(func() {
			file, err := os.Open("../testdata/test_pk.txt")
			defer file.Close()
			Expect(err).NotTo(HaveOccurred())
			reader := bufio.NewReader(file)
			m, err = LoadMember(reader)
			Expect(err).NotTo(HaveOccurred())

			file, err = os.Open("../testdata/test_committee.txt")
			defer file.Close()
			Expect(err).NotTo(HaveOccurred())
			reader = bufio.NewReader(file)
			c, err = LoadCommittee(reader)
			Expect(err).NotTo(HaveOccurred())
		})
		It("should generate valid setup config", func() {
			cnf = NewSetup(m, c)
			err := Valid(cnf, true)
			Expect(err).NotTo(HaveOccurred())
		})
		It("should generate valid consensus config", func() {
			cnf = New(m, c)
			err := Valid(cnf, false)
			Expect(err).NotTo(HaveOccurred())
		})

	})
})
