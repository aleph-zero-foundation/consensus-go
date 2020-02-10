package config

import (
	"gitlab.com/alephledger/consensus-go/pkg/gomel"

	"bytes"
	"encoding/json"
	"io"
	"reflect"
)

// ParamsLoader is an abstraction for parsing configurations from a given io.Reader instance.
type ParamsLoader interface {
	// LoadParams parses an instance of the Params type using a given instance of io.Reader.
	LoadParams(io.Reader, *Params) error
}

// ParamsWriter is an abstraction for storing configurations using a given instance of io.Writer.
type ParamsWriter interface {
	// StoreParams outputs a representation of the Params using the provided io.Writer.
	StoreParams(io.Writer, *Params) error
}

type jsonConfigLoader struct{}

func (l jsonConfigLoader) LoadParams(reader io.Reader, config *Params) error {
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
	// check if the provided JSON representation has the same number of fields as the Params type
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

func (l jsonConfigLoader) StoreParams(writer io.Writer, config *Params) error {
	return json.NewEncoder(writer).Encode(*config)
}

// NewJSONConfigLoader returns a new instance of the ParamsLoader type that expects that the provided configuration
// is stored using the JSON format.
func NewJSONConfigLoader() ParamsLoader {
	return jsonConfigLoader{}
}

// NewJSONConfigWriter returns a new instance of the ParamsWriter type that stores the configuration using the JSON
// format.
func NewJSONConfigWriter() ParamsWriter {
	return jsonConfigLoader{}
}
