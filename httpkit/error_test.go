package httpkit_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/shouni/go-http-kit/httpkit"
	"github.com/stretchr/testify/assert"
)

func TestIsHTTPRetryableError(t *testing.T) {
	client := httpkit.New(0)

	t.Run("ContextErrors", func(t *testing.T) {
		assert.False(t, client.IsHTTPRetryableError(context.Canceled))
		wrapped := fmt.Errorf("fail: %w", context.DeadlineExceeded)
		assert.False(t, client.IsHTTPRetryableError(wrapped))
	})

	t.Run("RetryableErrors", func(t *testing.T) {
		assert.True(t, client.IsHTTPRetryableError(fmt.Errorf("5xx リトライ対象")))
		assert.True(t, client.IsHTTPRetryableError(fmt.Errorf("i/o timeout")))
	})
}
