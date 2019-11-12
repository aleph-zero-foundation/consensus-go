package gomel

import "strings"

// DataError represents incorrect data received from a process.
// Indicates a problem with the process providing the data.
type DataError struct {
	msg string
}

// Error returns a string description of a DataError.
func (e *DataError) Error() string {
	return "DataError: " + e.msg
}

// NewDataError constructs a DataError from a given msg.
func NewDataError(msg string) *DataError {
	return &DataError{msg}
}

// ComplianceError is raised when encountering a unit that does not follow compliance rules.
// Indicates a problem with both the process providing the data and the unit's creator.
type ComplianceError struct {
	msg string
}

// Error returns a string description of a ComplianceError.
func (e *ComplianceError) Error() string {
	return "ComplianceError: " + e.msg
}

// NewComplianceError constructs a ComplianceError from a given msg.
func NewComplianceError(msg string) *ComplianceError {
	return &ComplianceError{msg}
}

// MissingDataError represents missing data needed to accept a unit.
// Examples are commitments to forks or proofs of successful RMCs.
type MissingDataError struct {
	msg string
}

// Error returns a string description of a MissingDataError.
func (e *MissingDataError) Error() string {
	return "MissingDataError: " + e.msg
}

// NewMissingDataError constructs a MissingDataError from a given msg.
func NewMissingDataError(msg string) *MissingDataError {
	return &MissingDataError{msg}
}

// DuplicateUnit is an error-like object used when encountering a unit that is already known. Usually not a problem.
type DuplicateUnit struct {
	Unit Unit
}

// Error returns a (fixed) string description of a DuplicateUnit.
func (e *DuplicateUnit) Error() string {
	return "Unit already in dag."
}

// NewDuplicateUnit constructs a DuplicateUnit error for the given unit.
func NewDuplicateUnit(unit Unit) *DuplicateUnit {
	return &DuplicateUnit{unit}
}

// DuplicatePreunit is an error-like object used when encountering a unit that is already known. Usually not a problem.
type DuplicatePreunit struct {
	Pu Preunit
}

// Error returns a (fixed) string description of a DuplicatePreunit.
func (e *DuplicatePreunit) Error() string {
	return "Unit already in dag."
}

// NewDuplicatePreunit constructs a DuplicatePreunit error for the given preunit.
func NewDuplicatePreunit(pu Preunit) *DuplicatePreunit {
	return &DuplicatePreunit{pu}
}

// UnknownParents is an error-like object used when trying to add a unit whose parents are not in the dag.
type UnknownParents struct {
	Amount int
}

// Error returns a (fixed) string description of a UnknownParents.
func (e *UnknownParents) Error() string {
	return "Unknown parents"
}

// NewUnknownParents constructs a UnknownParents error for the given unit.
func NewUnknownParents(howMany int) *UnknownParents {
	return &UnknownParents{howMany}
}

// ConfigError is returned when a provided configuration can not be parsed.
type ConfigError struct {
	msg string
}

func (e *ConfigError) Error() string {
	return "ConfigError: " + e.msg
}

// NewConfigError constructs a ConfigError from a given msg.
func NewConfigError(msg string) *ConfigError {
	return &ConfigError{msg}
}

// AggregateError represents a set of errors returned from adding an antichain of units.
type AggregateError struct {
	errs []error
}

// NewAggregateError using the given slice of errors.
func NewAggregateError(errs []error) *AggregateError {
	return &AggregateError{errs}
}

func (ae *AggregateError) Error() string {
	var result strings.Builder
	for _, e := range ae.errs {
		if e != nil {
			result.WriteString(e.Error())
			result.WriteRune('\n')
		}
	}
	return result.String()
}

// Errors returns a slice of all the aggregated errors, in the same order as the preunits added.
func (ae *AggregateError) Errors() []error {
	return ae.errs
}

// AmbiguousParents is an error-like object used when trying to add
// a unit whose parents cannot be determined by pid and height.
// It contains all possible parents sliced by creator.
type AmbiguousParents struct {
	Units [][]Unit
}

func (e *AmbiguousParents) Error() string {
	return "Ambiguous parents"
}

// NewAmbiguousParents constructs an AmbigousParents error for a given process.
func NewAmbiguousParents(units [][]Unit) *AmbiguousParents {
	return &AmbiguousParents{units}
}
