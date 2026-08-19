package copy

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"math"
)

// maxGrainBytes caps a single VMDK grain allocation from untrusted headers.
const maxGrainBytes = 64 << 20 // 64 MiB

// grainDecompressor reuses a single zlib reader across grains to avoid
// per-grain allocations of the internal flate dictionary (~44 KiB each).
type grainDecompressor struct {
	br *bytes.Reader
	zr io.ReadCloser
}

func newGrainDecompressor() *grainDecompressor {
	return &grainDecompressor{br: bytes.NewReader(nil)}
}

func (d *grainDecompressor) decompress(compressed, buf []byte) ([]byte, error) {
	d.br.Reset(compressed)

	if d.zr == nil {
		zr, err := zlib.NewReader(d.br)
		if err != nil {
			return nil, err
		}
		d.zr = zr
	} else {
		if err := d.zr.(zlib.Resetter).Reset(d.br, nil); err != nil {
			return nil, err
		}
	}

	n, err := io.ReadFull(d.zr, buf)
	if err == io.ErrUnexpectedEOF {
		return buf[:n], nil
	}
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

// Stream-optimized VMDK format constants.
const (
	vmdkMagic      = 0x564d444b // "VMDK" (little-endian: 'K','D','M','V')
	sectorSize     = 512
	grainMarkerEOS = 0 // end-of-stream marker type
)

// sparseHeader field offsets within the 512-byte on-disk header.
const (
	hdrOffMagic    = 0
	hdrOffVersion  = 4
	hdrOffCapacity = 12
	hdrOffGrain    = 20
	hdrOffOverHead = 64
)

const grainMarkerSize = 12 // uint64 LBA + uint32 Size

// StreamToRaw reads a stream-optimized VMDK from r and writes the
// decompressed raw disk image to w. The writer must support Seek for
// sparse output (zero regions are skipped). onProgress is called with
// bytes written so far and total capacity; it may be nil.
func StreamToRaw(ctx context.Context, r io.Reader, w io.WriteSeeker, onProgress func(written, total int64)) error {
	var hdrBuf [sectorSize]byte
	if _, err := io.ReadFull(r, hdrBuf[:]); err != nil {
		return fmt.Errorf("vmdk: read header: %w", err)
	}

	magic := binary.LittleEndian.Uint32(hdrBuf[hdrOffMagic:])
	if magic != vmdkMagic {
		return fmt.Errorf("vmdk: bad magic 0x%08x (expected 0x%08x)", magic, vmdkMagic)
	}
	version := binary.LittleEndian.Uint32(hdrBuf[hdrOffVersion:])
	if version < 1 || version > 3 {
		return fmt.Errorf("vmdk: unsupported version %d", version)
	}

	capacity := binary.LittleEndian.Uint64(hdrBuf[hdrOffCapacity:])
	grainSizeSectors := binary.LittleEndian.Uint64(hdrBuf[hdrOffGrain:])
	overHead := binary.LittleEndian.Uint64(hdrBuf[hdrOffOverHead:])

	if capacity > uint64(math.MaxInt64/sectorSize) {
		return fmt.Errorf("vmdk: capacity %d sectors overflows byte size", capacity)
	}
	totalBytes := int64(capacity) * sectorSize

	if grainSizeSectors == 0 {
		return fmt.Errorf("vmdk: grain size is zero")
	}
	if grainSizeSectors > uint64(math.MaxInt64/sectorSize) {
		return fmt.Errorf("vmdk: grain size %d sectors overflows byte size", grainSizeSectors)
	}
	grainBytes := int64(grainSizeSectors) * sectorSize
	if grainBytes <= 0 {
		return fmt.Errorf("vmdk: grain size is non-positive")
	}
	if grainBytes > maxGrainBytes {
		return fmt.Errorf("vmdk: grain size %d exceeds limit %d", grainBytes, maxGrainBytes)
	}

	slog.Debug("vmdk stream parameters",
		"capacity", totalBytes,
		"grainBytes", grainBytes,
		"overhead", overHead,
	)

	// Skip the rest of the overhead area (descriptor, etc.) to reach grain data.
	skipSectors := int64(overHead) - 1
	if skipSectors > 0 {
		if _, err := io.CopyN(io.Discard, r, skipSectors*sectorSize); err != nil {
			return fmt.Errorf("vmdk: skip overhead: %w", err)
		}
	}

	// compBuf starts at grainBytes: compressed grains are almost always
	// smaller than the original. Grows on demand up to compBufLimit.
	sr := &vmdkStreamReader{
		r:            r,
		w:            w,
		decomp:       newGrainDecompressor(),
		compBuf:      make([]byte, grainBytes),
		grainBuf:     make([]byte, grainBytes),
		compBufLimit: int(grainBytes) * 2,
		totalBytes:   totalBytes,
		onProgress:   onProgress,
	}
	var markerBuf [grainMarkerSize]byte

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		if _, err := io.ReadFull(r, markerBuf[:]); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break
			}
			return fmt.Errorf("vmdk: read grain marker: %w", err)
		}
		lba := binary.LittleEndian.Uint64(markerBuf[0:8])
		size := binary.LittleEndian.Uint32(markerBuf[8:12])

		if size == 0 {
			eos, err := sr.processMetadataMarker(lba)
			if err != nil {
				return err
			}
			if eos {
				break
			}
			continue
		}

		if err := sr.processCompressedGrain(lba, size); err != nil {
			return err
		}
	}

	return nil
}

