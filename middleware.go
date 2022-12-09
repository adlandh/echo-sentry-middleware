package echo_sentry_middleware

import (
	"bytes"
	"io"
	"strconv"

	"github.com/getsentry/sentry-go"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

const operationName = "echo-sentry-middleware"

type (
	// SentryConfig defines the config for Sentry Perfomance middleware.
	SentryConfig struct {
		// Skipper defines a function to skip middleware.
		Skipper middleware.Skipper

		// add req headers & resp headers to tracing tags
		AreHeadersDump bool

		// add req body & resp body to attributes
		IsBodyDump bool

		// prevent logging long http request bodies
		LimitHTTPBody bool

		// http body limit size (in bytes)
		// NOTE: don't specify values larger than 60000 as jaeger can't handle values in span.LogKV larger than 60000 bytes
		LimitSize int
	}
)

var (
	// DefaultOtelConfig is the default OpenTelemetry middleware config.
	DefaultOtelConfig = SentryConfig{
		Skipper:        middleware.DefaultSkipper,
		AreHeadersDump: true,
		IsBodyDump:     false,
		LimitHTTPBody:  true,
		LimitSize:      1000,
	}
)

// Middleware returns a OpenTelemetry middleware with default config
func Middleware() echo.MiddlewareFunc {
	return MiddlewareWithConfig(DefaultOtelConfig)
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
			ctx := request.Context()
			defer func() {
				request = request.WithContext(savedCtx)
				c.SetRequest(request)
			}()
			tname := "HTTP " + request.Method + " URL: " + c.Path()
			if c.Path() != c.Request().RequestURI {
				tname = tname + " URI: " + c.Request().RequestURI
			}
			realIP := c.RealIP()
			requestID := getRequestID(c) // request-id generated by reverse-proxy

			var err error

			span := sentry.StartSpan(ctx, operationName, sentry.TransactionName(tname))
			defer span.Finish()

			transaction := sentry.TransactionFromContext(c.Request().Context())
			transaction.SetTag("client_ip", realIP)
			transaction.SetTag("request_id", requestID)
			transaction.SetTag("remote_addr", request.RemoteAddr)
			transaction.SetTag("request_uri", request.RequestURI)
			transaction.SetTag("http.path", c.Path())

			if username, _, ok := request.BasicAuth(); ok {
				transaction.SetTag("http.user", username)
			}

			//Add path parameters
			for _, paramName := range c.ParamNames() {
				transaction.SetTag("http.path."+paramName, c.Param(paramName))
			}

			//Dump request headers
			if config.AreHeadersDump {
				for k := range request.Header {
					transaction.SetTag("http.req.header."+k, request.Header.Get(k))
				}
			}

			// Dump request & response body
			var respDumper *responseDumper
			if config.IsBodyDump {
				// request
				reqBody := []byte{}
				if c.Request().Body != nil {
					reqBody, _ = io.ReadAll(c.Request().Body)

					if config.LimitHTTPBody {
						transaction.SetTag("http.req.body", limitString(string(reqBody), config.LimitSize))
					} else {
						transaction.SetTag("http.req.body", string(reqBody))
					}
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

			transaction.SetTag("http.resp.status", strconv.Itoa(c.Response().Status))

			//Dump response headers
			if config.AreHeadersDump {
				for k := range c.Response().Header() {
					transaction.SetTag("http.resp.header."+k, c.Response().Header().Get(k))
				}
			}

			// Dump response body
			if config.IsBodyDump {
				if config.LimitHTTPBody {
					transaction.SetTag("http.resp.body", limitString(respDumper.GetResponse(), config.LimitSize))
				} else {
					transaction.SetTag("http.resp.body", respDumper.GetResponse())
				}
			}

			return nil // error was already processed with ctx.Error(err)
		}
	}
}
