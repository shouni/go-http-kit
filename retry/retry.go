// Package retry provides utilities for retrying operations with exponential
// backoff and a custom error predicate.
//
// # Basic usage
//
//	err := retry.Run(ctx, func() error { return callAPI() },
//	    retry.WithName("ExternalAPI"),
//	    retry.WithMaxRetries(5),
//	    retry.WithShouldRetry(isTransient),
//	    retry.WithNotify(func(err error, attempt uint, next time.Duration) {
//	        slog.Warn("retrying", "attempt", attempt, "next", next, "err", err)
//	    }),
//	)
//
// Use RunValue for an operation that returns a value.
//
//	body, err := retry.RunValue(ctx, func() ([]byte, error) { return fetch(ctx) },
//	    retry.WithMaxRetries(3),
//	)
//
// Use RunCtx / RunValueCtx to pass the context to the operation.
//
//	err := retry.RunCtx(ctx, func(ctx context.Context) error { return callAPI(ctx) })
//
// # Stopping conditions
//
// Retrying stops at the maximum number of attempts or when the context ends.
// The elapsed-time limit is off by default (enable it with WithMaxElapsedTime).
//
// # Delay hints
//
// When the error an operation returns implements DelayHinter, the next wait
// is the requested value rather than the computed exponential backoff (for
// HTTP Retry-After support and the like). Use WithMaxRetryAfter to cap the
// requested value.
//
// # Errors
//
// A failure returns *Error, which errors.Is matches against ErrExhausted /
// ErrPermanent, the context's error, and the last error the operation itself
// returned.
//
//	if errors.Is(err, retry.ErrExhausted) { ... }
//	if errors.Is(err, context.DeadlineExceeded) { ... }
package retry

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cenkalti/backoff/v7"
)

// Run retries the operation with exponential backoff.
// It returns nil on success and *Error on failure.
//
// A nil ctx is treated as context.Background().
func Run(ctx context.Context, op Operation, opts ...Option) error {
	if op == nil {
		return ErrNilOperation
	}
	_, err := RunValue(ctx, func() (struct{}, error) {
		return struct{}{}, op()
	}, opts...)
	return err
}

// RunCtx is the variant of Run whose operation takes a context.
// The operation receives the caller's ctx as is, so there is no need to
// capture ctx in a closure.
//
//	err := retry.RunCtx(ctx, func(ctx context.Context) error {
//	    return callAPI(ctx)
//	}, retry.WithMaxRetries(5))
//
// ctx stops the retry loop, but it interrupts a running operation only if the
// operation itself watches ctx.
//
// A nil ctx is treated as context.Background(), and that is what the
// operation receives.
func RunCtx(ctx context.Context, op OperationCtx, opts ...Option) error {
	if op == nil {
		return ErrNilOperation
	}
	_, err := RunValueCtx(ctx, func(ctx context.Context) (struct{}, error) {
		return struct{}{}, op(ctx)
	}, opts...)
	return err
}

