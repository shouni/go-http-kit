# 🌐 Go HTTP Kit

[![CI](https://github.com/shouni/go-http-kit/actions/workflows/ci.yml/badge.svg)](https://github.com/shouni/go-http-kit/actions/workflows/ci.yml)
[![Status](https://img.shields.io/badge/Status-Active-brightgreen)](#)
[![Language](https://img.shields.io/badge/Language-Go-blue)](https://go.dev/)
[![Go Version](https://img.shields.io/github/go-mod/go-version/shouni/go-http-kit)](https://go.dev/)
[![GitHub tag (latest by date)](https://img.shields.io/github/v/tag/shouni/go-http-kit)](https://github.com/shouni/go-http-kit/tags)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Reference](https://pkg.go.dev/badge/github.com/shouni/go-http-kit.svg)](https://pkg.go.dev/github.com/shouni/go-http-kit)

English | [日本語](README.ja.md)

## 🚀 About - Caller-side defences by default; no server side

`go-http-kit` provides a client (`httpkit`) that comes with the defences you would otherwise keep adding every time
you call out over HTTP, and the general-purpose retry engine underneath it (`retry`). SSRF / DNS rebinding
protection, exponential backoff and a response size cap apply the same way whichever method you come in through.

It takes on only the **calling** side. The boilerplate for the side that answers lives in
[go-serve-kit](https://github.com/shouni/go-serve-kit), and managing credentials and interpreting the fetched bytes
stay with the caller. The `net/http`-compatible `Doer` is the injection point, so an existing `http.Client` or a test
mock can be plugged in as is.

---

## ✨ Features

* **SSRF / DNS rebinding protection by default** — uses the `netarmor/securenet` client, and URL pre-validation
  runs automatically on every request path (allowed schemes: `http` / `https`). Only `Do`, which returns a bare
  `*http.Response`, skips both retries and pre-validation.
* **Retries with exponential backoff** — on 5xx / 408 / 429 and on network errors, including a per-attempt timeout.
  Any other 4xx is a `NonRetryableHTTPError` and is not retried. When the server returns `Retry-After` (either
  seconds or an HTTP-date), it takes precedence over the computed backoff. When the requested wait exceeds the cap
  (2 minutes by default, `WithMaxRetryAfter`), the client gives up without waiting and returns
  `retry.ErrRetryAfterTooLong`.
* **Response size cap** — buffered reads have a cap, and going over it is `ErrResponseBodyTooLarge`. When
  `Content-Length` already exceeds the cap, it returns without reading the body.
* **Streams are outside the overall timeout** — long downloads are not cut off midway
  (the connection pool is shared with the buffered paths).
* **Derive with `WithoutRetry`** — turn off only the retries for sending, while sharing the configuration and the
  connection pool.
* **`retry` works on its own** — exposed as a general-purpose retry that does not depend on HTTP.

Configuration is through functional options, and failures can be classified by type and sentinel. The full list of
options, methods, error values and constants is on [pkg.go.dev](https://pkg.go.dev/github.com/shouni/go-http-kit).

---

## 📦 Package Structure

```text
go-http-kit/
├── httpkit/   # HTTP client: SSRF pre-validation, retries, size cap, streams
└── retry/     # General-purpose exponential backoff (a cenkalti/backoff wrapper); the base httpkit runs on
```

`retry` decides only "when and how long to wait" and knows nothing about HTTP. "What is worth retrying" and "how to
resend a request" belong to `httpkit`. Because of this split, fetch loops that do not go through HTTP can run on the
same engine.

---

## 🚦 Usage

```go
client := httpkit.New(
    httpkit.WithTimeout(15*time.Second),
    httpkit.WithMaxRetries(3),
)

var out struct {
    Name string `json:"name"`
}
if err := client.GetJSON(ctx, "https://api.example.com/data", &out); err != nil {
    return err
}
```

Every option can be omitted. `httpkit.New()` returns a client with the default timeout, 3 retries and SSRF
protection. If you also need the status or headers, `Get` / `Post` / `Send` return a `*Result`, and large files are
received with `ReadStream` / `GetStream`.

**The retry decision is made from the error alone and never looks at the HTTP method. POSTs are retried too, so a
non-idempotent send can run twice.** Declare calls that must not be resent by turning retries off with
`client.WithoutRetry()` (or `WithNoRetry` for the whole client).

**The expensive pitfalls are each written up in their godoc as well** — why the method is not used to decide
automatically (the package documentation), that `WithTimeout` stretches when combined with retries
(`DefaultHTTPTimeout`), that it does not apply to stream body reads (`GetStream` / `ReadStream`), that hand-built
requests get no common headers and need `req.GetBody` (`Send`), and that passing `WithDoer` does not turn off
pre-validation (`WithDoer`).

---

## 🤝 Dependencies

* [shouni/netarmor](https://github.com/shouni/netarmor) - network security (`securenet`)
* [cenkalti/backoff](https://github.com/cenkalti/backoff) - the exponential backoff implementation behind the `retry` package

---

## 📜 License

This project is released under the [MIT License](https://opensource.org/licenses/MIT).
