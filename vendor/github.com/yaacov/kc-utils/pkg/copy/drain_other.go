//go:build !linux

package copy

import "os"

// drainWriter is a no-op pass-through on non-Linux platforms.
// Page cache management is only relevant in Linux cgroups (containers).
type drainWriter struct {
	f *os.File
}

func newDrainWriter(f *os.File) *drainWriter {
	return &drainWriter{f: f}
}

func (d *drainWriter) Write(p []byte) (int, error) {
	return d.f.Write(p)
}

func (d *drainWriter) Seek(offset int64, whence int) (int64, error) {
	return d.f.Seek(offset, whence)
}

func (d *drainWriter) Flush() {}
