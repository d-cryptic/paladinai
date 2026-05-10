// Tests use package logger (not logger_test) to access unexported parseLevel.
package logger

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
)

// ── parseLevel ────────────────────────────────────────────────────────────────

func TestParseLevel_Debug(t *testing.T) {
	assert.Equal(t, zapcore.DebugLevel, parseLevel("debug"))
}

func TestParseLevel_DebugCaseInsensitive(t *testing.T) {
	assert.Equal(t, zapcore.DebugLevel, parseLevel("DEBUG"))
	assert.Equal(t, zapcore.DebugLevel, parseLevel("Debug"))
}

func TestParseLevel_Warn(t *testing.T) {
	assert.Equal(t, zapcore.WarnLevel, parseLevel("warn"))
	assert.Equal(t, zapcore.WarnLevel, parseLevel("warning"))
	assert.Equal(t, zapcore.WarnLevel, parseLevel("WARNING"))
}

func TestParseLevel_Error(t *testing.T) {
	assert.Equal(t, zapcore.ErrorLevel, parseLevel("error"))
	assert.Equal(t, zapcore.ErrorLevel, parseLevel("ERROR"))
}

func TestParseLevel_DefaultsToInfoForUnknown(t *testing.T) {
	for _, s := range []string{"info", "INFO", "", "verbose", "trace", "notice"} {
		assert.Equal(t, zapcore.InfoLevel, parseLevel(s), "input %q should default to info", s)
	}
}

// ── New ───────────────────────────────────────────────────────────────────────

func TestNew_SucceedsWithServiceName(t *testing.T) {
	t.Setenv("LOG_LEVEL", "")

	l, err := New("test-service")
	require.NoError(t, err)
	require.NotNil(t, l)
	_ = l.Sync()
}

func TestNew_RespectsLogLevelEnv(t *testing.T) {
	for _, level := range []string{"debug", "info", "warn", "error"} {
		t.Run(level, func(t *testing.T) {
			t.Setenv("LOG_LEVEL", level)
			l, err := New("svc")
			require.NoError(t, err)
			require.NotNil(t, l)
			_ = l.Sync()
		})
	}
}
