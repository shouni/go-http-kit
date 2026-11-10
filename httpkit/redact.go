package httpkit

import (
	"net/url"
	"strings"
)

// redactedQuery replaces a redacted query. It records only that a query was present.
const redactedQuery = "REDACTED"

// redactedUserinfo replaces the userinfo of a URL that cannot be parsed.
// It matches the notation url.URL.Redacted uses for passwords.
const redactedUserinfo = "xxxxx"

// redactURL returns the URL string to put in error messages and retry
// operation names.
//
// Errors go to logs as they are. A query can carry a signed URL's signature or
// an API key in `?key=`, and url.URL.Redacted hides only the userinfo
// password. So the query and fragment values are dropped, keeping only the
// fact that they were present. Scheme, host and path are enough to trace
// which call failed.
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

// redactURLString is the string form of redactURL. Even for a URL that cannot
// be parsed, everything from `?` / `#` on may contain secrets and is cut off,
// and if the authority after `://` contains `@`, the whole userinfo is hidden
// (there is nothing to tell the password apart by).
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

// redactURLError returns a copy of err with its URL redacted when err is a
// *url.Error. Doer failures and the parse failures url.Parse returns take
// this form.
//
// An http.Client.Do error carries the request URL (or, mid-redirect, the URL
// at that point) with only the password removed, query included. Merely
// wrapping it would leave the URL in the second half of the message even
// though we redacted ours. The original error value is not modified; a copy
// is made and substituted, so an injected Doer reusing the same value is
// unaffected. Err is kept as is, so classification with errors.Is /
// errors.As does not change.
//
// Only the top-level *url.Error is inspected. That is the form http.Client
// returns, and copying a *url.Error wrapped further down would mean
// rebuilding the outer wrappers.
func redactURLError(err error) error {
	ue, ok := err.(*url.Error)
	if !ok || ue == nil {
		return err
	}
	c := *ue
	c.URL = redactURLString(ue.URL)
	return &c
}
