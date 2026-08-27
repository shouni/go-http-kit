# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`go-http-kit` is a Go library with two packages: `httpkit` (an HTTP client with SSRF/DNS-rebinding protection, exponential-backoff retry, response-size limiting, and JSON/stream helpers) and `retry` (the generic backoff engine `httpkit` runs on, a thin wrapper over `cenkalti/backoff/v7`). `retry` moved here from `netarmor` so that module stays a dependency-free security library; its only other consumer, `go-web-reader`, already depends on this one. URL/network validation still comes from `netarmor/securenet`. The client is `Doer`-compatible so callers can inject `*http.Client` or mocks.

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

- **`client.go`** — `Client` struct and `New()`. Holds `httpClient Doer` (buffered paths), `streamClient Doer` (stream paths), `RetryConfig`, `SkipNetworkValidation`, `DisableRetry`, `MaxBodySize`, `UserAgent`, `DisableBrowserHeaders`. `New` applies `ClientOption`s, then `ensureHTTPClient` picks the underlying `Doer`s: if `WithHTTPClient` wasn't used, the buffered client defaults to `securenet.NewSafeHTTPClient` (SSRF/DNS-rebinding-safe) unless `SkipNetworkValidation` is true, in which case it falls back to a plain `http.Client` with a cloned `DefaultTransport`. `newStreamClient` then derives the stream client: same `*http.Transport` (shared connection pool), but `Timeout: 0` — because `http.Client.Timeout` covers body reads and would kill long downloads — with the header phase bounded via `Transport.ResponseHeaderTimeout`. An injected `Doer` is used as-is for both roles.
- **`options.go`** — functional options (`WithHTTPClient`, `WithMaxRetries`, `WithNoRetry`, `WithInitialInterval`, `WithMaxInterval`, `WithSkipNetworkValidation`, `WithMaxResponseBodySize`, `WithUserAgent`, `WithoutBrowserHeaders`). `WithMaxRetries(0)` sets `DisableRetry` so retries are skipped entirely rather than running one attempt through the backoff machinery; `WithMaxRetries(n>0)` clears `DisableRetry` so option order can't leave retries accidentally disabled.
- **`interface.go`** — the public interfaces (`Doer`, `Requester`, `Downloader`, `URLValidator`, composed into `HTTPClient`). Any change to `Client`'s public method signatures must be mirrored here.
- **`request_helpers.go`** — shared plumbing used by every request path: `makeRequest` (builds `*http.Request`, adds common headers — no validation here), `addCommonHeaders` (Chrome-like `User-Agent`/`sec-ch-ua`/`Accept-Language`; UA comes from `Client.UserAgent`, the sec-ch-ua trio is suppressed by `DisableBrowserHeaders`), `doWithRetry` (delegates to `retry.Run` with options built by `RetryConfig.retryOptions()` unless `DisableRetry`), and `executeWithClone` — the single choke point every request path goes through: rejects nil requests with `ErrNilRequest`, runs the SSRF pre-check via `ValidateURL` once before the retry loop (using the request's own `ctx`, skipped when `SkipNetworkValidation`), then clones the request per attempt; on retries after the first, it requires `req.GetBody` to rebuild the body — returns `ErrRequestBodyNotReplayable` if absent. Because validation lives here, hand-built requests passed to `DoRequest`/`DoStreamRequest` are validated too.
- **`request.go`** — the `Requester` implementation: `DoRequest`, `FetchBytes` (returns body *and* `Content-Type`), `PostRawBodyAndFetchBytes` (sets `req.GetBody` so retries can replay the body), `PostJSONAndFetchBytes`, `FetchAndDecodeJSON`. `DoRequest` and `FetchBytes` share `doBuffered` (executes with retry, returns body + response headers, applies the client's `maxBodySize()` via `handleResponseWithLimit`).
- **`request_stream.go`** — the `Downloader` implementation (`FetchStream`, `GetStream`, `DoStreamRequest`). Runs on `streamClient` via `doStream` so body reads aren't killed by the overall client timeout. Uses `checkResponseStatus` instead of `HandleResponse` since the body must stay open for the caller/callback rather than being fully buffered; on error statuses it reads at most `MaxErrorBodySize` for the error detail. `FetchStream` is a thin wrapper over `GetStream`.
- **`response.go`** — `HandleResponse` (public wrapper over `handleResponseWithLimit` with the default `MaxResponseBodySize`; client methods pass `maxBodySize()` instead so `WithMaxResponseBodySize` takes effect; rejects early via `Content-Length` when possible, then via an `io.LimitReader` set to limit + 1 to detect overflow after the fact) and `IsHTTPRetryableError`, the `retry.ShouldRetryFunc` implementation: context errors and `NonRetryableHTTPError` never retry, internal sentinel errors (`ErrNilRequest`, `ErrNilResponse`, `ErrResponseBodyTooLarge`, etc.) never retry, `RetryableHTTPError` always retries, anything else (transient network errors) retries by default.
- **`error.go`** — `classifyStatusError` is the single place status codes are mapped to errors: 2xx → nil, retryable statuses (5xx, 408, 429 — see `isRetryableStatus`) → `*RetryableHTTPError`, everything else → `*NonRetryableHTTPError`. `RetryableHTTPError` carries the parsed `Retry-After` header (`RetryAfterDelay`, via `parseRetryAfter` — integer seconds or HTTP-date, 0 when absent/invalid) and implements `retry.DelayHinter`, so a server-specified wait overrides the computed backoff interval. The stored `Body` is capped at `MaxErrorBodySize` (copied on truncation so the huge backing array isn't retained). Both `request.go` (via `handleResponseWithLimit`) and `request_stream.go` (via `checkResponseStatus`) call into this rather than duplicating status logic. `IsRetryableHTTPError`/`IsNonRetryableError` use `errors.As` for classification. Display truncation (`formatBodyForError`) is byte-based, cut at rune boundaries, capped at `MaxBodyDisplaySize`.
- **`const.go`** — `DefaultHTTPTimeout`, `MaxResponseBodySize` (25MB default, per-client via `WithMaxResponseBodySize`), `MaxErrorBodySize` (64KB, cap on `Body` stored in error values and the read cap in `checkResponseStatus`), `MaxBodyDisplaySize` (1KB, error-message display only), and the spoofed `UserAgent`/`sec-ch-ua`/`AcceptLanguage` header values (Chrome 151; override via `WithUserAgent`/`WithoutBrowserHeaders`).

### Retry model

Retries happen by cloning the original `*http.Request` on each attempt (`executeWithClone`); a body-bearing request must set `req.GetBody` or retry fails with `ErrRequestBodyNotReplayable` on the second attempt. `PostRawBodyAndFetchBytes`/`PostJSONAndFetchBytes` set `GetBody` automatically; hand-built requests passed to `DoRequest` must set it themselves.

**The retry predicate never sees the HTTP method, and that is deliberate — do not "fix" it.** `IsHTTPRetryableError(err error) bool` is handed only the error, so a POST is retried on 5xx and transient network errors exactly like a GET. Neither a 5xx nor a read timeout proves the server didn't process the request, so retrying a non-idempotent call can duplicate its side effect — a second Slack message, a second enqueued job. Making the predicate method-aware looks like the obvious correction and is wrong here, because **the HTTP method is not a usable proxy for idempotency in this family of repos** — it misclassifies in both directions:

- `go-voicevox`'s synthesis call (`api/client.go`) is a **POST with no side effect** — the same text yields the same audio. It runs against a local engine that does get flaky under load, so it is exactly where a retry earns its keep. "Don't retry POST" would remove it.
- `ap-mcp`'s delete (`internal/client/base.go`) is a **DELETE, idempotent by RFC 7231**, and is nonetheless routed through a no-retry client on purpose ("二重削除を避ける実益は薄いものの、他の書き込み操作と同様に"). "Retry idempotent methods" would reinstate a retry the author deliberately removed.

Idempotency is a property of the *operation*, not of the method, which is why the mainstream implementations use explicit declaration rather than method sniffing: Go's own `net/http` treats a request as replayable if it is GET/HEAD/OPTIONS/TRACE **or carries an `Idempotency-Key` header**; gRPC declares retry policy per method in service config; the AWS SDK carries idempotency in operation metadata. The escape hatch is always a caller declaration.

`Client.WithoutRetry()` is that declaration here, and its granularity matches how these repos are already organised: the retry decision tracks a *role* (a reading client vs a writing client), not an individual call — `ap-mcp` had already split `readClient`/`writeClient` by hand. A per-request option (`PostJSON(..., httpkit.NoRetry())`) is therefore not needed yet; add it only when a single client genuinely needs both behaviours.

`WithoutRetry()` returns a shallow copy with `DisableRetry` set, sharing the same `Doer` — so the derived client keeps the original's timeout, SSRF settings, and connection pool. It exists because callers were hand-rolling it: `ap-mcp` built two `httpkit.New` calls differing only in `WithNoRetry`, and `ap-mcp-slack` constructs a whole separate client for its webhook path. Both duplicate configuration that can drift, and the second `New` also builds a second `securenet` client and connection pool. Prefer `WithoutRetry()` over a second `New` when the only difference is the retry policy; use `WithNoRetry` when the whole client should never retry.

One thing that *is* open: the predicate's final `return true` retries any error it could not classify. Narrowing it to a known set of transient network errors is defensible, but `net.Error` classification varies by platform and narrowing too far drops retries that currently work. Leave it until a real miss is observed.

### Test layout

Tests mix black-box (`package httpkit_test`, e.g. `client_test.go`, `request_test.go`) and white-box (`package httpkit`, suffixed `_internal_test.go`, e.g. `client_internal_test.go`) styles — use the white-box files when a test needs access to unexported helpers (`makeRequest`, `classifyStatusError`, etc.).
