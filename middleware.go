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

type BodySkipper func(echo.Context) (skipReqBody bool, skipRespBody bool)

func defaultBodySkipper(echo.Context) (skipReqBody bool, skipRespBody bool) {
	return
}

type (
	// SentryConfig defines the config for Sentry Performance middleware.
	SentryConfig struct {
		// Skipper defines a function to skip middleware.
		Skipper middleware.Skipper

		// BodySkipper defines a function to exclude body from logging
		BodySkipper BodySkipper

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

// Middleware returns a Sentry middleware with default config
func Middleware() echo.MiddlewareFunc {
	return MiddlewareWithConfig(DefaultSentryConfig)
}

// MiddlewareWithConfig returns a Sentry middleware with config.
func MiddlewareWithConfig(config SentryConfig) echo.MiddlewareFunc {
	if config.Skipper == nil {
		config.Skipper = middleware.DefaultSkipper
	}

	if config.BodySkipper == nil {
		config.BodySkipper = defaultBodySkipper
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

			skipReqBody, skipRespBody := config.BodySkipper(c)

			respDumper := dumpReq(c, config, span, request, skipReqBody, skipRespBody)

			// setup request context - add span
			c.SetRequest(request.WithContext(ctx))

			// call next middleware / controller
			err := next(c)
			if err != nil {
				setTag(span, "echo.error", err.Error())
				c.Error(err) // call custom registered error handler
			}

			dumpResp(c, config, span, respDumper, skipRespBody)

			return err
		}
	}
}

// dumpResp captures response information and adds it to the Sentry span.
func dumpResp(c echo.Context, config SentryConfig, span *sentry.Span, respDumper *response.Dumper, skipRespBody bool) {
	// Add request ID to span
	setTag(span, "request_id", getRequestID(c))

	// Set span status based on HTTP response status
	response := c.Response()
	span.Status = sentry.HTTPtoSpanStatus(response.Status)
	setTag(span, "resp.status", strconv.Itoa(response.Status))

	// Dump response headers if enabled
	if config.AreHeadersDump {
		captureResponseHeaders(response, span)
	}

	// Dump response body if enabled
	if config.IsBodyDump {
		if respDumper != nil {
			captureResponseBody(respDumper, span, skipRespBody)
		} else if skipRespBody {
			setTag(span, "resp.body", "[excluded]")
		}
	}
}

// captureResponseHeaders adds response headers to the span as tags
func captureResponseHeaders(response *echo.Response, span *sentry.Span) {
	for k := range response.Header() {
		setTag(span, "resp.header."+k, response.Header().Get(k))
	}
}

// captureResponseBody adds the response body to the span as a tag
func captureResponseBody(respDumper *response.Dumper, span *sentry.Span, skipRespBody bool) {
	respBody := respDumper.GetResponse()

	if respBody != "" && skipRespBody {
		respBody = "[excluded]"
	}

	if !skipRespBody {
		respBody = limitStringWithDots(respBody, MaxTagValueLength)
	}

	setTag(span, "resp.body", respBody)
}

// dumpReq captures request information and adds it to the Sentry span.
// It returns a response dumper if body dumping is enabled.
func dumpReq(c echo.Context, config SentryConfig, span *sentry.Span, request *http.Request, skipReqBody bool, skipRespBody bool) *response.Dumper {
	// Add basic auth username if present
	if username, _, ok := request.BasicAuth(); ok {
		setTag(span, "user", username)
	}

	// Add path parameters
	for _, paramName := range c.ParamNames() {
		setTag(span, "path."+paramName, c.Param(paramName))
	}

	// Dump request headers if enabled
	if config.AreHeadersDump {
		for k := range request.Header {
			setTag(span, "req.header."+k, request.Header.Get(k))
		}
	}

	// Initialize response dumper
	var respDumper *response.Dumper

	// Handle body dumping if enabled
	if config.IsBodyDump {
		// Capture request body if present
		if request.Body != nil {
			captureRequestBody(request, span, skipReqBody)
		}

		// Set up response body capture
		if !skipRespBody {
			respDumper = response.NewDumper(c.Response().Writer)
			c.Response().Writer = respDumper
		}
	}

	return respDumper
}

// captureRequestBody reads the request body, adds it to the span, and resets the body for further processing
func captureRequestBody(request *http.Request, span *sentry.Span, skipReqBody bool) {
	reqBody := []byte("[excluded]")

	if !skipReqBody {
		var err error

		bodyBytes, err := io.ReadAll(request.Body)
		_ = request.Body.Close()
		// Reset original request body so it can be read again by handlers
		request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		if err == nil {
			reqBody = bodyBytes
		}
	}

	setTag(span, "req.body", string(reqBody))
}

// createSpan creates a new Sentry span for the current request and returns:
// - the original HTTP request
// - the created Sentry span
// - a cleanup function that restores the original context and finishes the span
func createSpan(c echo.Context) (*http.Request, *sentry.Span, func()) {
	request := c.Request()
	originalContext := request.Context()

	// Create operation name using the HTTP method and path pattern (e.g., "HTTP GET /users/:id")
	operationName := "HTTP " + request.Method + " " + c.Path()

	// Create transaction name using the HTTP method and full request URI (e.g., "HTTP GET /users/123")
	transactionName := "HTTP " + request.Method + " " + request.RequestURI

	// Start a new Sentry span
	span := sentry.StartSpan(originalContext, operationName, sentry.WithTransactionName(transactionName))

	// Return the cleanup function that will be called when the middleware is done
	cleanupFunc := func() {
		// Restore the original context
		request = request.WithContext(originalContext)
		c.SetRequest(request)

		// Finish the span
		span.Finish()
	}

	return request, span, cleanupFunc
}
