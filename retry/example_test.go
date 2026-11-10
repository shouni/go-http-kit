package retry_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/shouni/go-http-kit/retry"
)

func ExampleRun() {
	attempts := 0

	err := retry.Run(context.Background(), func() error {
		attempts++
		if attempts < 3 {
			return errors.New("temporary failure")
		}
		return nil
	},
		retry.WithName("ExternalAPI"),
		retry.WithInitialInterval(time.Millisecond),
	)

	fmt.Printf("attempts=%d err=%v\n", attempts, err)
	// Output: attempts=3 err=<nil>
}

func ExampleRunValue() {
	body, err := retry.RunValue(context.Background(), func() (string, error) {
		return "response body", nil
	}, retry.WithName("Fetch"))

	fmt.Printf("body=%q err=%v\n", body, err)
	// Output: body="response body" err=<nil>
}

// Use RunCtx to pass the context to the operation.
// If the operation watches ctx, a running attempt can be interrupted too.
func ExampleRunCtx() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	attempts := 0
	err := retry.RunCtx(ctx, func(ctx context.Context) error {
		attempts++
		if attempts < 2 {
			return errors.New("temporary failure")
		}
		// ctx can be passed straight down to lower-level calls.
		return ctx.Err()
	},
		retry.WithName("ExternalAPI"),
		retry.WithInitialInterval(time.Millisecond),
	)

	fmt.Printf("attempts=%d err=%v\n", attempts, err)
	// Output: attempts=2 err=<nil>
}

var errServiceDown = errors.New("service down")

// A retry failure can be classified with errors.Is / errors.As.
func ExampleRun_errorHandling() {
	err := retry.Run(context.Background(), func() error { return errServiceDown },
		retry.WithName("Payment"),
		retry.WithMaxRetries(1),
		retry.WithInitialInterval(time.Millisecond),
	)

	fmt.Println("exhausted:", errors.Is(err, retry.ErrExhausted))
	fmt.Println("original:", errors.Is(err, errServiceDown))

	if re, ok := errors.AsType[*retry.Error](err); ok {
		fmt.Println("attempts:", re.Attempts)
	}
	// Output:
	// exhausted: true
	// original: true
	// attempts: 2
}

// When ShouldRetryFunc returns false, retrying stops right there.
func ExampleWithShouldRetry() {
	attempts := 0
	errUnauthorized := errors.New("401 unauthorized")

	err := retry.Run(context.Background(), func() error {
		attempts++
		return errUnauthorized
	},
		retry.WithInitialInterval(time.Millisecond),
		retry.WithShouldRetry(func(err error) bool {
			// An authentication error will not fix itself, so do not retry it.
			return !errors.Is(err, errUnauthorized)
		}),
	)

	fmt.Println("attempts:", attempts)
	fmt.Println("permanent:", errors.Is(err, retry.ErrPermanent))
	// Output:
	// attempts: 1
	// permanent: true
}

// throttledError implements DelayHinter, carrying a server-specified wait.
type throttledError struct{}

func (*throttledError) Error() string             { return "throttled" }
func (*throttledError) RetryAfter() time.Duration { return 5 * time.Millisecond }

// When the error implements RetryAfter, that value is used as the next wait
// instead of the computed exponential backoff (for HTTP Retry-After header
// support and the like).
func ExampleDelayHinter() {
	_ = retry.Run(context.Background(), func() error { return &throttledError{} },
		retry.WithMaxRetries(1),
		retry.WithInitialInterval(time.Millisecond),
		retry.WithNotify(func(err error, attempt uint, next time.Duration) {
			fmt.Printf("attempt %d failed: %v (retrying in %v)\n", attempt, err, next)
		}),
	)
	// Output: attempt 1 failed: throttled (retrying in 5ms)
}

func ExampleWithNotify() {
	_ = retry.Run(context.Background(), func() error { return errors.New("boom") },
		retry.WithMaxRetries(2),
		retry.WithInitialInterval(time.Millisecond),
		retry.WithMaxInterval(time.Millisecond),
		retry.WithRandomizationFactor(0), // disable jitter so the output is deterministic
		retry.WithNotify(func(err error, attempt uint, next time.Duration) {
			fmt.Printf("attempt %d failed: %v (retrying in %v)\n", attempt, err, next)
		}),
	)
	// Output:
	// attempt 1 failed: boom (retrying in 1ms)
	// attempt 2 failed: boom (retrying in 1ms)
}
