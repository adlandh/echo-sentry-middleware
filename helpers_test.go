package echosentrymiddleware

import (
	"net/http/httptest"
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
			require.True(t, len(prepareTagValue(tt.str)) <= 200)
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
			require.True(t, len(prepareTagName(tt.str)) <= 32)
			require.Equal(t, tt.want, prepareTagName(tt.str))
		})
	}
}

func TestGetRequestID(t *testing.T) {
	e := echo.New()

	t.Run("token in header", func(t *testing.T) {
		r := httptest.NewRequest(echo.GET, "/", nil)
		r.Header.Set(echo.HeaderXRequestID, "test")
		w := httptest.NewRecorder()
		c := e.NewContext(r, w)
		e.ServeHTTP(w, r)
		require.Equal(t, "test", getRequestID(c))
	})

	t.Run("generate token", func(t *testing.T) {
		e.Use(middleware.RequestID())
		r := httptest.NewRequest(echo.GET, "/", nil)
		w := httptest.NewRecorder()
		c := e.NewContext(r, w)
		e.ServeHTTP(w, r)
		require.Equal(t, 32, len(getRequestID(c)))
	})
}
