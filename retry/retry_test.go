package retry_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/shouni/go-http-kit/retry"
)

// Short intervals keep the tests fast.
// Do not write tests that depend on the defaults (5s/30s).
func fastOpts(extra ...retry.Option) []retry.Option {
	return append([]retry.Option{
		retry.WithMaxRetries(2),
		retry.WithInitialInterval(10 * time.Millisecond),
		retry.WithMaxInterval(50 * time.Millisecond),
		retry.WithRandomizationFactor(0),
	}, extra...)
}

func TestRun(t *testing.T) {
	ctx := context.Background()

	t.Run("成功: 1回失敗した後にリトライで成功すること", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			calls := 0
			err := retry.Run(ctx, func() error {
				calls++
				if calls == 1 {
					return errors.New("temporary error")
				}
				return nil
			}, fastOpts()...)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if calls != 2 {
				t.Errorf("retry count: want 2, got %d", calls)
			}
		})
	})

	t.Run("失敗: 最大リトライ回数を超えてエラーが返ること", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			calls := 0
			errExpected := errors.New("persistent error")
			err := retry.Run(ctx, func() error {
				calls++
				return errExpected
			}, fastOpts(retry.WithName("TestOp"))...)

			if !errors.Is(err, retry.ErrExhausted) {
				t.Errorf("want ErrExhausted, got: %v", err)
			}
			if !errors.Is(err, errExpected) {
				t.Errorf("the original error is not wrapped: %v", err)
			}
			if !strings.Contains(err.Error(), "TestOp") {
				t.Errorf("the operation name is missing from the error message: %v", err)
			}
			// first (1) + retries (2) = 3 in total
			if calls != 3 {
				t.Errorf("attempt count: want 3, got %d", calls)
			}

			re, ok := errors.AsType[*retry.Error](err)
			if !ok {
				t.Fatalf("want *retry.Error, got: %v", err)
			}
			if re.Attempts != 3 {
				t.Errorf("Attempts: want 3, got %d", re.Attempts)
			}
			if re.Permanent {
				t.Error("Permanent should be false")
			}
		})
	})

	t.Run("中断: ShouldRetryFuncがfalseを返した時に即座に中止すること", func(t *testing.T) {
		calls := 0
		fatalErr := errors.New("fatal error")
		shouldRetry := func(err error) bool {
			return !strings.Contains(err.Error(), "fatal")
		}

		err := retry.Run(ctx, func() error {
			calls++
			return fatalErr
		}, fastOpts(retry.WithShouldRetry(shouldRetry))...)

		if !errors.Is(err, retry.ErrPermanent) {
			t.Errorf("want ErrPermanent, got: %v", err)
		}
		if errors.Is(err, retry.ErrExhausted) {
			t.Error("a permanent error must not be ErrExhausted")
		}
		if !errors.Is(err, fatalErr) {
			t.Errorf("the original error is not wrapped: %v", err)
		}
		if calls != 1 {
			t.Errorf("should stop immediately, but ran %d times", calls)
		}
	})

	t.Run("キャンセル: Contextがキャンセルされた場合に中断すること", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()

		opErr := errors.New("any error")
		err := retry.Run(cancelCtx, func() error { return opErr }, fastOpts()...)

		if !errors.Is(err, context.Canceled) {
			t.Errorf("want context.Canceled, got: %v", err)
		}
		// Check that *Error keeps both the reason it stopped and the last
		// operation error.
		if !errors.Is(err, opErr) {
			t.Errorf("the last operation error was lost: %v", err)
		}
		if errors.Is(err, retry.ErrExhausted) {
			t.Error("a context cancellation must not be ErrExhausted")
		}
	})

	t.Run("キャンセル: WithCancelCause の原因が伝播すること", func(t *testing.T) {
		causeErr := errors.New("shutting down")
		cancelCtx, cancel := context.WithCancelCause(context.Background())
		cancel(causeErr)

		err := retry.Run(cancelCtx, func() error { return errors.New("any error") }, fastOpts()...)

		if !errors.Is(err, causeErr) {
			t.Errorf("context.Cause was not propagated: %v", err)
		}
	})

	t.Run("タイムアウト: Contextのデッドラインで打ち切られること", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			timeoutCtx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
			defer cancel()

			err := retry.Run(timeoutCtx, func() error { return errors.New("slow failure") },
				retry.WithMaxRetries(100),
				retry.WithInitialInterval(10*time.Millisecond),
				retry.WithMaxInterval(10*time.Millisecond),
				retry.WithRandomizationFactor(0),
			)

			if !errors.Is(err, context.DeadlineExceeded) {
				t.Errorf("want context.DeadlineExceeded, got: %v", err)
			}
		})
	})

	t.Run("WithMaxRetries(0) はリトライせず1回だけ実行すること", func(t *testing.T) {
		calls := 0
		err := retry.Run(ctx, func() error {
			calls++
			return errors.New("boom")
		}, retry.WithMaxRetries(0), retry.WithInitialInterval(time.Millisecond))

		if calls != 1 {
			t.Errorf("attempt count: want 1, got %d", calls)
		}
		if !errors.Is(err, retry.ErrExhausted) {
			t.Errorf("want ErrExhausted, got: %v", err)
		}
	})

	t.Run("WithMaxAttempts は初回実行を含む回数であること", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			calls := 0
			_ = retry.Run(ctx, func() error {
				calls++
				return errors.New("boom")
			}, retry.WithMaxAttempts(2), retry.WithInitialInterval(time.Millisecond),
				retry.WithRandomizationFactor(0))

			if calls != 2 {
				t.Errorf("attempt count: want 2, got %d", calls)
			}
		})
	})

	t.Run("WithMaxAttempts(0) は1回として扱われること", func(t *testing.T) {
		calls := 0
		_ = retry.Run(ctx, func() error {
			calls++
			return errors.New("boom")
		}, retry.WithMaxAttempts(0), retry.WithInitialInterval(time.Millisecond))

		if calls != 1 {
			t.Errorf("attempt count: want 1, got %d", calls)
		}
	})

	t.Run("WithNotify が各リトライ前に呼ばれること", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			var attempts []uint
			var waits []time.Duration

			_ = retry.Run(ctx, func() error { return errors.New("boom") },
				fastOpts(retry.WithNotify(func(_ error, attempt uint, next time.Duration) {
					attempts = append(attempts, attempt)
					waits = append(waits, next)
				}))...)

			// 3 attempts = 2 notifications before retries
			if len(attempts) != 2 {
				t.Fatalf("notification count: want 2, got %d", len(attempts))
			}
			if attempts[0] != 1 || attempts[1] != 2 {
				t.Errorf("wrong attempt values: %v", attempts)
			}
			for i, w := range waits {
				if w <= 0 {
					t.Errorf("wrong wait (index %d): %v", i, w)
				}
			}
		})
	})

	t.Run("WithRandomizationFactor(0) で待機時間が決定的になること", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			var waits []time.Duration

			_ = retry.Run(ctx, func() error { return errors.New("boom") },
				retry.WithMaxRetries(2),
				retry.WithInitialInterval(10*time.Millisecond),
				retry.WithMaxInterval(time.Second),
				retry.WithMultiplier(2),
				retry.WithRandomizationFactor(0),
				retry.WithNotify(func(_ error, _ uint, next time.Duration) {
					waits = append(waits, next)
				}),
			)

			if len(waits) != 2 {
				t.Fatalf("notification count: want 2, got %d", len(waits))
			}
			if waits[0] != 10*time.Millisecond {
				t.Errorf("wrong first wait: %v", waits[0])
			}
			if waits[1] != 20*time.Millisecond {
				t.Errorf("Multiplier was not applied: %v", waits[1])
			}
		})
	})

	t.Run("WithMaxElapsedTime で経過時間により打ち切られること", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			calls := 0
			opErr := errors.New("boom")
			err := retry.Run(ctx, func() error {
				calls++
				return opErr
			},
				retry.WithMaxRetries(1000),
				retry.WithInitialInterval(20*time.Millisecond),
				retry.WithMaxInterval(20*time.Millisecond),
				retry.WithRandomizationFactor(0),
				retry.WithMaxElapsedTime(50*time.Millisecond),
			)

			// The fake clock makes the count deterministic: runs at 0ms / 20ms / 40ms
			// with a 20ms interval, and the next one at 60ms would pass
			// MaxElapsedTime (50ms), so it stops after 3.
			if calls != 3 {
				t.Errorf("not stopped by elapsed time: ran %d times, want 3", calls)
			}
			// Stopping on elapsed time is also classified as ErrExhausted.
			if !errors.Is(err, retry.ErrExhausted) {
				t.Errorf("want ErrExhausted, got: %v", err)
			}
			if !errors.Is(err, opErr) {
				t.Errorf("the last operation error was lost: %v", err)
			}
			if errors.Is(err, retry.ErrPermanent) {
				t.Errorf("must not be ErrPermanent: %v", err)
			}
		})
	})

	t.Run("失敗: nil Operation は即座にエラーになること", func(t *testing.T) {
		if err := retry.Run(ctx, nil); !errors.Is(err, retry.ErrNilOperation) {
			t.Errorf("want ErrNilOperation, got: %v", err)
		}
	})

	t.Run("nil context は Background として扱われること", func(t *testing.T) {
		// Check that passing nil by mistake does not panic.
		var nilCtx context.Context
		if err := retry.Run(nilCtx, func() error { return nil }); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

// rateLimitedError is a test error implementing DelayHinter.
type rateLimitedError struct{ after time.Duration }

func (e *rateLimitedError) Error() string             { return "rate limited" }
func (e *rateLimitedError) RetryAfter() time.Duration { return e.after }

func TestRetryAfterHint(t *testing.T) {
	ctx := context.Background()

	t.Run("上限を超えるヒントは待たずに打ち切ること", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			hinted := &rateLimitedError{after: time.Hour}
			var calls int

			start := time.Now()
			err := retry.Run(ctx, func() error { calls++; return hinted },
				retry.WithMaxRetries(3),
				retry.WithMaxRetryAfter(time.Minute),
			)

			if calls != 1 {
				t.Errorf("attempts = %d, want 1", calls)
			}
			if waited := time.Since(start); waited != 0 {
				t.Errorf("waited %v where it should have given up", waited)
			}
			if !errors.Is(err, retry.ErrRetryAfterTooLong) {
				t.Errorf("errors.Is(err, ErrRetryAfterTooLong) = false: %v", err)
			}
			if !errors.Is(err, retry.ErrPermanent) {
				t.Errorf("errors.Is(err, ErrPermanent) = false: %v", err)
			}
			if !errors.Is(err, hinted) {
				t.Errorf("the original error was lost: %v", err)
			}
		})
	})

	t.Run("上限以内のヒントは従うこと", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			hinted := &rateLimitedError{after: 40 * time.Millisecond}
			var waits []time.Duration

			_ = retry.Run(ctx, func() error { return hinted },
				retry.WithMaxRetries(1),
				retry.WithMaxRetryAfter(50*time.Millisecond),
				retry.WithNotify(func(_ error, _ uint, next time.Duration) {
					waits = append(waits, next)
				}),
			)

			if len(waits) != 1 || waits[0] != 40*time.Millisecond {
				t.Errorf("waits = %v, want [40ms]", waits)
			}
		})
	})

	t.Run("ヒントが次の待機時間として使われること", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			hinted := &rateLimitedError{after: 80 * time.Millisecond}
			var waits []time.Duration

			_ = retry.Run(ctx, func() error { return hinted },
				retry.WithMaxRetries(1),
				retry.WithInitialInterval(time.Millisecond),
				retry.WithRandomizationFactor(0),
				retry.WithNotify(func(_ error, _ uint, next time.Duration) {
					waits = append(waits, next)
				}),
			)

			if len(waits) != 1 {
				t.Fatalf("notification count: want 1, got %d", len(waits))
			}
			if waits[0] != 80*time.Millisecond {
				t.Errorf("the hint was not used as the wait: %v", waits[0])
			}
		})
	})

	t.Run("ラップされたヒントも errors.As で検出されること", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			hinted := &rateLimitedError{after: 60 * time.Millisecond}
			var waits []time.Duration

			_ = retry.Run(ctx, func() error { return fmt.Errorf("request failed: %w", hinted) },
				retry.WithMaxRetries(1),
				retry.WithInitialInterval(time.Millisecond),
				retry.WithRandomizationFactor(0),
				retry.WithNotify(func(_ error, _ uint, next time.Duration) {
					waits = append(waits, next)
				}),
			)

			if len(waits) != 1 || waits[0] != 60*time.Millisecond {
				t.Errorf("the wrapped hint was not used: %v", waits)
			}
		})
	})

	t.Run("0以下のヒントは無視され通常のバックオフになること", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			hinted := &rateLimitedError{after: 0}
			var waits []time.Duration

			_ = retry.Run(ctx, func() error { return hinted },
				retry.WithMaxRetries(1),
				retry.WithInitialInterval(10*time.Millisecond),
				retry.WithRandomizationFactor(0),
				retry.WithNotify(func(_ error, _ uint, next time.Duration) {
					waits = append(waits, next)
				}),
			)

			if len(waits) != 1 || waits[0] != 10*time.Millisecond {
				t.Errorf("a hint of 0 should be ignored: %v", waits)
			}
		})
	})

	t.Run("フックと最終エラーには元のエラーが渡ること", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			hinted := &rateLimitedError{after: time.Millisecond}
			var notified []error

			err := retry.Run(ctx, func() error { return hinted },
				retry.WithMaxRetries(1),
				retry.WithInitialInterval(time.Millisecond),
				retry.WithRandomizationFactor(0),
				retry.WithNotify(func(err error, _ uint, _ time.Duration) {
					notified = append(notified, err)
				}),
			)

			if len(notified) != 1 || !errors.Is(notified[0], hinted) {
				t.Errorf("the hook did not receive the original error: %v", notified)
			}
			re, ok := errors.AsType[*retry.Error](err)
			if !ok {
				t.Fatalf("want *retry.Error, got: %v", err)
			}
			if re.Err != hinted { //nolint:errorlint // checks it is the original error itself, not a wrapped form
				t.Errorf("Error.Err is not the original error: %v", re.Err)
			}
		})
	})

	t.Run("ShouldRetryFunc の打ち切りがヒントより優先されること", func(t *testing.T) {
		calls := 0
		err := retry.Run(ctx, func() error {
			calls++
			return &rateLimitedError{after: time.Millisecond}
		},
			retry.WithMaxRetries(3),
			retry.WithInitialInterval(time.Millisecond),
			retry.WithShouldRetry(func(error) bool { return false }),
		)

		if calls != 1 {
			t.Errorf("should stop immediately, but ran %d times", calls)
		}
		if !errors.Is(err, retry.ErrPermanent) {
			t.Errorf("want ErrPermanent, got: %v", err)
		}
	})
}

