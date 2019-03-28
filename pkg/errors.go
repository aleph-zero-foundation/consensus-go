package alephzero

type DataError struct {
	msg string
}

func (e *DataError) Error() string {
	return "DataError: " + e.msg
}

func NewDataError(msg string) *DataError {
	return &DataError{msg}
}

type ComplianceError struct {
	msg string
}

func (e *ComplianceError) Error() string {
	return "ComplianceError: " + e.msg
}

func NewComplianceError(msg string) *ComplianceError {
	return &ComplianceError{msg}
}

type DuplicateUnit struct {
}

func (e *DuplicateUnit) Error() string {
	return "Unit already in poset."
}
