# echo-sentry-middleware

Echo Sentry Performance middleware based on Jaeger tracing middleware

## Usage:

```shell
go get github.com/adlandh/echo-sentry-middleware
```

In your app:

```go
package main

import (
	"fmt"
	"net/http"

	echo_sentry_middleware "github.com/adlandh/echo-sentry-middleware"
	"github.com/getsentry/sentry-go"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
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