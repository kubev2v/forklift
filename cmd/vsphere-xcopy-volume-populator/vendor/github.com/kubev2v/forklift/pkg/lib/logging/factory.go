package logging

import (
	"os"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Builder.
type Builder interface {
	New() logr.Logger
	V(int, logr.Logger) logr.Logger
}

// Zap builder factory.
type ZapBuilder struct {
}

// Build new logger.
func (b *ZapBuilder) New() (logger logr.Logger) {
	var encoder zapcore.Encoder
	sinker := zapcore.AddSync(os.Stderr)
	level := zap.NewAtomicLevelAt(zap.DebugLevel)
	options := []zap.Option{
		zap.AddStacktrace(zap.ErrorLevel),
		zap.ErrorOutput(sinker),
		zap.AddCallerSkip(1),
	}
	if Settings.Development {
		cfg := zap.NewDevelopmentEncoderConfig()
		cfg.EncodeTime = zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05.000")
		encoder = zapcore.NewConsoleEncoder(cfg)
	} else {
		cfg := zap.NewProductionEncoderConfig()
		cfg.EncodeTime = zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05.000")
		encoder = zapcore.NewJSONEncoder(cfg)
	}

	logger = zapr.NewLogger(
		zap.New(
			zapcore.NewCore(
				encoder,
				sinker,
				level)).WithOptions(options...))

	return
}

// Debug logger.
func (b *ZapBuilder) V(level int, in logr.Logger) (l logr.Logger) {
	if Settings.atDebug(level) {
		l = in.V(1)
	} else {
		l = in.V(0)
	}

	return
}
