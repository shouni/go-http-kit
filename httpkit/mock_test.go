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