// RunValueCtx is the variant of RunValue whose operation takes a context.
// See RunCtx for details.
func RunValueCtx[T any](ctx context.Context, op func(ctx context.Context) (T, error), opts ...Option) (T, error) {
	var zero T
	if op == nil {
		return zero, ErrNilOperation
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return RunValue(ctx, func() (T, error) { return op(ctx) }, opts...)
}

// RunValue is the variant of Run that returns a value: whatever the operation
// returned. On failure the value is what the last attempt returned (usually
// the zero value).
//
// A nil ctx is treated as context.Background().
func RunValue[T any](ctx context.Context, op func() (T, error), opts ...Option) (T, error) {
	var zero T
	if op == nil {
		return zero, ErrNilOperation
	}
	if ctx == nil {
		ctx = context.Background()
	}

	s := newSettings(opts)

	var attempts uint

	wrapped := func() (T, error) {
		attempts++
		v, err := op()
		if err == nil {
			return v, nil
		}

		// When judged not worth retrying, stop backoff from trying again.
		if s.shouldRetry != nil && !s.shouldRetry(err) {
			return v, backoff.Permanent(err)
		}

		// When the error states a wait, pass it on as the next interval with
		// backoff.RetryAfter. The original error is kept as the cause and comes
		// back as RetryError.LastErr when retrying stops, so it is not lost.
		if d, ok := retryAfterHint(err); ok {
			if s.maxRetryAfter > 0 && d > s.maxRetryAfter {
				return v, backoff.Permanent(fmt.Errorf("%w (%v > %v): %w", ErrRetryAfterTooLong, d, s.maxRetryAfter, err))
			}
			return v, backoff.RetryAfter(d, err)
		}
		return v, err
	}

	boOpts := []backoff.RetryOption{
		backoff.WithBackOff(&backoff.ExponentialBackOff{
			InitialInterval:     s.initialInterval,
			MaxInterval:         s.maxInterval,
			Multiplier:          s.multiplier,
			RandomizationFactor: s.randomizationFactor,
		}),
		// backoff's MaxTries is the total number of attempts, including the first.
		backoff.WithMaxTries(addSaturating(s.maxRetries, 1)),
		// backoff defaults to 15 minutes. Always pass it, since we stop on count and context only.
		backoff.WithMaxElapsedTime(s.maxElapsedTime),
	}
	if s.notify != nil {
		boOpts = append(boOpts, backoff.WithNotify(func(err error, next time.Duration) {
			// The error from backoff may be wrapped in a RetryAfterError, so hand
			// the hook the original error the operation returned.
			s.notify(unwrapRetryAfter(err), attempts, next)
		}))
	}

	v, err := backoff.Retry(ctx, wrapped, boOpts...)
	if err == nil {
		return v, nil
	}
	return v, newError(s.name, attempts, err)
}

// newError converts the error backoff returned into an *Error.
//
// On failure backoff always returns a *backoff.RetryError holding both the
// last operation error (LastErr) and the reason it stopped (Cause). Cause is
// mapped as follows:
//
//   - ErrPermanent      : Permanent = true (classified as ErrPermanent)
//   - ErrExhausted      : attempt limit; Cause is not kept (classified as ErrExhausted)
//   - ErrMaxElapsedTime : elapsed-time limit; same as above
//   - anything else     : kept in Cause as the reason the context ended
func newError(name string, attempts uint, err error) *Error {
	e := &Error{Op: name, Attempts: attempts, Err: err}

	re := backoff.AsRetryError(err)
	if re == nil {
		return e
	}
	e.Err = re.LastErr

	switch {
	case errors.Is(re.Cause, backoff.ErrPermanent):
		e.Permanent = true
	case errors.Is(re.Cause, backoff.ErrExhausted), errors.Is(re.Cause, backoff.ErrMaxElapsedTime):
		// Stopping is expressed as ErrExhausted, so Cause is not set.
	default:
		e.Cause = re.Cause
	}
	return e
}

// unwrapRetryAfter extracts the original error wrapped by backoff.RetryAfter.
func unwrapRetryAfter(err error) error {
	if ra, ok := errors.AsType[*backoff.RetryAfterError](err); ok {
		if inner := errors.Unwrap(ra); inner != nil {
			return inner
		}
	}
	return err
}

// addSaturating adds two uints, saturating instead of overflowing.
func addSaturating(a, b uint) uint {
	if a > ^uint(0)-b {
		return ^uint(0)
	}
	return a + b
}

// retryAfterHint extracts a DelayHinter's requested wait from the error chain.
func retryAfterHint(err error) (time.Duration, bool) {
	// errors.AsType requires T to satisfy error. DelayHinter does not embed
	// error, so this stays errors.As (go fix converts it incorrectly).
	var h DelayHinter
	if errors.As(err, &h) {
		if d := h.RetryAfter(); d > 0 {
			return d, true
		}
	}
	return 0, false
}
