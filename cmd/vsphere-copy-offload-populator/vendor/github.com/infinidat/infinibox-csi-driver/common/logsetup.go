package common

import (
	"log/slog"
	"os"
)

func SetupSlog(isController bool) *slog.Logger {
	appLogFormat := os.Getenv("APP_LOG_FORMAT")
	appLogLevel := os.Getenv("APP_LOG_LEVEL")
	var logLevel slog.Leveler
	switch appLogLevel {
	case "error":
		logLevel = slog.LevelError
	case "warn":
		logLevel = slog.LevelWarn
	case "info":
		logLevel = slog.LevelInfo
	case "debug":
		logLevel = slog.LevelDebug
	case "trace":
		logLevel = slog.LevelDebug
	default:
		logLevel = slog.LevelInfo
	}
	slogOpts := &slog.HandlerOptions{
		Level:       logLevel,
		AddSource:   true,
		ReplaceAttr: CustomSlogFormatter,
	}
	// TODO figure out why custom controllers need this formatter and not the normal one
	if isController {
		slogOpts.ReplaceAttr = CustomControllerTimeFormatter
	}
	thisLogger := slog.New(slog.NewJSONHandler(os.Stdout, slogOpts))
	if appLogFormat == "text" {
		thisLogger = slog.New(slog.NewTextHandler(os.Stdout, slogOpts))
	}
	return thisLogger
}
