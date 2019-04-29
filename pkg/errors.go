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
}

// Error returns a (fixed) string description of a DuplicateUnit.
func (e *DuplicateUnit) Error() string {
	return "Unit already in poset."
}
