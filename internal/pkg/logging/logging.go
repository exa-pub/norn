// Package logging provides a global zap logger singleton.
package logging

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var global *zap.Logger

// Init initialises the global zap logger. Call once at startup.
// dev=true selects human-readable output; dev=false selects JSON.
func Init(dev bool) {
	if dev {
		cfg := zap.NewDevelopmentConfig()
		// Only add stack traces for Error and above — WARN stack traces
		// are too noisy for recurring operational warnings.
		cfg.EncoderConfig.StacktraceKey = "stacktrace"
		global, _ = cfg.Build(zap.AddStacktrace(zapcore.ErrorLevel))
	} else {
		global, _ = zap.NewProduction()
	}
	zap.ReplaceGlobals(global)
}

// L returns the global logger.
func L() *zap.Logger { return zap.L() }

// S returns the global sugared logger.
func S() *zap.SugaredLogger { return zap.S() }

// Sync flushes buffered log entries. Call on shutdown.
func Sync() {
	if global != nil {
		_ = global.Sync()
	}
}
