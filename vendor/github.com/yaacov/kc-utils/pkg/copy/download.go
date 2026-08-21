package copy

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

const downloadBufSize = 256 << 10 // 256 KiB read buffer per disk

type nfcDownloader interface {
	downloadURL(ctx context.Context, rawURL string) (io.ReadCloser, error)
}

// CopyDisk downloads a single disk from an NFC export URL and writes raw
// data to the target path (block device or filesystem disk.img).
// Downloads use the govmomi client on lease so ESXi thumbprints from the NFC
// lease are honored (see Lease.downloadURL).
func CopyDisk(ctx context.Context, lease *Lease, disk DiskURL, target Target, onProgress func(pct int)) error {
	return copyDiskFromDownloader(ctx, lease, disk, target, onProgress)
}

func copyDiskFromDownloader(ctx context.Context, dl nfcDownloader, disk DiskURL, target Target, onProgress func(pct int)) error {
	if err := ensureTargetFile(target); err != nil {
		return err
	}

	flags := os.O_WRONLY
	if !target.IsBlockDev {
		flags |= os.O_CREATE | os.O_TRUNC
	}
	f, err := os.OpenFile(target.Path, flags, 0o660)
	if err != nil {
		return fmt.Errorf("open target %s: %w", target.Path, err)
	}
	defer f.Close()

	started := time.Now()
	slog.Info("starting disk download",
		"index", target.Index,
		"source", disk.DiskPath,
		"target", target.Path,
		"block", target.IsBlockDev,
		"size", disk.Size,
	)

	body, err := dl.downloadURL(ctx, disk.URL)
	if err != nil {
		return fmt.Errorf("HTTP GET %s: %w", disk.DiskPath, err)
	}
	defer body.Close()

	lastPct := -1
	progressCb := func(written, total int64) {
		if total <= 0 || onProgress == nil {
			return
		}
		pct := int(written * 100 / total)
		if pct > 100 {
			pct = 100
		}
		if shouldLogProgress(pct, lastPct) {
			lastPct = pct
			onProgress(pct)
		}
	}

	dw := newDrainWriter(f)

	br := bufio.NewReaderSize(body, downloadBufSize)
	if err := StreamToRaw(ctx, br, dw, progressCb); err != nil {
		return fmt.Errorf("stream to raw %s: %w", target.Path, err)
	}

	dw.Flush()

	if target.IsBlockDev {
		if err := f.Sync(); err != nil {
			return fmt.Errorf("sync block device %s: %w", target.Path, err)
		}
	}

	slog.Info("disk download finished",
		"index", target.Index,
		"source", disk.DiskPath,
		"target", target.Path,
		"duration", time.Since(started).Round(time.Second).String(),
	)
	return nil
}

// shouldLogProgress reports 0%, 100%, the first sample, and each new 5% bucket.
func shouldLogProgress(pct, lastLogged int) bool {
	if pct == lastLogged {
		return false
	}
	if pct == 0 || pct == 100 || lastLogged < 0 {
		return true
	}
	return pct/5 > lastLogged/5
}

func ensureTargetFile(target Target) error {
	if target.IsBlockDev {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(target.Path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(target.Path, os.O_CREATE|os.O_WRONLY, 0o660)
	if err != nil {
		return err
	}
	return f.Close()
}
