package retry

import (
	"context"
	"time"
)

const (
	// DefaultMaxRetries is the default maximum number of retries (not counting the first attempt).
	DefaultMaxRetries = 3
	// InitialBackoffInterval is the default initial backoff interval.
	InitialBackoffInterval = 5 * time.Second
	// MaxBackoffInterval is the default maximum backoff interval.
	MaxBackoffInterval = 30 * time.Second
	// DefaultMultiplier is the default backoff multiplier.
	DefaultMultiplier = 1.5
	// DefaultRandomizationFactor is the default jitter factor.
	// The actual wait is randomly spread within ±50% of the computed value.
	DefaultRandomizationFactor = 0.5
)

// Operation is a function representing a retryable operation.
type Operation func() error

// OperationCtx is a function representing a retryable operation that takes a
// context. RunCtx / RunValueCtx pass it the caller's ctx as is.
type OperationCtx func(ctx context.Context) error

// ShouldRetryFunc takes an error and reports whether it is retryable.
// Returning false stops retrying and sets Error.Permanent.
type ShouldRetryFunc func(error) bool

// NotifyFunc is called just before a retry.
// attempt is the number of the failed attempt (1-based), and next is the wait
// before the next attempt. Use it for logging or metrics.
type NotifyFunc func(err error, attempt uint, next time.Duration)

// DelayHinter lets an error state the minimum wait before the next retry.
// Implement it on an error type when the server specifies a wait, as with the
// Retry-After header of an HTTP 429/503.
//
// It is detected with errors.As, so it works on wrapped errors too.
// When RetryAfter returns a positive value, the next wait is that value
// instead of the computed exponential backoff, and the backoff schedule is
// reset afterwards. Values of 0 or less are ignored. WithMaxElapsedTime and
// context cancellation still apply as usual. When the requested wait exceeds
// the WithMaxRetryAfter cap, retrying stops with ErrRetryAfterTooLong without
// waiting.
type DelayHinter interface {
	RetryAfter() time.Duration
}

// settings holds the internal configuration of Run / RunValue.
type settings struct {
	name                string
	maxRetries          uint
	initialInterval     time.Duration
	maxInterval         time.Duration
	multiplier          float64
	randomizationFactor float64
	maxElapsedTime      time.Duration
	maxRetryAfter       time.Duration
	shouldRetry         ShouldRetryFunc
	notify              NotifyFunc
}

// Option adjusts the behaviour of Run / RunValue.
type Option func(*settings)

func newSettings(opts []Option) *settings {
	s := &settings{
		maxRetries:          DefaultMaxRetries,
		initialInterval:     InitialBackoffInterval,
		maxInterval:         MaxBackoffInterval,
		multiplier:          DefaultMultiplier,
		randomizationFactor: DefaultRandomizationFactor,
		// Retries are bounded by count, so the elapsed-time limit is off by default.
		maxElapsedTime: 0,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(s)
		}
	}
	return s
}

// WithName sets the operation name, which is included in error messages.
func WithName(name string) Option {
	return func(s *settings) { s.name = name }
}

// WithMaxRetries sets the maximum number of retries on top of the first
// attempt. 0 means "run once, without retrying".
func WithMaxRetries(n uint) Option {
	return func(s *settings) { s.maxRetries = n }
}

// WithMaxAttempts sets the maximum number of attempts, including the first.
// 1 means no retries. 0 is treated as 1.
func WithMaxAttempts(n uint) Option {
	return func(s *settings) {
		if n == 0 {
			n = 1
		}
		s.maxRetries = n - 1
	}
}

// WithInitialInterval sets the wait before the first retry. Negative values are ignored.
func WithInitialInterval(d time.Duration) Option {
	return func(s *settings) {
		if d >= 0 {
			s.initialInterval = d
		}
	}
}

// WithMaxInterval sets the upper bound on the wait. Negative values are ignored.
func WithMaxInterval(d time.Duration) Option {
	return func(s *settings) {
		if d >= 0 {
			s.maxInterval = d
		}
	}
}

// WithMultiplier sets the growth factor of the wait. 1.0 gives a fixed interval.
func WithMultiplier(m float64) Option {
	return func(s *settings) {
		if m > 0 {
			s.multiplier = m
		}
	}
}

// WithRandomizationFactor sets the amount of jitter in [0, 1].
// 0 means no jitter (a fully fixed schedule).
// Out-of-range values are ignored.
func WithRandomizationFactor(f float64) Option {
	return func(s *settings) {
		if f >= 0 && f <= 1 {
			s.randomizationFactor = f
		}
	}
}

// WithMaxElapsedTime caps the total elapsed time of retrying.
// 0 disables it, so retries are bounded by count and context only (the default).
// Negative values are ignored.
func WithMaxElapsedTime(d time.Duration) Option {
	return func(s *settings) {
		if d >= 0 {
			s.maxElapsedTime = d
		}
	}
}

// WithMaxRetryAfter caps the wait a DelayHinter (Retry-After) may request.
// When the requested wait exceeds it, retrying stops without waiting and
// ErrRetryAfterTooLong is returned. 0 (the default) means no cap.
//
// Over the cap it stops rather than clamping the wait, because hitting the
// server earlier than it said "not yet" only gets the same answer, and handing
// the decision back to the caller is faster. The requested wait itself stays
// on Error.Err (the last error).
func WithMaxRetryAfter(d time.Duration) Option {
	return func(s *settings) {
		if d >= 0 {
			s.maxRetryAfter = d
		}
	}
}

// WithShouldRetry sets the predicate deciding whether an error should be
// retried. When unset, every error is retried.
func WithShouldRetry(fn ShouldRetryFunc) Option {
	return func(s *settings) { s.shouldRetry = fn }
}

// WithNotify sets a hook called just before each retry.
func WithNotify(fn NotifyFunc) Option {
	return func(s *settings) { s.notify = fn }
}
