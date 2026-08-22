/*
Copyright 2026 Infinidat
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
