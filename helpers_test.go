package echo_sentry_middleware

import (
	"math/rand"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

func Test_limitTagValue(t *testing.T) {
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

func Test_generateToken(t *testing.T) {
	rand.Seed(time.Now().UnixMicro())
	count := rand.Intn(20)
	for tt := 0; tt < count; tt++ {
		require.Equal(t, 32, len(generateToken()))
	}
}

func Test_getRequestID(t *testing.T) {
	e := echo.New()

	t.Run("token in header", func(t *testing.T) {
		req := httptest.NewRequest(echo.GET, "/", nil)
		req.Header.Set(echo.HeaderXRequestID, "test")
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		require.Equal(t, "test", getRequestID(c))
	})

	t.Run("generate token", func(t *testing.T) {
		req := httptest.NewRequest(echo.GET, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		require.Equal(t, 32, len(getRequestID(c)))
	})
}
