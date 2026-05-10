package logger

import (
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New returns a production-ready zap logger.
// LOG_LEVEL env var controls verbosity (debug, info, warn, error).
func New(service string) (*zap.Logger, error) {
	level := parseLevel(os.Getenv("LOG_LEVEL"))

	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "ts"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	cfg := zap.Config{
		Level:             zap.NewAtomicLevelAt(level),
		Development:       level == zapcore.DebugLevel,
		DisableCaller:     false,
		DisableStacktrace: level != zapcore.DebugLevel,
		Sampling:          nil,
		Encoding:          "json",
		EncoderConfig:     encoderCfg,
		OutputPaths:       []string{"stdout"},
		ErrorOutputPaths:  []string{"stderr"},
		InitialFields:     map[string]interface{}{"service": service},
	}

	return cfg.Build()
}

// Must panics if logger construction fails (use only in main).
func Must(service string) *zap.Logger {
	l, err := New(service)
	if err != nil {
		panic(err)
	}
	return l
}

func parseLevel(s string) zapcore.Level {
	switch strings.ToLower(s) {
	case "debug":
		return zapcore.DebugLevel
	case "warn", "warning":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}
