package echosentrymiddleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/suite"
)

type TransportMock struct {
}

func (t *TransportMock) Configure(_ sentry.ClientOptions) {}
func (t *TransportMock) SendEvent(_ *sentry.Event) {
}
func (t *TransportMock) Flush(_ time.Duration) bool {
	return true
}
func (t *TransportMock) Events() []*sentry.Event {
	return nil
}

type MiddlewareTestSuite struct {
	suite.Suite
	transport *TransportMock
	client    *sentry.Client
	e         *echo.Echo
}

func (s *MiddlewareTestSuite) SetupTest() {
	var err error
	s.transport = &TransportMock{}
	s.client, err = sentry.NewClient(sentry.ClientOptions{
		EnableTracing: true,
		Transport:     s.transport,
	})
	s.NoError(err)
	s.e = echo.New()
}

func (s *MiddlewareTestSuite) TestMiddleware() {
	s.e.Use(Middleware())

	s.Run("Test Get", func() {
		s.e.GET("/", func(c echo.Context) error {
			span := sentry.TransactionFromContext(c.Request().Context())
			s.NotNil(span)
			s.NotEmpty(span.SpanID)
			s.NotEmpty(span.Tags["client_ip"])
			s.Equal(echo.MIMEApplicationJSON, span.Tags["req.header.Content-Type"])
			s.Equal("test", span.Tags["req.header.Testheader"])
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
	})
	s.Run("Test Post", func() {
		s.e.POST("/", func(c echo.Context) error {
			span := sentry.TransactionFromContext(c.Request().Context())
			s.NotNil(span)
			s.NotEmpty(span.SpanID)
			s.NotEmpty(span.Tags["client_ip"])
			s.Equal(echo.MIMEApplicationJSON, span.Tags["req.header.Content-Type"])
			s.Equal("test", span.Tags["req.header.Testheader"])
			s.Empty(span.Tags["req.body"])
			return c.String(http.StatusOK, "test")
		})
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
}

func (s *MiddlewareTestSuite) TestMiddlewareWithConfig() {
	s.e.Use(MiddlewareWithConfig(SentryConfig{
		AreHeadersDump: true,
		IsBodyDump:     true,
	}))

	s.Run("Test Get", func() {
		s.e.GET("/", func(c echo.Context) error {
			span := sentry.TransactionFromContext(c.Request().Context())
			s.NotNil(span)
			s.NotEmpty(span.SpanID)
			s.NotEmpty(span.Tags["client_ip"])
			s.Equal(echo.MIMEApplicationJSON, span.Tags["req.header.Content-Type"])
			s.Equal("test", span.Tags["req.header.Testheader"])
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
	})
	s.Run("Test Post", func() {
		s.e.POST("/", func(c echo.Context) error {
			span := sentry.TransactionFromContext(c.Request().Context())
			s.NotNil(span)
			s.NotEmpty(span.SpanID)
			s.NotEmpty(span.Tags["client_ip"])
			s.Equal(echo.MIMETextPlain, span.Tags["req.header.Content-Type"])
			s.Equal("test", span.Tags["req.header.Testheader"])
			s.Equal("testBody", span.Tags["req.body"])
			return c.String(http.StatusOK, "test")
		})
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
}

func TestMiddleware(t *testing.T) {
	suite.Run(t, new(MiddlewareTestSuite))
}
