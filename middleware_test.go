package echosentrymiddleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/suite"
)

const (
	contentTypeHeader = "req.header.Content-Type"
	testHeader        = "req.header.Testheader"
	respStatus        = "resp.status"
)

type TransportMock struct {
	lock   sync.Mutex
	events []*sentry.Event
}

func (*TransportMock) Configure(_ sentry.ClientOptions) { /* stub */ }
func (t *TransportMock) SendEvent(event *sentry.Event) {
	t.lock.Lock()
	defer t.lock.Unlock()
	t.events = append(t.events, event)
}
func (t *TransportMock) Flush(_ time.Duration) bool {
	clear(t.events)
	return true
}
func (t *TransportMock) Events() []*sentry.Event {
	t.lock.Lock()
	defer t.lock.Unlock()
	return t.events
}

type MiddlewareTestSuite struct {
	suite.Suite
	transport *TransportMock
	e         *echo.Echo
}

func (s *MiddlewareTestSuite) SetupTest() {
	var err error
	s.transport = &TransportMock{}
	err = sentry.Init(sentry.ClientOptions{
		EnableTracing: true,
		Transport:     s.transport,
	})
	s.NoError(err)
	s.e = echo.New()
}

func (s *MiddlewareTestSuite) TestMiddleware() {
	s.e.Use(Middleware())

	s.Run("Test Get", func() {
		var span *sentry.Span
		s.e.GET("/", func(c echo.Context) error {
			span = sentry.TransactionFromContext(c.Request().Context())
			s.NotNil(span)
			s.NotEmpty(span.SpanID)
			s.NotEmpty(span.Tags["client_ip"])
			s.Equal(echo.MIMEApplicationJSON, span.Tags[contentTypeHeader])
			s.Equal("test", span.Tags[testHeader])
			return c.String(http.StatusOK, "test")
		})

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		req.Header.Set("testHeader", "test")
		rec := httptest.NewRecorder()
		s.e.ServeHTTP(rec, req)
		s.Equal(http.StatusOK, rec.Code)
		body, err := io.ReadAll(rec.Body)
		s.NoError(err)
		s.Equal("test", string(body))
		s.Equal(sentry.HTTPtoSpanStatus(http.StatusOK), span.Status)
		s.Equal(strconv.Itoa(http.StatusOK), span.Tags[respStatus])
	})
	s.Run("Test Post", func() {
		var span *sentry.Span
		s.e.POST("/", func(c echo.Context) error {
			span = sentry.TransactionFromContext(c.Request().Context())
			s.NotNil(span)
			s.NotEmpty(span.SpanID)
			s.NotEmpty(span.Tags["client_ip"])
			s.Equal(echo.MIMETextPlain, span.Tags[contentTypeHeader])
			s.Equal("test", span.Tags[testHeader])
			s.Empty(span.Tags["req.body"])
			return c.String(http.StatusOK, "test")
		})

		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("testBody"))
		req.Header.Set(echo.HeaderContentType, echo.MIMETextPlain)
		req.Header.Set("testHeader", "test")
		rec := httptest.NewRecorder()
		s.e.ServeHTTP(rec, req)
		s.Equal(http.StatusOK, rec.Code)
		body, err := io.ReadAll(rec.Body)
		s.NoError(err)
		s.Equal("test", string(body))
		s.Equal(sentry.HTTPtoSpanStatus(http.StatusOK), span.Status)
		s.Equal(strconv.Itoa(http.StatusOK), span.Tags[respStatus])
	})
}

func (s *MiddlewareTestSuite) TestMiddlewareWithConfig() {
	s.e.Use(MiddlewareWithConfig(SentryConfig{
		AreHeadersDump: true,
		IsBodyDump:     true,
		BodySkipper: func(context echo.Context) (skipReqBody bool, skipRespBody bool) {
			if context.Request().Header.Get(echo.HeaderContentType) == "skip it" {
				return true, true
			}
			return false, false
		},
	}))

	s.Run("Test Get", func() {
		var span *sentry.Span
		s.e.GET("/", func(c echo.Context) error {
			span = sentry.TransactionFromContext(c.Request().Context())
			s.NotNil(span)
			s.NotEmpty(span.SpanID)
			s.NotEmpty(span.Tags["client_ip"])
			s.Equal(echo.MIMEApplicationJSON, span.Tags[contentTypeHeader])
			s.Equal("test", span.Tags[testHeader])
			return c.String(http.StatusOK, "test")
		})

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		req.Header.Set("testHeader", "test")
		rec := httptest.NewRecorder()
		s.e.ServeHTTP(rec, req)
		s.Equal(http.StatusOK, rec.Code)
		body, err := io.ReadAll(rec.Body)
		s.NoError(err)
		s.Equal("test", string(body))
		s.Equal(sentry.HTTPtoSpanStatus(http.StatusOK), span.Status)
		s.Equal(strconv.Itoa(http.StatusOK), span.Tags[respStatus])
		s.Equal("test", span.Tags["resp.body"])
	})

	s.Run("Test Post", func() {
		var span *sentry.Span
		s.e.POST("/", func(c echo.Context) error {
			span = sentry.TransactionFromContext(c.Request().Context())
			s.NotNil(span)
			s.NotEmpty(span.SpanID)
			s.NotEmpty(span.Tags["client_ip"])
			s.Equal(echo.MIMETextPlain, span.Tags[contentTypeHeader])
			s.Equal("test", span.Tags[testHeader])
			s.Equal("testBody", span.Tags["req.body"])
			return c.String(http.StatusOK, "test")
		})

		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("testBody"))
		req.Header.Set(echo.HeaderContentType, echo.MIMETextPlain)
		req.Header.Set("testHeader", "test")
		rec := httptest.NewRecorder()
		s.e.ServeHTTP(rec, req)
		s.Equal(http.StatusOK, rec.Code)
		body, err := io.ReadAll(rec.Body)
		s.NoError(err)
		s.Equal("test", string(body))
		s.Equal(sentry.HTTPtoSpanStatus(http.StatusOK), span.Status)
		s.Equal(strconv.Itoa(http.StatusOK), span.Tags[respStatus])
		s.Equal("test", span.Tags["resp.body"])
	})

	s.Run("Test Skip Body", func() {
		var span *sentry.Span
		s.e.POST("/", func(c echo.Context) error {
			span = sentry.TransactionFromContext(c.Request().Context())
			s.NotNil(span)
			s.NotEmpty(span.SpanID)
			s.NotEmpty(span.Tags["client_ip"])
			s.Equal("[excluded]", span.Tags["req.body"])
			return c.String(http.StatusOK, "test")
		})

		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("testBody"))
		req.Header.Set(echo.HeaderContentType, "skip it")
		rec := httptest.NewRecorder()
		s.e.ServeHTTP(rec, req)
		s.Equal(http.StatusOK, rec.Code)
		body, err := io.ReadAll(rec.Body)
		s.NoError(err)
		s.Equal("test", string(body))
		s.Equal(sentry.HTTPtoSpanStatus(http.StatusOK), span.Status)
		s.Equal(strconv.Itoa(http.StatusOK), span.Tags[respStatus])
		s.Equal("[excluded]", span.Tags["resp.body"])
	})
}

func TestMiddleware(t *testing.T) {
	suite.Run(t, new(MiddlewareTestSuite))
}
