package httpkit_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/shouni/go-http-kit/httpkit"
)

func TestResult_ContentType(t *testing.T) {
	testCases := []struct {
		name string
		res  *httpkit.Result
		want string
	}{
		{"returns header value as is", &httpkit.Result{Header: http.Header{"Content-Type": []string{"text/html; charset=utf-8"}}}, "text/html; charset=utf-8"},
		{"empty without header", &httpkit.Result{Header: http.Header{}}, ""},
		{"nil Header does not panic", &httpkit.Result{}, ""},
		{"nil receiver does not panic", nil, ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.res.ContentType(); got != tc.want {
				t.Errorf("ContentType() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestResult_DecodeJSON(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
	}

	t.Run("decodes", func(t *testing.T) {
		res := &httpkit.Result{Body: []byte(`{"name":"kit"}`)}

		var got payload
		if err := res.DecodeJSON(&got); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Name != "kit" {
			t.Errorf("Name = %q, want %q", got.Name, "kit")
		}
	})

	t.Run("empty body is an error", func(t *testing.T) {
		res := &httpkit.Result{}

		var got payload
		err := res.DecodeJSON(&got)
		if err == nil {
			t.Fatal("an empty body, as with a 204, should be a decode error")
		}
		if !strings.Contains(err.Error(), "failed to decode JSON") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("nil receiver does not panic", func(t *testing.T) {
		var res *httpkit.Result

		var got payload
		if err := res.DecodeJSON(&got); err == nil {
			t.Fatal("a nil receiver should be a decode error")
		}
	})
}
