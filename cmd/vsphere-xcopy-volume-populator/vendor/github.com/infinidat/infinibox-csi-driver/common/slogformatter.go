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
	"path"
	"time"
)

func CustomSlogFormatter(groups []string, a slog.Attr) slog.Attr {
	if a.Key == slog.LevelKey {
		// Handle custom level values.

		level := a.Value.Any().(slog.Level)
		switch {

		case level < slog.LevelDebug:
			a.Value = slog.StringValue("TRACE")
		}
	}
	if a.Key == slog.TimeKey {
		// Cast the value to time.Time
		t := a.Value.Any().(time.Time)
		// Format the time as desired (e.g., "2006-01-02 15:04:05 MST")
		a.Value = slog.StringValue(t.Format("2006-01-02 15:04:05.000 MST"))
	}
	if a.Key == slog.SourceKey {
		s := a.Value.Any().(*slog.Source)
		s.File = path.Base(s.File)
		s.Function = path.Base(s.Function)
	}
	return a
}

func CustomControllerTimeFormatter(groups []string, a slog.Attr) slog.Attr {
	if a.Key == slog.TimeKey {
		// Cast the value to time.Time
		t := a.Value.Any().(time.Time)
		// Format the time as desired (e.g., "2006-01-02 15:04:05 MST")
		a.Value = slog.StringValue(t.Format("2006-01-02 15:04:05.000 MST"))
	}
	return a
}
