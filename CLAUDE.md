# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`go-http-kit` is a single-package Go library (`httpkit`) providing an HTTP client with SSRF/DNS-rebinding protection, exponential-backoff retry, response-size limiting, and JSON/stream helpers. It wraps `github.com/shouni/netarmor` (`securenet` for URL/network validation, `retry` for backoff) behind a `Doer`-compatible interface so callers can inject `*http.Client` or mocks.

## Commands

```bash
go build ./...                      # build
go vet ./...                        # vet
gofmt -l .                          # check formatting (CI fails if output is non-empty); use `gofmt -w .` to fix
go test -race ./...                 # full test suite, matches CI
go test -race ./httpkit/... -run TestName   # run a single test
golangci-lint run                   # lint (CI pins golangci-lint v2.12.2; config in .golangci.yml)
govulncheck ./...                   # vulnerability scan (also runs in CI)
```

CI (`.github/workflows/ci.yml`) runs three parallel jobs on push/PR to `main`/`develop`: build+vet+gofmt+`go test -race`, `golangci-lint`, and `govulncheck`. Go version is read from `go.mod` (currently 1.26).

## Architecture

Everything lives in the `httpkit` package (no internal packages, no subdirectories). Files are split by responsibility, and understanding a request's full path requires reading across several of them:

- **`client.go`** — `Client` struct and `New()`. Holds `httpClient Doer`, `RetryConfig`, `SkipNetworkValidation`, `DisableRetry`. `New` applies `ClientOption`s, then `ensureHTTPClient` picks the underlying `Doer`: if `WithHTTPClient` wasn't used, it defaults to `securenet.NewSafeHTTPClient` (SSRF/DNS-rebinding-safe) unless `SkipNetworkValidation` is true, in which case it falls back to a plain `http.Client`.
- **`options.go`** — functional options (`WithHTTPClient`, `WithMaxRetries`, `WithNoRetry`, `WithInitialInterval`, `WithMaxInterval`, `WithSkipNetworkValidation`). `WithMaxRetries(0)` sets `DisableRetry` so retries are skipped entirely rather than running one attempt through the backoff machinery.
- **`interface.go`** — the public interfaces (`Doer`, `Requester`, `Downloader`, `URLValidator`, composed into `HTTPClient`). Any change to `Client`'s public method signatures must be mirrored here.
- **`request_helpers.go`** — shared plumbing used by every request path: `makeRequest` (SSRF pre-check via `ValidateURL` using the request's own `ctx` unless skipped, then builds `*http.Request`, adds common headers), `addCommonHeaders` (fixed Chrome-like `User-Agent`/`sec-ch-ua`/`Accept-Language` from `const.go`), `doWithRetry` (delegates to `netarmor/retry.Run` with options built by `RetryConfig.retryOptions()` unless `DisableRetry`), and `executeWithClone` (clones the request per attempt; on retries after the first, it requires `req.GetBody` to rebuild the body — returns `ErrRequestBodyNotReplayable` if absent).
- **`request.go`** — the `Requester` implementation: `DoRequest`, `FetchBytes` (returns body *and* `Content-Type`), `PostRawBodyAndFetchBytes` (sets `req.GetBody` so retries can replay the body), `PostJSONAndFetchBytes`, `FetchAndDecodeJSON`. All route through `makeRequest` + `executeWithClone` + `HandleResponse`.
- **`request_stream.go`** — the `Downloader` implementation (`FetchStream`, `GetStream`, `DoStreamRequest`). Uses `checkResponseStatus` instead of `HandleResponse` since the body must stay open for the caller/callback rather than being fully buffered.
- **`response.go`** — `HandleResponse` (buffers up to `MaxResponseBodySize`, rejecting early via `Content-Length` when possible, then via an `io.LimitReader` set to `MaxResponseBodySize + 1` to detect overflow after the fact) and `IsHTTPRetryableError`, the `retry.ShouldRetryFunc` implementation: context errors and `NonRetryableHTTPError` never retry, internal sentinel errors (`ErrNilResponse`, `ErrResponseBodyTooLarge`, etc.) never retry, `RetryableHTTPError` (5xx) always retries, anything else (transient network errors) retries by default.
- **`error.go`** — `classifyStatusError` is the single place status codes are mapped to errors: 2xx → nil, 5xx → `*RetryableHTTPError`, everything else → `*NonRetryableHTTPError`. Both `request.go` (via `HandleResponse`) and `request_stream.go` (via `checkResponseStatus`) call into this rather than duplicating status logic. `IsRetryableHTTPError`/`IsNonRetryableError` use `errors.As` for classification.
- **`const.go`** — `DefaultHTTPTimeout`, `MaxResponseBodySize` (25MB), `MaxBodyDisplaySize` (1KB, used both for error-message truncation and as the read cap in `checkResponseStatus`), and the spoofed `UserAgent`/`sec-ch-ua`/`AcceptLanguage` header values.

### Retry model

Retries happen by cloning the original `*http.Request` on each attempt (`executeWithClone`); a body-bearing request must set `req.GetBody` or retry fails with `ErrRequestBodyNotReplayable` on the second attempt. `PostRawBodyAndFetchBytes`/`PostJSONAndFetchBytes` set `GetBody` automatically; hand-built requests passed to `DoRequest` must set it themselves.

### Test layout

Tests mix black-box (`package httpkit_test`, e.g. `client_test.go`, `request_test.go`) and white-box (`package httpkit`, suffixed `_internal_test.go`, e.g. `client_internal_test.go`) styles — use the white-box files when a test needs access to unexported helpers (`makeRequest`, `classifyStatusError`, etc.).
