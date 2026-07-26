# Echo Sentry Middleware
[![Go Reference](https://pkg.go.dev/badge/github.com/adlandh/echo-sentry-middleware/v2.svg)](https://pkg.go.dev/github.com/adlandh/echo-sentry-middleware/v2)
[![Go Report Card](https://goreportcard.com/badge/github.com/adlandh/echo-sentry-middleware/v2)](https://goreportcard.com/report/github.com/adlandh/echo-sentry-middleware/v2)
[![Go Version](https://img.shields.io/github/go-mod/go-version/adlandh/echo-sentry-middleware)](https://github.com/adlandh/echo-sentry-middleware)


Echo middleware for sending performance traces to Sentry. This middleware captures HTTP request and response information and sends it to Sentry as spans, allowing you to monitor the performance of your Echo application.

## Features

- Captures HTTP request and response information as Sentry spans
- Configurable to include or exclude headers and bodies
- Supports skipping specific requests or specific parts of requests
- Integrates seamlessly with Echo's middleware system

## Installation

```shell
go get github.com/adlandh/echo-sentry-middleware/v2
```

## Usage

First, initialize Sentry in your application with tracing enabled:

```go
package main

import (
	"fmt"
	"net/http"

	echo_sentry_middleware "github.com/adlandh/echo-sentry-middleware/v2"
	"github.com/getsentry/sentry-go"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

func main() {
	if err := sentry.Init(sentry.ClientOptions{
		Dsn: "https://examplePublicKey@o0.ingest.sentry.io/0",
		// Enable tracing
		EnableTracing: true,
		// Specify a fixed sample rate:
		// We recommend adjusting this value in production
		TracesSampleRate: 1.0,
	}); err != nil {
		fmt.Printf("Sentry initialization failed: %v\n", err)
	}

	// Then create your app
	app := echo.New()

	// Add middleware with secure defaults.
	app.Use(echo_sentry_middleware.Middleware())

	// Add some endpoints
	app.POST("/", func(c *echo.Context) error {
		return c.String(http.StatusOK, "Hello, World!")
	})

	app.GET("/", func(ctx *echo.Context) error {
		return ctx.String(http.StatusOK, "Hello, World!")
	})

	// And run it
	app.Logger.Fatal(app.Start(":3000"))

}
```

## Configuration Options

The middleware can be configured using the `SentryConfig` struct:

```go
type SentryConfig struct {
	// Skipper defines a function to skip middleware execution
	Skipper middleware.Skipper

	// BodySkipper explicitly selects request/response bodies that may be captured.
	// All bodies are excluded when this is nil.
	BodySkipper BodySkipper

	// Add request & response headers to tracing tags.
	// Headers whose values may be recorded. Nil uses the default allowlist.
	SafeHeaders []string

	// Only allowlisted protocol metadata is recorded; all other values are redacted.
	AreHeadersDump bool

	// Enable body capture for bodies allowed by BodySkipper.
	IsBodyDump bool
}
```

### Default Configuration

You can use the middleware with default configuration:

```go
app.Use(echo_sentry_middleware.Middleware())
```

The default configuration:
- Uses the default Echo skipper (which doesn't skip any requests)
- Excludes request and response headers from the spans
- Excludes request and response bodies from the spans
- Uses a request-local Sentry Hub

### Custom Body Skipper

Body capture requires both `IsBodyDump` and a `BodySkipper` that explicitly allows the route. Only enable it for non-sensitive content:

```go
app.Use(echo_sentry_middleware.MiddlewareWithConfig(
	echo_sentry_middleware.SentryConfig{
		IsBodyDump:     true,
		BodySkipper: func(c *echo.Context) (skipReqBody bool, skipRespBody bool) {
			// Capture only this audited diagnostics endpoint.
			if c.Path() == "/diagnostics" {
				return false, false
			}
			return true, true
		},
	}))
```

Raw bodies may contain passwords, tokens, payment details, or personal data. The middleware truncates captured values but does not redact body fields.

## Captured Information

When enabled, the middleware captures the following information and sends it to Sentry as span tags:

### Always Captured
- Path pattern (`path`)
- Request ID (`request_id`)
- Response status code (`resp.status`)

### Captured When Headers Dump is Enabled
- Request headers (as `req.header.{header_name}`)
- Response headers (as `resp.header.{header_name}`)
- Only `Accept`, `Accept-Encoding`, `Cache-Control`, `Content-Encoding`, `Content-Length`, `Content-Type`, and `Transfer-Encoding` values are recorded
- Every other header value is replaced with `[redacted]`

Set `SafeHeaders` to replace the default allowlist. Header names are case-insensitive; use an empty non-nil slice to redact every header value.

### Captured When Body Dump is Enabled
- Request body (as `req.body`)
- Response body (as `resp.body`)
