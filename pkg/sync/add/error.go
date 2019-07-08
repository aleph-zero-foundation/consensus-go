package add

import (
	"strings"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
)

// AggregateError represents a set of errors returned from adding an antichain of units.
type AggregateError struct {
	errs []error
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

// Pruned returns a version of this error that contains no nil entries and, optionally, no UnknownParents errors.
func (ae *AggregateError) Pruned(ignoreUnknownParents bool) *AggregateError {
	if ae == nil {
		return nil
	}
	var result AggregateError
	for _, e := range ae.errs {
		if e != nil {
			if ignoreUnknownParents {
				if _, ok := e.(*gomel.UnknownParent); !ok {
					result.errs = append(result.errs, e)
				}
			} else {
				result.errs = append(result.errs, e)
			}
		}
	}
	if len(result.errs) == 0 {
		return nil
	}
	return &result
}
