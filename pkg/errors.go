package gomel

// An error representing incorrect data received from a process.
// Indicates a problem with the process providing the data.
type DataError struct {
	msg string
}

func (e *DataError) Error() string {
	return "DataError: " + e.msg
}

func NewDataError(msg string) *DataError {
	return &DataError{msg}
}

// An error representing encountering a noncompliant unit.
// Indicates a problem with both the process providing the data and the unit's creator.
type ComplianceError struct {
	msg string
}

func (e *ComplianceError) Error() string {
	return "ComplianceError: " + e.msg
}

func NewComplianceError(msg string) *ComplianceError {
	return &ComplianceError{msg}
}

// An error representing encountering a unit that is already known. Usually not a problem.
type DuplicateUnit struct {
}

func (e *DuplicateUnit) Error() string {
	return "Unit already in poset."
}