func TestRunCtx(t *testing.T) {
	t.Run("操作に呼び出し側の ctx が渡されること", func(t *testing.T) {
		type key struct{}
		ctx := context.WithValue(context.Background(), key{}, "marker")

		var got any
		err := retry.RunCtx(ctx, func(ctx context.Context) error {
			got = ctx.Value(key{})
			return nil
		})

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if got != "marker" {
			t.Errorf("ctx was not passed: %v", got)
		}
	})

	t.Run("操作が ctx を監視していればリトライ中に中断できること", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			cancelCtx, cancel := context.WithCancel(context.Background())
			defer cancel()

			calls := 0
			opErr := errors.New("boom")
			err := retry.RunCtx(cancelCtx, func(ctx context.Context) error {
				calls++
				if calls == 2 {
					cancel()
				}
				if ctx.Err() != nil {
					return ctx.Err()
				}
				return opErr
			}, fastOpts(retry.WithMaxRetries(10))...)

			if calls != 2 {
				t.Errorf("attempt count: want 2, got %d", calls)
			}
			if !errors.Is(err, context.Canceled) {
				t.Errorf("want context.Canceled, got: %v", err)
			}
		})
	})

	t.Run("失敗: nil Operation は即座にエラーになること", func(t *testing.T) {
		if err := retry.RunCtx(context.Background(), nil); !errors.Is(err, retry.ErrNilOperation) {
			t.Errorf("want ErrNilOperation, got: %v", err)
		}
	})

	t.Run("nil context は Background として操作に渡されること", func(t *testing.T) {
		var nilCtx context.Context
		err := retry.RunCtx(nilCtx, func(ctx context.Context) error {
			if ctx == nil {
				t.Error("the operation received a nil ctx")
			}
			return nil
		})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestRunValueCtx(t *testing.T) {
	t.Run("リトライ後に値を返せること", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			calls := 0
			got, err := retry.RunValueCtx(context.Background(), func(ctx context.Context) (int, error) {
				calls++
				if calls < 2 {
					return 0, errors.New("boom")
				}
				return 42, ctx.Err()
			}, fastOpts()...)

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if got != 42 {
				t.Errorf("value: want 42, got %d", got)
			}
		})
	})

	t.Run("失敗: nil Operation は即座にエラーになること", func(t *testing.T) {
		_, err := retry.RunValueCtx[int](context.Background(), nil)
		if !errors.Is(err, retry.ErrNilOperation) {
			t.Errorf("want ErrNilOperation, got: %v", err)
		}
	})
}

