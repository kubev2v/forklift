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