// vmdkStreamReader holds shared state for processing a stream-optimized VMDK.
type vmdkStreamReader struct {
	r            io.Reader
	w            io.WriteSeeker
	decomp       *grainDecompressor
	compBuf      []byte
	grainBuf     []byte
	compBufLimit int
	written      int64
	totalBytes   int64
	onProgress   func(written, total int64)
}

// processMetadataMarker reads a metadata or EOS marker from the stream.
// It returns (true, nil) when an end-of-stream condition is reached.
func (sr *vmdkStreamReader) processMetadataMarker(lba uint64) (bool, error) {
	// Metadata or EOS marker. Read the type field to decide.
	var typeBuf [4]byte
	if _, err := io.ReadFull(sr.r, typeBuf[:]); err != nil {
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return true, nil
		}
		return false, fmt.Errorf("vmdk: read marker type: %w", err)
	}
	markerType := binary.LittleEndian.Uint32(typeBuf[:])
	// Type 0 = EOS, 1 = grain table, 2 = grain directory, 3 = footer.
	if markerType == grainMarkerEOS {
		return true, nil
	}
	// Metadata markers occupy a full sector. We've read 12 + 4 = 16
	// bytes so far; skip the remaining padding to the sector boundary.
	const metaMarkerHdr = grainMarkerSize + 4 // 16 bytes consumed
	markerPad := sectorSize - metaMarkerHdr
	if _, err := io.CopyN(io.Discard, sr.r, int64(markerPad)); err != nil {
		return false, fmt.Errorf("vmdk: skip marker padding (type %d): %w", markerType, err)
	}
	// For grain table / grain directory / footer markers, LBA holds
	// the number of sectors of metadata that follow.
	metaBytes := int64(lba) * sectorSize
	slog.Debug("vmdk: skipping metadata marker",
		"type", markerType,
		"sectors", lba,
		"bytes", metaBytes,
	)
	if metaBytes > 0 {
		if _, err := io.CopyN(io.Discard, sr.r, metaBytes); err != nil {
			return false, fmt.Errorf("vmdk: skip metadata (type %d): %w", markerType, err)
		}
	}
	return false, nil
}

// processCompressedGrain reads, decompresses, and writes a single grain.
func (sr *vmdkStreamReader) processCompressedGrain(lba uint64, size uint32) error {
	dataSize := int(size)
	if dataSize > sr.compBufLimit {
		return fmt.Errorf("vmdk: compressed grain at LBA %d claims %d bytes (limit %d)", lba, dataSize, sr.compBufLimit)
	}
	if dataSize > len(sr.compBuf) {
		sr.compBuf = make([]byte, dataSize)
	}
	if _, err := io.ReadFull(sr.r, sr.compBuf[:dataSize]); err != nil {
		return fmt.Errorf("vmdk: read grain data at LBA %d: %w", lba, err)
	}

	// Pad to sector boundary (stream-optimized grains are sector-aligned).
	padBytes := (sectorSize - (grainMarkerSize+dataSize)%sectorSize) % sectorSize
	if padBytes > 0 {
		var padBuf [sectorSize]byte
		if _, err := io.ReadFull(sr.r, padBuf[:padBytes]); err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			return fmt.Errorf("vmdk: read grain padding at LBA %d: %w", lba, err)
		}
	}

	decompressed, err := sr.decomp.decompress(sr.compBuf[:dataSize], sr.grainBuf)
	if err != nil {
		return fmt.Errorf("vmdk: decompress grain at LBA %d: %w", lba, err)
	}

	if lba > uint64(math.MaxInt64/sectorSize) {
		return fmt.Errorf("vmdk: grain LBA %d overflows offset", lba)
	}
	offset := int64(lba) * sectorSize
	end := offset + int64(len(decompressed))
	if end < offset {
		return fmt.Errorf("vmdk: grain at LBA %d write range overflows", lba)
	}
	if offset >= sr.totalBytes || end > sr.totalBytes {
		return fmt.Errorf("vmdk: grain at LBA %d writes beyond capacity (offset=%d end=%d capacity=%d)",
			lba, offset, end, sr.totalBytes)
	}

	if _, err := sr.w.Seek(offset, io.SeekStart); err != nil {
		return fmt.Errorf("vmdk: seek to offset %d: %w", offset, err)
	}
	if _, err := sr.w.Write(decompressed); err != nil {
		return fmt.Errorf("vmdk: write grain at LBA %d: %w", lba, err)
	}

	sr.written += int64(len(decompressed))
	if sr.onProgress != nil {
		sr.onProgress(sr.written, sr.totalBytes)
	}
	return nil
}
