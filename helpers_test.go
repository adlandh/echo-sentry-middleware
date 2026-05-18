package echosentrymiddleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/getsentry/sentry-go"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"github.com/stretchr/testify/require"
)

func TestLimitTagValue(t *testing.T) {
	tests := []struct {
		name string
		str  string
		want string
	}{
		{
			name: "Short string",
			str:  "i3WEG3605Kj7i3WEG3605Kj7i3WEG3605Kj7i3WEG3605Kj7i3WEG3605Kj7i3WEG3605Kj7i3WEG3605Kj7i3WEG3605Kj7i3WEG3605Kj7",
			want: "i3WEG3605Kj7i3WEG3605Kj7i3WEG3605Kj7i3WEG3605Kj7i3WEG3605Kj7i3WEG3605Kj7i3WEG3605Kj7i3WEG3605Kj7i3WEG3605Kj7",
		},
		{
			name: "Long string containing \\n",
			str:  "05Kj7z2AXCl603gMJu6B23z2sD05\nKj7z2AXCl603gMJu6B23z2sD05Kj7z\n2AXCl603gMJu6B23z2sD05Kj7z2AXCl603gMJu6B23z2sD05Kj7z2AXCl603gMJu6B23z2sD05Kj7z2AXCl603gMJu6B23z2sD05Kj7z2AXCl603gMJu6B23z2sD05Kj7z2AXCl603gMJu6B23z2sD05Kj7z2AXCl603gMJu6B23z2sD05Kj7z2AXCl603gMJu6B23z2sD05Kj7z2AXCl603gMJu6B23z2sD",
			want: "05Kj7z2AXCl603gMJu6B23z2sD05 Kj7z2AXCl603gMJu6B23z2sD05Kj7z 2AXCl603gMJu6B23z2sD05Kj7z2AXCl603gMJu6B23z2sD05Kj7z2AXCl603gMJu6B23z2sD05Kj7z2AXCl603gMJu6B23z2sD05Kj7z2AXCl603gMJu6B23z2sD05Kj7z2AXCl60...",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.LessOrEqual(t, len(prepareTagValue(tt.str)), MaxTagValueLength)
			require.Equal(t, tt.want, prepareTagValue(tt.str))
		})
	}
}

func TestLimitTagName(t *testing.T) {
	tests := []struct {
		name string
		str  string
		want string
	}{
		{
			name: "Short string",
			str:  "i3WEG3605Kj7",
			want: "i3WEG3605Kj7",
		},
		{
			name: "Long string",
			str:  strings.Repeat("a", MaxTagNameLength+10),
			want: strings.Repeat("a", MaxTagNameLength),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.LessOrEqual(t, len(prepareTagName(tt.str)), MaxTagNameLength)
			require.Equal(t, tt.want, prepareTagName(tt.str))
		})
	}
}

func TestGetRequestID(t *testing.T) {
	t.Run("token in header", func(t *testing.T) {
		e := echo.New()
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set(echo.HeaderXRequestID, "test")
		w := httptest.NewRecorder()
		c := e.NewContext(r, w)
		e.ServeHTTP(w, r)
		require.Equal(t, "test", getRequestID(c))
	})

	t.Run("generate token", func(t *testing.T) {
		e := echo.New()
		e.Use(middleware.RequestID())
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		c := e.NewContext(r, w)
		e.ServeHTTP(w, r)
		require.Equal(t, 32, len(getRequestID(c)))
	})

	t.Run("no token without middleware", func(t *testing.T) {
		e := echo.New()
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		c := e.NewContext(r, w)
		e.ServeHTTP(w, r)
		require.Empty(t, getRequestID(c))
	})
}

func TestLimitTagValueEdgeCases(t *testing.T) {
	t.Run("empty string", func(t *testing.T) {
		require.Equal(t, "", prepareTagValue(""))
	})

	t.Run("exactly max length", func(t *testing.T) {
		input := strings.Repeat("a", MaxTagValueLength)
		want := strings.Repeat("a", MaxTagValueLength-3) + "..."
		require.Len(t, input, MaxTagValueLength)
		require.Equal(t, want, prepareTagValue(input))
	})

	t.Run("normalize newlines and tabs", func(t *testing.T) {
		input := "line1\r\nline2\tline3\nline4"
		want := "line1  line2 line3 line4"
		require.Equal(t, want, prepareTagValue(input))
	})

	t.Run("zero size returns input unchanged", func(t *testing.T) {
		require.Equal(t, "hello", limitStringWithDots("hello", 0))
		require.Equal(t, "hello", limitString("hello", 0))
	})

	t.Run("tiny size skips dot suffix", func(t *testing.T) {
		// size <= minDotsLimit (10) triggers the no-dots branch.
		require.Equal(t, "hello", limitStringWithDots("hello world", 5))
	})

	t.Run("truncation respects utf8 rune boundary", func(t *testing.T) {
		// "世" is a 3-byte rune. Build a string that, when truncated at
		// MaxTagValueLength-3 bytes (the room left for "..."), would split
		// the final rune mid-sequence if we cut blindly.
		head := strings.Repeat("a", MaxTagValueLength-5)
		input := head + "世xx" + strings.Repeat("b", 20)
		got := prepareTagValue(input)
		require.True(t, utf8.ValidString(got), "result must be valid UTF-8")
		require.LessOrEqual(t, len(got), MaxTagValueLength)
	})
}

func TestSetTag(t *testing.T) {
	t.Run("empty tag is dropped", func(t *testing.T) {
		span := sentry.StartSpan(context.Background(), "test")
		defer span.Finish()
		setTag(span, "", "value")
		require.Empty(t, span.Tags)
	})

	t.Run("empty value is preserved", func(t *testing.T) {
		span := sentry.StartSpan(context.Background(), "test")
		defer span.Finish()
		setTag(span, "k", "")
		_, ok := span.Tags["k"]
		require.True(t, ok, "empty value should still set the tag")
	})
}

func TestLimitTagNameEdgeCases(t *testing.T) {
	t.Run("empty string", func(t *testing.T) {
		require.Equal(t, "", prepareTagName(""))
	})

	t.Run("exactly max length", func(t *testing.T) {
		input := strings.Repeat("a", MaxTagNameLength)
		require.Len(t, input, MaxTagNameLength)
		require.Equal(t, input, prepareTagName(input))
	})
}
