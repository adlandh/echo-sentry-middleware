// Package echosentrymiddleware is a middleware for echo framework that sends traces to Sentry
package echosentrymiddleware

import (
	"bytes"
	"io"
	"net/http"
	"strconv"

	"github.com/adlandh/response-dumper"
	"github.com/getsentry/sentry-go"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type (
	// SentryConfig defines the config for Sentry Performance middleware.
	SentryConfig struct {
		// Skipper defines a function to skip middleware.
		Skipper middleware.Skipper

		// add req headers & resp headers to tracing tags
		AreHeadersDump bool

		// add req body & resp body to attributes
		IsBodyDump bool
	}
)

var (
	// DefaultSentryConfig is the default Sentry Performance middleware config.
	DefaultSentryConfig = SentryConfig{
		Skipper:        middleware.DefaultSkipper,
		AreHeadersDump: true,
		IsBodyDump:     false,
	}
)

// Middleware returns a OpenTelemetry middleware with default config
func Middleware() echo.MiddlewareFunc {
	return MiddlewareWithConfig(DefaultSentryConfig)
}

// MiddlewareWithConfig returns a OpenTelemetry middleware with config.
func MiddlewareWithConfig(config SentryConfig) echo.MiddlewareFunc {
	if config.Skipper == nil {
		config.Skipper = middleware.DefaultSkipper
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if config.Skipper(c) || c.Request() == nil || c.Response() == nil {
				return next(c)
			}

			request, span, endSpan := createSpan(c)
			defer endSpan()

			ctx := span.Context()

			setTag(span, "client_ip", c.RealIP())
			setTag(span, "remote_addr", request.RemoteAddr)
			setTag(span, "request_uri", request.RequestURI)
			setTag(span, "path", c.Path())

			respDumper := dumpReq(c, config, span, request)

			// setup request context - add span
			c.SetRequest(request.WithContext(ctx))

			// call next middleware / controller
			err := next(c)
			if err != nil {
				setTag(span, "echo.error", err.Error())
				c.Error(err) // call custom registered error handler
			}

			dumpResp(c, config, span, respDumper)

			return err
		}
	}
}

func dumpResp(c echo.Context, config SentryConfig, span *sentry.Span, respDumper *response.Dumper) {
	setTag(span, "request_id", getRequestID(c))
	span.Status = sentry.HTTPtoSpanStatus(c.Response().Status)
	setTag(span, "resp.status", strconv.Itoa(c.Response().Status))

	// Dump response headers
	if config.AreHeadersDump {
		for k := range c.Response().Header() {
			setTag(span, "resp.header."+k, c.Response().Header().Get(k))
		}
	}

	// Dump response body
	if config.IsBodyDump {
		setTag(span, "resp.body", respDumper.GetResponse())
	}
}

func dumpReq(c echo.Context, config SentryConfig, span *sentry.Span, request *http.Request) *response.Dumper {
	if username, _, ok := request.BasicAuth(); ok {
		setTag(span, "user", username)
	}

	// Add path parameters
	for _, paramName := range c.ParamNames() {
		setTag(span, "path."+paramName, c.Param(paramName))
	}

	// Dump request headers
	if config.AreHeadersDump {
		for k := range request.Header {
			setTag(span, "req.header."+k, request.Header.Get(k))
		}
	}

	// Dump request & response body
	var respDumper *response.Dumper

	if config.IsBodyDump {
		// request
		if request.Body != nil {
			reqBody, err := io.ReadAll(request.Body)
			if err == nil {
				setTag(span, "req.body", string(reqBody))

				_ = request.Body.Close()
				request.Body = io.NopCloser(bytes.NewBuffer(reqBody)) // reset original request body
			}
		}

		// response
		respDumper = response.NewDumper(c.Response())
		c.Response().Writer = respDumper
	}

	return respDumper
}

func createSpan(c echo.Context) (*http.Request, *sentry.Span, func()) {
	request := c.Request()
	savedCtx := request.Context()
	opname := "HTTP " + request.Method + " " + c.Path()
	tname := "HTTP " + request.Method + " " + c.Request().RequestURI
	span := sentry.StartSpan(savedCtx, opname, sentry.WithTransactionName(tname))

	return request, span, func() {
		request = request.WithContext(savedCtx)
		c.SetRequest(request)

		defer span.Finish()
	}
}
