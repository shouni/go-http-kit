package retry

import (
	"errors"
	"fmt"
)

// Sentinel errors. Callers can tell the reason for a failure with errors.Is.
//
// Following Go convention, error messages are English, lowercase and without
// trailing punctuation.
var (
	// ErrNilOperation is returned when a nil Operation is passed.
	ErrNilOperation = errors.New("retry: operation is nil")

	// ErrExhausted reports that the maximum number of attempts was reached without success.
	ErrExhausted = errors.New("retry: attempts exhausted")

	// ErrPermanent reports that retrying stopped because ShouldRetryFunc returned false.
	ErrPermanent = errors.New("retry: permanent error")

	// ErrRetryAfterTooLong reports that retrying stopped because the wait a
	// DelayHinter requested exceeded the WithMaxRetryAfter cap. Error.Permanent
	// is also true.
	ErrRetryAfterTooLong = errors.New("retry: server-requested delay exceeds the limit")
)

// Error is the typed error returned when retrying fails.
//
// Unwrap returns several errors, so all of the following checks work:
//
//	errors.Is(err, retry.ErrExhausted)      // reached the maximum number of attempts
//	errors.Is(err, retry.ErrPermanent)      // judged not retryable
//	errors.Is(err, context.DeadlineExceeded) // the context timed out
//	errors.Is(err, myOperationError)         // the last error the operation itself returned
type Error struct {
	// Op is the operation name. When empty it is shown as "operation".
	Op string
	// Attempts is the number of times the operation actually ran (including the first).
	Attempts uint
	// Permanent reports whether retrying stopped because ShouldRetryFunc returned false.
	Permanent bool
	// Err is the last error the operation returned.
	Err error
	// Cause holds the reason the context ended, if it did.
	// It is nil when retrying ended for a reason other than the context.
	Cause error
}

func (e *Error) Error() string {
	switch {
	case e.Permanent:
		return fmt.Sprintf("retry: %s aborted after %d attempt(s): permanent error: %v",
			e.opName(), e.Attempts, e.Err)
	case e.Cause != nil:
		return fmt.Sprintf("retry: %s aborted after %d attempt(s): %v (last error: %v)",
			e.opName(), e.Attempts, e.Cause, e.Err)
	default:
		return fmt.Sprintf("retry: %s failed after %d attempt(s): %v",
			e.opName(), e.Attempts, e.Err)
	}
}

// Unwrap returns the operation error, the context cause and the classifying sentinel together.
func (e *Error) Unwrap() []error {
	errs := make([]error, 0, 3)
	if e.Err != nil {
		errs = append(errs, e.Err)
	}
	if e.Cause != nil {
		errs = append(errs, e.Cause)
	}
	switch {
	case e.Permanent:
		errs = append(errs, ErrPermanent)
	case e.Cause == nil:
		errs = append(errs, ErrExhausted)
	}
	return errs
}

func (e *Error) opName() string {
	if e.Op == "" {
		return "operation"
	}
	return e.Op
}