func TestRunValue(t *testing.T) {
	ctx := context.Background()

	t.Run("成功時に値を返すこと", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			calls := 0
			got, err := retry.RunValue(ctx, func() (string, error) {
				calls++
				if calls == 1 {
					return "", errors.New("temporary")
				}
				return "ok", nil
			}, fastOpts()...)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != "ok" {
				t.Errorf("return value: want \"ok\", got %q", got)
			}
		})
	})

	t.Run("失敗時はゼロ値とエラーを返すこと", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			got, err := retry.RunValue(ctx, func() (int, error) {
				return 0, errors.New("boom")
			}, fastOpts()...)

			if got != 0 {
				t.Errorf("want the zero value, got %d", got)
			}
			if !errors.Is(err, retry.ErrExhausted) {
				t.Errorf("want ErrExhausted, got: %v", err)
			}
		})
	})

	t.Run("失敗: nil Operation は即座にエラーになること", func(t *testing.T) {
		_, err := retry.RunValue[int](ctx, nil)
		if !errors.Is(err, retry.ErrNilOperation) {
			t.Errorf("want ErrNilOperation, got: %v", err)
		}
	})
}

func TestErrorFormatting(t *testing.T) {
	base := errors.New("boom")

	tests := []struct {
		name string
		err  *retry.Error
		want string
	}{
		{"Exhausted", &retry.Error{Op: "API", Attempts: 3, Err: base}, "failed after 3 attempt(s)"},
		{"Permanent", &retry.Error{Op: "API", Attempts: 1, Permanent: true, Err: base}, "permanent error"},
		{"Context", &retry.Error{Op: "API", Attempts: 2, Err: base, Cause: context.Canceled}, "last error: boom"},
		{"NoOpName", &retry.Error{Attempts: 1, Err: base}, "operation"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(tt.err.Error(), tt.want) {
				t.Errorf("message does not contain %q: %s", tt.want, tt.err.Error())
			}
		})
	}
}
