// Copyright © 2025 Dell Inc. or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//      http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package log

import (
	"log/slog"
	"os"
	"sync"
)

var (
	mu       sync.Mutex           // guards logLevel
	logLevel = new(slog.LevelVar) // Info by default
	// Log is a logger for goscaleio and api packages to use
	Log   = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel}))
	debug = false // False by default, will turn true if level is set to debug
)

func SetLogLevel(level slog.Level) {
	mu.Lock()
	defer mu.Unlock()
	logLevel.Set(level)
	if level == slog.LevelDebug {
		debug = true
	} else {
		debug = false
	}
}

func DoLog(
	l func(msg string, args ...any),
	msg string,
) {
	if debug {
		l(msg)
	}
}
