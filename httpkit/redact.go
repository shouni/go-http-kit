package httpkit

import (
	"net/url"
	"strings"
)

// redactedQuery は、伏せたクエリの代わりに置く値です。クエリが付いていたことだけを残します。
const redactedQuery = "REDACTED"

// redactedUserinfo は、解析できない URL のユーザー情報の代わりに置く値です。
// url.URL.Redacted がパスワードに使う表記に揃えています。
const redactedUserinfo = "xxxxx"

// redactURL は、エラーメッセージやリトライの操作名に載せる URL の文字列を返します。
//
// エラーはそのままログに出ます。クエリには署名付き URL の署名や `?key=` の API キーが
// 乗ることがあり、url.URL.Redacted が伏せるのはユーザー情報のパスワードだけです。
// そこでクエリとフラグメントは値を落とし、付いていたことだけを残します。
// どこへの呼び出しが失敗したかを追うには、スキーム・ホスト・パスで足ります。
func redactURL(u *url.URL) string {
	if u == nil {
		return ""
	}
	c := *u
	if c.RawQuery != "" || c.ForceQuery {
		c.RawQuery = redactedQuery
		c.ForceQuery = false
	}
	c.Fragment, c.RawFragment = "", ""
	return c.Redacted()
}

// redactURLString は redactURL の文字列版です。解析できない URL でも、
// `?` / `#` 以降は秘密を含みうるので切り落とし、`://` の後ろのオーソリティに `@` が
// あればユーザー情報ごと伏せます（パスワードだけを見分ける手がかりがないため）。
func redactURLString(raw string) string {
	if u, err := url.Parse(raw); err == nil {
		return redactURL(u)
	}

	s := raw
	suffix := ""
	if i := strings.IndexAny(s, "?#"); i >= 0 {
		s = s[:i]
		suffix = "?" + redactedQuery
	}
	if i := strings.Index(s, "://"); i >= 0 {
		authStart := i + len("://")
		authEnd := len(s)
		if j := strings.IndexByte(s[authStart:], '/'); j >= 0 {
			authEnd = authStart + j
		}
		if at := strings.LastIndexByte(s[authStart:authEnd], '@'); at >= 0 {
			s = s[:authStart] + redactedUserinfo + s[authStart+at:]
		}
	}
	return s + suffix
}

// redactURLError は、エラーが *url.Error なら、その URL を伏せた写しを返します。
// Doer の失敗と、url.Parse が返す解析の失敗がこの形です。
//
// http.Client.Do のエラーは、パスワードだけを落としたリクエスト URL（リダイレクト中なら
// その時点の URL）をクエリごと抱えています。包んで返すだけでは、こちらで伏せても
// 文面の後半にそのまま出ます。元のエラー値は書き換えず、写しを作って差し替えるので、
// 注入された Doer が同じ値を使い回していても影響しません。Err はそのまま残るため、
// errors.Is / errors.As による分類は変わりません。
//
// 見るのは最上位の *url.Error だけです。http.Client が返す形はこれで、さらに奥に
// 包まれた *url.Error の写しを作るには、外側の包みを組み直す必要があるためです。
func redactURLError(err error) error {
	ue, ok := err.(*url.Error)
	if !ok || ue == nil {
		return err
	}
	c := *ue
	c.URL = redactURLString(ue.URL)
	return &c
}
