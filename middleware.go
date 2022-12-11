package echo_sentry_middleware

import (
	"bytes"
	"io"
	"strconv"

	"github.com/getsentry/sentry-go"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type (
	// SentryConfig defines the config for Sentry Perfomance middleware.
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
	// DefaultSentryConfig is the default Sengry Performance middleware config.
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
			if config.Skipper(c) {
				return next(c)
			}

			request := c.Request()
			savedCtx := request.Context()

			defer func() {
				request = request.WithContext(savedCtx)
				c.SetRequest(request)
			}()

			opname := "HTTP " + request.Method + " " + c.Path()
			tname := "HTTP " + request.Method + " " + c.Request().RequestURI

			realIP := c.RealIP()
			requestID := getRequestID(c) // request-id generated by reverse-proxy

			var err error

			span := sentry.StartSpan(savedCtx, opname, sentry.TransactionName(tname))
			defer span.Finish()
			ctx := span.Context()

			transaction := sentry.TransactionFromContext(ctx)
			transaction.SetTag("client_ip", prepareTagValue(realIP))
			transaction.SetTag("request_id", prepareTagValue(requestID))
			transaction.SetTag("remote_addr", prepareTagValue(request.RemoteAddr))
			transaction.SetTag("request_uri", prepareTagValue(request.RequestURI))
			transaction.SetTag("path", prepareTagValue(c.Path()))

			if username, _, ok := request.BasicAuth(); ok {
				transaction.SetTag("user", prepareTagValue(username))
			}

			//Add path parameters
			for _, paramName := range c.ParamNames() {
				transaction.SetTag("path."+paramName, prepareTagValue(c.Param(paramName)))
			}

			//Dump request headers
			if config.AreHeadersDump {
				for k := range request.Header {
					transaction.SetTag("req.header."+k, request.Header.Get(k))
				}
			}

			// Dump request & response body
			var respDumper *responseDumper
			if config.IsBodyDump {
				// request
				reqBody := []byte{}
				if c.Request().Body != nil {
					reqBody, _ = io.ReadAll(c.Request().Body)

					transaction.SetTag("req.body", prepareTagValue(string(reqBody)))

				}

				request.Body = io.NopCloser(bytes.NewBuffer(reqBody)) // reset original request body

				// response
				respDumper = newResponseDumper(c.Response())
				c.Response().Writer = respDumper
			}

			// setup request context - add span
			c.SetRequest(request.WithContext(ctx))

			// call next middleware / controller
			err = next(c)
			if err != nil {
				transaction.SetTag("echo.error", err.Error())
				c.Error(err) // call custom registered error handler
			}

			transaction.SetTag("resp.status", strconv.Itoa(c.Response().Status))

			//Dump response headers
			if config.AreHeadersDump {
				for k := range c.Response().Header() {
					transaction.SetTag("resp.header."+k, prepareTagValue(c.Response().Header().Get(k)))
				}
			}

			// Dump response body
			if config.IsBodyDump {
				transaction.SetTag("resp.body", prepareTagValue(respDumper.GetResponse()))
			}

			return nil // error was already processed with ctx.Error(err)
		}
	}
}
