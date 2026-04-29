package vddk

import (
	"sync"
)

var (
	mu     sync.RWMutex
	libDir string
)

// SetLibDir sets the VDDK library directory path for internal use
func SetLibDir(dir string) {
	mu.Lock()
	defer mu.Unlock()
	libDir = dir
}

// GetLibDir returns the configured VDDK library directory
// Returns empty string if not set (caller must set it via SetLibDir)
func GetLibDir() string {
	mu.RLock()
	defer mu.RUnlock()
	return libDir
}

// GetLibPath returns the full library path (lib64 subdirectory)
// Used for LD_LIBRARY_PATH filtering
func GetLibPath() string {
	dir := GetLibDir()
	if dir == "" {
		return ""
	}
	return dir + "/lib64"
}
