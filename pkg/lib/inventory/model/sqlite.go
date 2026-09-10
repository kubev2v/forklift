package model

import (
	"errors"
	"fmt"
	"os"
	"strings"

	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	sqlite3 "modernc.org/sqlite"
	sqlite3lib "modernc.org/sqlite/lib"
)

const (
	// DefaultIOErrRebuildThreshold is the number of consecutive SQLITE_IOERR*
	// failures after which the inventory cache DB is discarded and rebuilt.
	DefaultIOErrRebuildThreshold = 3
	// sqlitePrimaryResultCodeMask isolates the primary result code from an
	// extended SQLITE_IOERR_* code (for example SQLITE_IOERR_SHORT_READ = 522).
	sqlitePrimaryResultCodeMask = 0xFF
)

// IsIOErr reports whether err is SQLITE_IOERR or any SQLITE_IOERR_* extended
// code (SQLITE_IOERR_SHORT_READ, SQLITE_IOERR_READ, and so on).
func IsIOErr(err error) bool {
	if err == nil {
		return false
	}
	var sqliteErr *sqlite3.Error
	if errors.As(err, &sqliteErr) {
		return isIOErrCode(sqliteErr.Code())
	}
	return strings.Contains(err.Error(), "SQLITE_IOERR")
}

func isIOErrCode(code int) bool {
	return code&sqlitePrimaryResultCodeMask == sqlite3lib.SQLITE_IOERR
}

// Rebuild discards the cache DB (including WAL/SHM sidecars) and reopens an
// empty schema. Collectors call this to self-heal after persistent sqlite I/O
// errors instead of retrying against a damaged file indefinitely.
func Rebuild(db DB) error {
	if db == nil {
		return liberr.New("rebuild requested with nil DB")
	}
	if client, ok := db.(*Client); ok {
		return client.Rebuild()
	}
	return nil
}

// Rebuild closes the current DB, deletes the cache files, and opens a fresh
// empty database at the same path.
func (r *Client) Rebuild() (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("rebuild inventory cache: %v", p)
		}
	}()
	r.log.Info("Rebuilding inventory cache DB.")
	_ = r.Close(true)
	return r.Open(false)
}

func removeDBFiles(path string) {
	for _, p := range []string{path, path + "-wal", path + "-shm", path + "-journal"} {
		_ = os.Remove(p)
	}
}

// IOErrHealer counts consecutive SQLITE_IOERR* failures and rebuilds the
// inventory cache after Threshold hits. Non-IO errors reset the counter.
type IOErrHealer struct {
	DB        DB
	Log       logging.LevelLogger
	Threshold int
	count     int
}

// Observe records err. After Threshold consecutive SQLITE_IOERR* failures it
// discards and rebuilds the cache DB. Returns true when a rebuild ran.
func (h *IOErrHealer) Observe(err error) (rebuilt bool) {
	if h == nil {
		return false
	}
	if err == nil || !IsIOErr(err) {
		h.count = 0
		return false
	}
	h.count++
	threshold := h.Threshold
	if threshold <= 0 {
		threshold = DefaultIOErrRebuildThreshold
	}
	if h.Log != nil {
		h.Log.Error(
			err,
			"sqlite IO error on inventory cache.",
			"consecutive",
			h.count,
			"threshold",
			threshold)
	}
	if h.count < threshold {
		return false
	}
	h.count = 0
	if rebuildErr := Rebuild(h.DB); rebuildErr != nil {
		if h.Log != nil {
			h.Log.Error(
				rebuildErr,
				"failed to rebuild inventory cache after sqlite IO errors.")
		}
		return false
	}
	if h.Log != nil {
		h.Log.Info(
			"inventory cache discarded and rebuilt after consecutive sqlite IO errors.")
	}
	return true
}
