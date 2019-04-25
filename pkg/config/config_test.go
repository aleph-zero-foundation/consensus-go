package config_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "gitlab.com/alephledger/consensus-go/pkg/config"

	"bytes"
	"strings"
)

var _ = Describe("Configuration", func() {
	Describe("json configuration", func() {
		Describe("Store and Load Configuration", func() {
			It("should return same Configuration", func() {
				config := NewDefaultConfiguration()
				config.N_PARENTS = 10000
				configCopy := config

				// store configuation using a buffer
				var buf bytes.Buffer
				err := NewJSONConfigWriter().StoreConfiguration(&buf, &config)
				Expect(err).NotTo(HaveOccurred())

				// load the configuration from the buffer
				var newConfiguration Configuration
				err = NewJSONConfigLoader().LoadConfiguration(&buf, &newConfiguration)
				Expect(err).NotTo(HaveOccurred())
				Expect(newConfiguration).To(Equal(configCopy))
			})
		})

		Describe("parsing incomplete JSON configuration", func() {
			It("should return an error", func() {
				jsonConfig := "{\"N_PARENTS\": 1000}"
				configStream := strings.NewReader(jsonConfig)

				var config Configuration
				err := NewJSONConfigLoader().LoadConfiguration(configStream, &config)
				Expect(err).To(HaveOccurred())
			})
		})

		Describe("configuration with non-existent field", func() {
			It("should return an error", func() {
				jsonConfig := "{\"BLA_BLA\": 1000}"
				configStream := strings.NewReader(jsonConfig)

				var config Configuration
				err := NewJSONConfigLoader().LoadConfiguration(configStream, &config)
				Expect(err).To(HaveOccurred())
			})
		})

		Describe("broken configuration", func() {
			It("should return an error", func() {
				jsonConfig := "adasdjiojoi  a{ aaa/"
				configStream := strings.NewReader(jsonConfig)

				var config Configuration
				err := NewJSONConfigLoader().LoadConfiguration(configStream, &config)
				Expect(err).To(HaveOccurred())
			})
		})

		Describe("configuration with nil value", func() {
			It("should be parsed correctly", func() {
				config := NewDefaultConfiguration()
				config.UNITS_LIMIT = nil
				config.SYNCS_LIMIT = Value(10)
				configCopy := config

				// store configuation using a buffer
				var buf bytes.Buffer
				err := NewJSONConfigWriter().StoreConfiguration(&buf, &config)
				Expect(err).NotTo(HaveOccurred())

				// load the configuration from the buffer
				var newConfiguration Configuration
				err = NewJSONConfigLoader().LoadConfiguration(&buf, &newConfiguration)
				Expect(err).NotTo(HaveOccurred())
				Expect(newConfiguration).To(Equal(configCopy))
			})
		})
	})
})
