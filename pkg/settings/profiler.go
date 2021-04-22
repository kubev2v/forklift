package settings

import (
	"os"
	"time"
)

//
// Environment variables.
const (
	ProfilePath     = "PROFILE_PATH"
	ProfileDuration = "PROFILE_DURATION"
	ProfileKind     = "PROFILE_KIND"
	ProfileMemory   = "memory"
	ProfileCpu      = "cpu"
	ProfileMutex    = "mutex"
)

//
// Profiler settings
type Profiler struct {
	// Profiler output directory.
	Path string
	// Profiler duration (minutes).
	Duration time.Duration
	//
	Kind string
}

//
// Load settings.
func (r *Profiler) Load() error {
	minutes, _ := getEnvLimit(ProfileDuration, 0)
	r.Duration = time.Duration(minutes) * time.Minute
	if s, found := os.LookupEnv(ProfilePath); found {
		r.Path = s
	}
	if s, found := os.LookupEnv(ProfileKind); found {
		r.Kind = s
	} else {
		r.Kind = ProfileMemory
	}
	return nil
}
