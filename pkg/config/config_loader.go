package config

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"

	"bytes"
	"encoding/json"
	"io"
	"reflect"
)

// ConfigurationLoader is an abstraction for parsing configurations from a given io.Reader instance.
type ConfigurationLoader interface {
	// LoadConfiguration parses instance of the Configuration type using a given instance of io.Reader.
	LoadConfiguration(io.Reader, *Configuration) error
}

// ConfigurationWriter is an abstraction for storing configurations using a given instance of io.Writer.
type ConfigurationWriter interface {
	// StoreConfiguration outputs a human readable representation of the Configuration using provided io.Writer.
	StoreConfiguration(io.Writer, *Configuration) error
}

type jsonConfigLoader struct{}

func (l jsonConfigLoader) LoadConfiguration(reader io.Reader, config *Configuration) error {
	if config == nil {
		return gomel.NewConfigError("config parameter is nil")
	}

	var buffer bytes.Buffer
	decoder := json.NewDecoder(io.TeeReader(reader, &buffer))
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&config)
	if err != nil {
		return err
	}
	// check if the provided JSON representation has the same number of fields as the Configuration type
	var parsedJSON map[string]interface{}
	err = json.NewDecoder(&buffer).Decode(&parsedJSON)
	if err != nil {
		return err
	}
	if reflect.Indirect(reflect.ValueOf(config)).NumField() != len(parsedJSON) {
		return gomel.NewConfigError("Provided configuration has incorrect number of fields")
	}
	return nil
}

func (l jsonConfigLoader) StoreConfiguration(writer io.Writer, config *Configuration) error {
	return json.NewEncoder(writer).Encode(*config)
}

// NewJSONConfigLoader returns a new instance of the ConfigurationLoader type that expects that the provided configuration
// is stored using the JSON format.
func NewJSONConfigLoader() ConfigurationLoader {
	return jsonConfigLoader{}
}

// NewJSONConfigWriter returns a new instance of the ConfigurationWriter type that stores the configuration using the JSON
// format.
func NewJSONConfigWriter() ConfigurationWriter {
	return jsonConfigLoader{}
}
