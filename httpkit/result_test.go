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
		{"ヘッダーの値をそのまま返す", &httpkit.Result{Header: http.Header{"Content-Type": []string{"text/html; charset=utf-8"}}}, "text/html; charset=utf-8"},
		{"ヘッダーが無ければ空文字", &httpkit.Result{Header: http.Header{}}, ""},
		{"Header が nil でも落ちない", &httpkit.Result{}, ""},
		{"レシーバが nil でも落ちない", nil, ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.res.ContentType(); got != tc.want {
				t.Errorf("ContentType() = %q, 期待 %q", got, tc.want)
			}
		})
	}
}

func TestResult_DecodeJSON(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
	}

	t.Run("デコードできる", func(t *testing.T) {
		res := &httpkit.Result{Body: []byte(`{"name":"kit"}`)}

		var got payload
		if err := res.DecodeJSON(&got); err != nil {
			t.Fatalf("予期しないエラー: %v", err)
		}
		if got.Name != "kit" {
			t.Errorf("Name = %q, 期待 %q", got.Name, "kit")
		}
	})

	t.Run("空ボディはエラーになる", func(t *testing.T) {
		res := &httpkit.Result{}

		var got payload
		err := res.DecodeJSON(&got)
		if err == nil {
			t.Fatal("204 のような空ボディはデコードエラーになる想定です")
		}
		if !strings.Contains(err.Error(), "JSONデコードに失敗しました") {
			t.Errorf("エラーメッセージが想定と異なります: %v", err)
		}
	})

	t.Run("レシーバが nil でも落ちない", func(t *testing.T) {
		var res *httpkit.Result

		var got payload
		if err := res.DecodeJSON(&got); err == nil {
			t.Fatal("nil レシーバはデコードエラーになる想定です")
		}
	})
}
