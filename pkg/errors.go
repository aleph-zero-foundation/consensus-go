package gomel

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

// UnknownParent is raised when at least one of the parents of a preunit cannot be found in the dag.
// Depending on the syncing protocol this might or might not indicate problems with the process providing the preunit.
type UnknownParent struct{}

// Error returns a (fixed) string description of an UnknownParent.
func (e *UnknownParent) Error() string {
	return "Unknown parent."
}

// NewUnknownParent constructs an UnknownParent.
func NewUnknownParent() *UnknownParent {
	return &UnknownParent{}
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
