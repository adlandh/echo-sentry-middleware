# Repository Guide

## Scope
- This is one root Go package, module `github.com/adlandh/echo-sentry-middleware/v2`, targeting Go 1.25 and Echo v5.
- `middleware.go` owns request lifecycle and capture policy; `helpers.go` normalizes and truncates Sentry tags to 200 bytes without splitting UTF-8.
- There is no code generation, build step, external service, or integration-test setup.

## Verification
- Full suite: `go test ./...`.
- CI-equivalent suite: `go test -race -coverprofile=coverage.txt -covermode=atomic ./...`.
- Focus a testify suite method with a slash path, e.g. `go test -run 'TestMiddleware/TestSensitiveDataIsNotCaptured' ./...`.
- Focus a regular test with `go test -run TestLimitTagValue ./...`.
- Benchmarks: `go test -bench BenchmarkWithMiddleware -run '^$' ./...`.
- Format Go changes with `gofmt -w *.go`, then run `golangci-lint run ./...` and the race suite.
- `.golangci.yml` is the lint source of truth and has `run.tests: false`; lint success does not validate `_test.go` files. Lefthook runs lint and race tests in parallel, only for root `*.go` changes.

## Capture Invariants
- Keep headers and bodies disabled by default. `SafeHeaders == nil` uses the default allowlist; a non-nil list replaces it; an empty non-nil list redacts every header value. Header names are canonicalized case-insensitively, and non-allowlisted values stay `[redacted]`.
- `IsBodyDump` alone must not capture bodies: nil `BodySkipper` excludes both. Capture requires an explicit skipper returning `false` for the selected request/response side.
- Transactions use HTTP method plus Echo route template, never raw URI or path values. Do not add IPs, Basic Auth users, query strings, path parameters, or raw returned errors to tags.
- Clone and attach a Sentry Hub per request. Preserve downstream request-context changes.
- On panic, capture against the request Hub, finish the span with status 500, then repanic the original value. Returned statuses are resolved with `echo.ResolveResponseStatus`.
- Body capture is capped at `MaxTagValueLength * 4`; request prefixes are replayed so handlers still receive the complete body.

## Test Constraints
- `middleware_test.go` is a testify suite launched by top-level `TestMiddleware`; do not parallelize suite methods because `SetupTest` replaces global Sentry state.
- Security-policy changes need regression coverage for both the captured tag and what the downstream handler receives.
