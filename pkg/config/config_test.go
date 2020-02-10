package config_test

import (
	"bufio"
	"io/ioutil"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "gitlab.com/alephledger/consensus-go/pkg/config"

	"bytes"
	"strings"
)

var _ = Describe("Params", func() {
	Describe("json configuration", func() {
		Describe("Store and Load Params", func() {
			It("should return same Params", func() {
				config := NewDefaultParams()
				config.LevelLimit = 10000
				configCopy := config

				// store configuration using a buffer
				var buf bytes.Buffer
				err := NewJSONConfigWriter().StoreParams(&buf, &config)
				Expect(err).NotTo(HaveOccurred())

				// load the configuration from the buffer
				var newParams Params
				err = NewJSONConfigLoader().LoadParams(&buf, &newParams)
				Expect(err).NotTo(HaveOccurred())
				Expect(newParams).To(Equal(configCopy))
			})
		})

		Describe("parsing incomplete JSON configuration", func() {
			It("should return an error", func() {
				jsonConfig := "{\"LevelLimit\": 1000}"
				configStream := strings.NewReader(jsonConfig)

				var config Params
				err := NewJSONConfigLoader().LoadParams(configStream, &config)
				Expect(err).To(HaveOccurred())
			})
		})

		Describe("configuration with non-existent field", func() {
			It("should return an error", func() {
				jsonConfig := "{\"BlaBla\": 1000}"
				configStream := strings.NewReader(jsonConfig)

				var config Params
				err := NewJSONConfigLoader().LoadParams(configStream, &config)
				Expect(err).To(HaveOccurred())
			})
		})

		Describe("broken configuration", func() {
			It("should return an error", func() {
				jsonConfig := "adasdjiojoi  a{ aaa/"
				configStream := strings.NewReader(jsonConfig)

				var config Params
				err := NewJSONConfigLoader().LoadParams(configStream, &config)
				Expect(err).To(HaveOccurred())
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
})
