package echosentrymiddleware

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
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
			str:  "05Kj7z2AXCl603gMJu6B23z2sD05Kj7z2AX",
			want: "05Kj7z2AXCl603gMJu6B23z2sD05Kj7z",
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
		r := httptest.NewRequest(echo.GET, "/", nil)
		r.Header.Set(echo.HeaderXRequestID, "test")
		w := httptest.NewRecorder()
		c := e.NewContext(r, w)
		e.ServeHTTP(w, r)
		require.Equal(t, "test", getRequestID(c))
	})

	t.Run("generate token", func(t *testing.T) {
		e := echo.New()
		e.Use(middleware.RequestID())
		r := httptest.NewRequest(echo.GET, "/", nil)
		w := httptest.NewRecorder()
		c := e.NewContext(r, w)
		e.ServeHTTP(w, r)
		require.Equal(t, 32, len(getRequestID(c)))
	})

	t.Run("no token without middleware", func(t *testing.T) {
		e := echo.New()
		r := httptest.NewRequest(echo.GET, "/", nil)
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
}

func TestLimitTagNameEdgeCases(t *testing.T) {
	t.Run("empty string", func(t *testing.T) {
		require.Equal(t, "", prepareTagName(""))
	})

	t.Run("exactly max length", func(t *testing.T) {
		input := "0123456789abcdef0123456789abcdef"
		require.Len(t, input, MaxTagNameLength)
		require.Equal(t, input, prepareTagName(input))
	})
}
