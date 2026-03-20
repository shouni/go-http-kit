package httpkit_test

import (
	"bytes"
	"io"
	"net/http"
)

// MockDoer は Doer のモック実装です。
type MockDoer struct {
	Responses []*http.Response
	Errors    []error
	CallCount int
	CustomDo  func(req *http.Request) (*http.Response, error)
}

func (m *MockDoer) Do(req *http.Request) (*http.Response, error) {
	if m.CustomDo != nil {
		return m.CustomDo(req)
	}
	defer func() { m.CallCount++ }()
	index := m.CallCount

	if index < len(m.Errors) && m.Errors[index] != nil {
		return nil, m.Errors[index]
	}
	if index < len(m.Responses) {
		return m.Responses[index], nil
	}
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewBufferString("default"))}, nil
}

// errorReader は意図的に読み込みエラーを発生させる io.Reader です。
type errorReader struct{ err error }

func (e *errorReader) Read(p []byte) (n int, err error) { return 0, e.err }

const MaxResponseBodySize = int64(25 * 1024 * 1024)
