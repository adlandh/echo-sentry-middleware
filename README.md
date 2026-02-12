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

	// Add middleware
	app.Use(echo_sentry_middleware.MiddlewareWithConfig(
		echo_sentry_middleware.SentryConfig{
			// if you would like to save your request or response headers as tags, set AreHeadersDump to true
			AreHeadersDump: true,
			// if you would like to save your request or response body as tags, set IsBodyDump to true
			IsBodyDump: true,
		}))

	// Add some endpoints
	app.POST("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "Hello, World!")
	})

	app.GET("/", func(ctx echo.Context) error {
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

	// BodySkipper defines a function to exclude request/response body from logging
	BodySkipper BodySkipper

	// Add request & response headers to tracing tags
	AreHeadersDump bool

	// Add request & response body to attributes
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
- Includes headers in the spans
- Excludes request and response bodies from the spans

### Custom Body Skipper

You can define a custom function to skip logging of request and response bodies:

```go
app.Use(echo_sentry_middleware.MiddlewareWithConfig(
	echo_sentry_middleware.SentryConfig{
		AreHeadersDump: true,
		IsBodyDump:     true,
		BodySkipper: func(c echo.Context) (skipReqBody bool, skipRespBody bool) {
			// Skip request and response bodies for paths containing "sensitive"
			if strings.Contains(c.Path(), "sensitive") {
				return true, true
			}
			// Skip only request bodies for paths containing "upload"
			if strings.Contains(c.Path(), "upload") {
				return true, false
			}
			return false, false
		},
	}))
```

## Captured Information

When enabled, the middleware captures the following information and sends it to Sentry as span tags:

### Always Captured
- Client IP address (`client_ip`)
- Remote address (`remote_addr`)
- Request URI (`request_uri`)
- Path pattern (`path`)
- Request ID (`request_id`)
- Response status code (`resp.status`)
- Path parameters (as `path.{param_name}`)
- Basic auth username (as `user`) if present

### Captured When Headers Dump is Enabled
- Request headers (as `req.header.{header_name}`)
- Response headers (as `resp.header.{header_name}`)

### Captured When Body Dump is Enabled
- Request body (as `req.body`)
- Response body (as `resp.body`)
