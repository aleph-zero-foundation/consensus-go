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

type jsonParamsLoader struct{}

func (l jsonParamsLoader) LoadParams(reader io.Reader, config *Params) error {
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

func (l jsonParamsLoader) StoreParams(writer io.Writer, config *Params) error {
	return json.NewEncoder(writer).Encode(*config)
}

// NewJSONParamsLoader returns a new instance of the ParamsLoader type that expects that the provided configuration
// is stored using the JSON format.
func NewJSONParamsLoader() ParamsLoader {
	return jsonParamsLoader{}
}

// NewJSONParamsWriter returns a new instance of the ParamsWriter type that stores the configuration using the JSON
// format.
func NewJSONParamsWriter() ParamsWriter {
	return jsonParamsLoader{}
}
