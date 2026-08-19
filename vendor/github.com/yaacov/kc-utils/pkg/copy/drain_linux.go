//go:build linux

package copy

import (
	"log/slog"
	"os"
	"syscall"
)

const (
	drainInterval     = 32 << 20 // flush + drop page cache every 32 MiB written
	posixFadvDontneed = 4        // POSIX_FADV_DONTNEED
)

// drainWriter wraps an *os.File and periodically calls fdatasync +
// fadvise(FADV_DONTNEED) to release page cache back to the kernel.
// Without this, cgroup memory grows by the total amount written
// (3-4 GiB for typical VM disks) because every write() goes through
// the kernel page cache.
type drainWriter struct {
	f     *os.File
	fd    int
	dirty int64
}

func newDrainWriter(f *os.File) *drainWriter {
	return &drainWriter{f: f, fd: int(f.Fd())}
}

func (d *drainWriter) Write(p []byte) (int, error) {
	n, err := d.f.Write(p)
	d.dirty += int64(n)
	if d.dirty >= drainInterval {
		d.drain()
	}
	return n, err
}

func (d *drainWriter) Seek(offset int64, whence int) (int64, error) {
	return d.f.Seek(offset, whence)
}

// Flush forces a final drain of any remaining dirty data.
func (d *drainWriter) Flush() {
	if d.dirty > 0 {
		d.drain()
	}
}

func (d *drainWriter) drain() {
	if err := syscall.Fdatasync(d.fd); err != nil {
		slog.Debug("page cache drain: fdatasync failed", "error", err)
	}
	_, _, errno := syscall.Syscall6(
		syscall.SYS_FADVISE64,
		uintptr(d.fd), 0, 0, posixFadvDontneed, 0, 0,
	)
	if errno != 0 {
		slog.Debug("page cache drain: fadvise DONTNEED failed", "error", errno)
	}
	d.dirty = 0
}
