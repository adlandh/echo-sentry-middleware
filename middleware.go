// Package echosentrymiddleware is a middleware for echo framework that sends traces to Sentry
package echosentrymiddleware

import (
	"bytes"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/adlandh/response-dumper"
	"github.com/getsentry/sentry-go"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

// BodySkipper decides, per request, whether the request and/or response body
// should be omitted from the span tags.
type BodySkipper func(*echo.Context) (skipReqBody bool, skipRespBody bool)

func defaultBodySkipper(*echo.Context) (bool, bool) {
	return true, true
}

type (
	// SentryConfig defines the config for Sentry Performance middleware.
	SentryConfig struct {
		// Skipper defines a function to skip middleware.
		Skipper middleware.Skipper

		// BodySkipper explicitly selects which request and response bodies may be captured.
		// When nil, all bodies are excluded.
		BodySkipper BodySkipper

		// SafeHeaders lists headers whose values may be recorded when AreHeadersDump is enabled.
		// When nil, the default allowlist is used. An empty non-nil slice records no values.
		SafeHeaders []string

		// AreHeadersDump adds request and response headers to tracing tags.
		// Only allowlisted protocol metadata is recorded; all other values are redacted.
		AreHeadersDump bool

		// IsBodyDump enables body capture for bodies allowed by BodySkipper.
		IsBodyDump bool
	}
)

var (
	// DefaultSentryConfig is the default Sentry Performance middleware config.
	DefaultSentryConfig = SentryConfig{
		Skipper:        middleware.DefaultSkipper,
		AreHeadersDump: false,
		SafeHeaders:    []string{"Accept", "Accept-Encoding", "Cache-Control", "Content-Encoding", "Content-Length", "Content-Type", "Transfer-Encoding"},
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

	if config.SafeHeaders == nil {
		config.SafeHeaders = DefaultSentryConfig.SafeHeaders
	}

	safeHeaders := make(map[string]struct{}, len(config.SafeHeaders))
	for _, name := range config.SafeHeaders {
		safeHeaders[http.CanonicalHeaderKey(name)] = struct{}{}
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) (err error) {
			if config.Skipper(c) {
				return next(c)
			}

			request, span, hub := createSpan(c)
			setTag(span, "path", c.Path())

			skipReqBody, skipRespBody := config.BodySkipper(c)

			respDumper := dumpReq(c, config, safeHeaders, span, request, skipReqBody, skipRespBody)

			// setup request context - add span
			c.SetRequest(request.WithContext(span.Context()))

			defer func() {
				panicValue := recover()
				if panicValue != nil {
					err = echo.NewHTTPError(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))

					hub.RecoverWithContext(c.Request().Context(), panicValue)
				}

				dumpResp(c, config, safeHeaders, span, respDumper, skipRespBody, err)
				span.Finish()

				if panicValue != nil {
					panic(panicValue)
				}
			}()

			// call next middleware / controller
			return next(c)
		}
	}
}

// dumpResp captures response information and adds it to the Sentry span.
func dumpResp(c *echo.Context, config SentryConfig, safeHeaders map[string]struct{}, span *sentry.Span, respDumper *response.Dumper, skipRespBody bool, handlerErr error) {
	// Add request ID to span
	setTag(span, "request_id", getRequestID(c))

	// Set span status based on HTTP response status
	responseWriter, status := echo.ResolveResponseStatus(c.Response(), handlerErr)
	span.Status = sentry.HTTPtoSpanStatus(status)
	setTag(span, "resp.status", strconv.Itoa(status))

	// Dump response headers if enabled
	if config.AreHeadersDump && responseWriter != nil {
		captureHeaders("resp.header.", responseWriter.Header(), safeHeaders, span)
	}

	// Dump response body if enabled
	if config.IsBodyDump {
		if respDumper != nil {
			captureResponseBody(respDumper, span)
		} else if skipRespBody {
			setTag(span, "resp.body", "[excluded]")
		}
	}
}

// captureHeaders adds non-sensitive headers to the span as tags.
func captureHeaders(prefix string, header http.Header, safeHeaders map[string]struct{}, span *sentry.Span) {
	for k, v := range header {
		value := "[redacted]"
		if _, ok := safeHeaders[http.CanonicalHeaderKey(k)]; ok {
			value = strings.Join(v, ", ")
		}

		setTag(span, prefix+k, value)
	}
}

// captureResponseBody adds the response body to the span as a tag.
// Only invoked when the body was actually dumped, so skipRespBody is always false here.
func captureResponseBody(respDumper *response.Dumper, span *sentry.Span) {
	setTag(span, "resp.body", limitStringWithDots(respDumper.GetResponse(), MaxTagValueLength))
}

// maxBodyCaptureBytes caps how much of the request/response body is read for the span tag.
// The tag itself is further truncated to MaxTagValueLength; the extra headroom
// leaves room for UTF-8 boundary trimming. Bytes beyond this cap stream through
// to the handler unread (and uncaptured) instead of being buffered into memory.
const maxBodyCaptureBytes = MaxTagValueLength * 4

// dumpReq captures request information and adds it to the Sentry span.
// It returns a response dumper if body dumping is enabled.
func dumpReq(c *echo.Context, config SentryConfig, safeHeaders map[string]struct{}, span *sentry.Span, request *http.Request, skipReqBody bool, skipRespBody bool) *response.Dumper {
	// Dump request headers if enabled
	if config.AreHeadersDump {
		captureHeaders("req.header.", request.Header, safeHeaders, span)
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
			respDumper = response.NewDumper(c.Response(), response.WithMaxBytes(maxBodyCaptureBytes))
			c.SetResponse(respDumper)
		}
	}

	return respDumper
}

// captureRequestBody reads up to maxBodyCaptureBytes of the request body for the
// span tag, then replaces request.Body with a reader that re-emits the captured
// prefix followed by the unread remainder so handlers see the full body.
func captureRequestBody(request *http.Request, span *sentry.Span, skipReqBody bool) {
	reqBody := []byte("[excluded]")

	if !skipReqBody {
		originalBody := request.Body

		captured, err := io.ReadAll(io.LimitReader(originalBody, maxBodyCaptureBytes))
		if err == nil {
			request.Body = struct {
				io.Reader
				io.Closer
			}{
				Reader: io.MultiReader(bytes.NewReader(captured), originalBody),
				Closer: originalBody,
			}
			reqBody = captured
		} else {
			request.Body = originalBody
			reqBody = []byte("[read_error]")
		}
	}

	setTag(span, "req.body", string(reqBody))
}

// createSpan creates a request-local Sentry Hub and span.
func createSpan(c *echo.Context) (*http.Request, *sentry.Span, *sentry.Hub) {
	request := c.Request()

	hub := sentry.GetHubFromContext(request.Context())
	if hub == nil {
		hub = sentry.CurrentHub()
	}

	hub = hub.Clone()
	ctx := sentry.SetHubOnContext(request.Context(), hub)

	// Create operation name using the HTTP method and path pattern (e.g., "HTTP GET /users/:id")
	route := c.Path()
	if route == "" {
		route = "unmatched"
	}

	operationName := "HTTP " + request.Method + " " + route

	// Start a new Sentry span
	span := sentry.StartSpan(ctx, operationName, sentry.WithTransactionName(operationName))

	return request, span, hub
}
