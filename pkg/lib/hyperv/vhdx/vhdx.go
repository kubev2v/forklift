// Package vhdx derives the changed-block map of a VHDX differencing disk
// (AVHDX) by walking the file's own structures, per [MS-VHDX].
//
// Hyper-V on Windows Server 2019 exposes no Resilient Change Tracking API,
// so the checkpoint chain itself is the change log: each checkpoint freezes
// the active differencing disk, and the blocks present in that frozen AVHDX
// are exactly the blocks written since the previous checkpoint. QEMU cannot
// open an AVHDX, so the map is read here instead.
//
// The region table locates the BAT and metadata regions, the metadata items
// give block size, virtual disk size and logical sector size, and the BAT
// (with its interleaved sector-bitmap entries) says which blocks the
// differencing disk holds. Blocks that still live in the parent chain are
// reported as absent so the caller leaves the target untouched there.
// PAYLOAD_BLOCK_ZERO is reported as an explicit zero range so a prior full
// copy's stale data is cleared.
//
// [MS-VHDX]: https://learn.microsoft.com/openspecs/windows_protocols/ms-vhdx
package vhdx

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"sort"
)

const (
	kiB = 1024
	miB = 1024 * kiB
)

// Fixed header-section layout (MS-VHDX 3.1).
const (
	headerOffset            = 64 * kiB
	headerBackupOffset      = 128 * kiB
	headerSize              = 4096
	regionTableOffset       = 192 * kiB
	regionTableBackupOffset = 256 * kiB
	regionTableSize         = 64 * kiB
	regionTableHeaderSize   = 16
	regionTableEntrySize    = 32
)

// PAYLOAD_BLOCK_STATUS values (MS-VHDX 2.3.3.1). Present states carry data
// in the differencing disk. ZERO must clear the target. The rest resolve
// through the parent chain already written by the full copy pass.
const (
	blockNotPresent       = 0
	blockUndefined        = 1
	blockZero             = 2
	blockUnmapped         = 3
	blockFullyPresent     = 6
	blockPartiallyPresent = 7
)

// BAT entry encoding (MS-VHDX 2.3.3.1): state in bits 0-2, file offset in
// 1 MiB units in bits 20-63. Masking bits 20-63 yields the offset in bytes.
const (
	batStateMask      = 0x07
	batFileOffsetMask = 0xFFFFFFFFFFF00000
	batEntrySize      = 8
)

// Max sectors a 1 MiB sector-bitmap block can represent (MS-VHDX 2.3.2).
const maxSectorsPerBitmapBlock = 1 << 23

// Table entry-count ceilings from the spec, used to reject malformed files
// before any size-derived allocation.
const (
	maxRegionEntries     = 2047
	maxMetadataEntries   = 2047
	maxMetadataItemBytes = miB // MS-VHDX 3.5
)

// zeroFileOffset marks a Range that must zero the target instead of reading
// from the AVHDX (PAYLOAD_BLOCK_ZERO).
const zeroFileOffset = int64(-1)

// Region GUIDs (MS-VHDX 3.2), in stored little-endian byte order.
var (
	// batRegionGUID is 2DC27766-F623-4200-9D64-115E9BFD4A08.
	batRegionGUID = guid{0x66, 0x77, 0xc2, 0x2d, 0x23, 0xf6, 0x00, 0x42,
		0x9d, 0x64, 0x11, 0x5e, 0x9b, 0xfd, 0x4a, 0x08}
	// metadataRegionGUID is 8B7CA206-4790-4B9A-B8FE-575F050F886E.
	metadataRegionGUID = guid{0x06, 0xa2, 0x7c, 0x8b, 0x90, 0x47, 0x9a, 0x4b,
		0xb8, 0xfe, 0x57, 0x5f, 0x05, 0x0f, 0x88, 0x6e}
)

// Metadata item GUIDs (MS-VHDX 3.4.1), in stored little-endian byte order.
var (
	// fileParametersGUID is CAA16737-FA36-4D43-B3B6-33F0AA44E76B.
	fileParametersGUID = guid{0x37, 0x67, 0xa1, 0xca, 0x36, 0xfa, 0x43, 0x4d,
		0xb3, 0xb6, 0x33, 0xf0, 0xaa, 0x44, 0xe7, 0x6b}
	// virtualDiskSizeGUID is 2FA54224-CD1B-4876-B211-5DBED83BF4B8.
	virtualDiskSizeGUID = guid{0x24, 0x42, 0xa5, 0x2f, 0x1b, 0xcd, 0x76, 0x48,
		0xb2, 0x11, 0x5d, 0xbe, 0xd8, 0x3b, 0xf4, 0xb8}
	// logicalSectorSizeGUID is 8141BF1D-A96F-4709-BA47-F233A8FAAB5F.
	logicalSectorSizeGUID = guid{0x1d, 0xbf, 0x41, 0x81, 0x6f, 0xa9, 0x09, 0x47,
		0xba, 0x47, 0xf2, 0x33, 0xa8, 0xfa, 0xab, 0x5f}
)

type guid [16]byte

var castagnoli = crc32.MakeTable(crc32.Castagnoli)

// Range is a contiguous extent present in the differencing disk.
// VirtualOffset is where it belongs on the target (its offset within the
// virtual disk). FileOffset is where to read it from in the AVHDX, or
// zeroFileOffset when the extent must be zeroed on the target.
type Range struct {
	VirtualOffset int64
	FileOffset    int64
	Length        int64
}

// Geometry describes the differencing disk's layout, for logging and for
// callers that need to validate the target device size.
type Geometry struct {
	BlockSize   int64
	SectorSize  int64
	VirtualSize int64
	ChunkRatio  int64
	TotalBlocks int64
}

// errf builds a package-prefixed error. It routes through fmt.Errorf so
// that %w in a format string still wraps the underlying I/O error.
func errf(format string, a ...any) error {
	return fmt.Errorf("vhdx: "+format, a...)
}

// ChangedRanges returns the extents that the differencing disk holds, in
// ascending virtual-offset order, together with the disk geometry.
//
// Blocks in state NOT_PRESENT, UNDEFINED or UNMAPPED are omitted: their
// contents resolve through the parent chain. ZERO blocks are returned as
// zero-fill ranges so stale target data from a prior full copy is cleared.
func ChangedRanges(r io.ReaderAt) (ranges []Range, geo Geometry, err error) {
	if err := checkLogEmpty(r); err != nil {
		return nil, geo, err
	}
	regions, err := readRegionTable(r)
	if err != nil {
		return nil, geo, err
	}
	batRegion, ok := regions[batRegionGUID]
	if !ok {
		return nil, geo, errf("BAT region not present in region table")
	}
	metaRegion, ok := regions[metadataRegionGUID]
	if !ok {
		return nil, geo, errf("metadata region not present in region table")
	}

	geo, err = readGeometry(r, metaRegion)
	if err != nil {
		return nil, geo, err
	}

	fully, partial, zeros, bitmaps, err := walkBAT(r, batRegion, geo)
	if err != nil {
		return nil, geo, err
	}

	ranges = make([]Range, 0, len(fully)+len(partial)+len(zeros))
	for _, idx := range fully {
		off := idx.block * geo.BlockSize
		length := clampLength(off, geo.BlockSize, geo.VirtualSize)
		if length <= 0 {
			continue
		}
		ranges = append(ranges, Range{
			VirtualOffset: off,
			FileOffset:    idx.fileOffset,
			Length:        length,
		})
	}
	for _, idx := range zeros {
		off := idx.block * geo.BlockSize
		length := clampLength(off, geo.BlockSize, geo.VirtualSize)
		if length <= 0 {
			continue
		}
		ranges = append(ranges, Range{
			VirtualOffset: off,
			FileOffset:    zeroFileOffset,
			Length:        length,
		})
	}
	for _, idx := range partial {
		sub, err := bitmapRanges(r, idx, bitmaps, geo)
		if err != nil {
			return nil, geo, err
		}
		ranges = append(ranges, sub...)
	}
	sortByVirtualOffset(ranges)
	return ranges, geo, nil
}

// clampLength returns how many bytes of a block starting at off fit inside
// VirtualSize. VHDX virtual sizes are sector-aligned, not block-aligned.
func clampLength(off, blockSize, virtualSize int64) int64 {
	if off >= virtualSize {
		return 0
	}
	length := blockSize
	if off+length > virtualSize {
		length = virtualSize - off
	}
	return length
}

// checkLogEmpty rejects a VHDX whose current header has a non-zero LogGuid
// (MS-VHDX 2.2.1). We do not replay. LogLength is only the allocated region
// size (often 1 MiB) — ignore it for activity.
func checkLogEmpty(r io.ReaderAt) error {
	hdr, err := currentHeader(r)
	if err != nil {
		return err
	}
	var logGuid guid
	copy(logGuid[:], hdr[48:64]) // LogGuid @ header+48
	if logGuid != (guid{}) {
		return errf("active VHDX log (LogGuid non-zero); replay is not supported")
	}
	return nil
}

// currentHeader picks the valid header (signature + CRC-32C) with the
// higher SequenceNumber (MS-VHDX "current" header).
func currentHeader(r io.ReaderAt) ([]byte, error) {
	var (
		best     []byte
		bestSeq  uint64
		found    bool
		firstErr error
	)
	for _, at := range []int64{headerOffset, headerBackupOffset} {
		hdr := make([]byte, headerSize)
		if _, err := r.ReadAt(hdr, at); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if string(hdr[0:4]) != "head" {
			if firstErr == nil {
				firstErr = errf("bad header signature at %d", at)
			}
			continue
		}
		stored := binary.LittleEndian.Uint32(hdr[4:8])
		zeroed := make([]byte, headerSize)
		copy(zeroed, hdr)
		copy(zeroed[4:8], []byte{0, 0, 0, 0})
		if crc32.Checksum(zeroed, castagnoli) != stored {
			if firstErr == nil {
				firstErr = errf("header CRC mismatch at %d", at)
			}
			continue
		}
		seq := binary.LittleEndian.Uint64(hdr[8:16])
		if !found || seq > bestSeq {
			best = hdr
			bestSeq = seq
			found = true
		}
	}
	if !found {
		if firstErr != nil {
			return nil, errf("no valid header: %v", firstErr)
		}
		return nil, errf("no valid header")
	}
	return best, nil
}

// region is a located region-table entry.
type region struct {
	offset int64
	length int64
}

// readRegionTable parses the primary region table, falling back to the
// backup copy at 256 KiB when the primary is absent or fails its CRC.
func readRegionTable(r io.ReaderAt) (map[guid]region, error) {
	var firstErr error
	for _, at := range []int64{regionTableOffset, regionTableBackupOffset} {
		regions, err := parseRegionTable(r, at)
		if err == nil {
			return regions, nil
		}
		if firstErr == nil {
			firstErr = err
		}
	}
	return nil, errf("no valid region table (primary and backup both rejected: %v)", firstErr)
}

func parseRegionTable(r io.ReaderAt, at int64) (map[guid]region, error) {
	table := make([]byte, regionTableSize)
	if _, err := r.ReadAt(table, at); err != nil {
		return nil, errf("read region table at %d: %w", at, err)
	}
	if string(table[0:4]) != "regi" {
		return nil, errf("bad region table signature at %d", at)
	}

	// The CRC-32C covers the whole 64 KiB table with the checksum field
	// itself zeroed (MS-VHDX 3.2).
	stored := binary.LittleEndian.Uint32(table[4:8])
	zeroed := make([]byte, regionTableSize)
	copy(zeroed, table)
	copy(zeroed[4:8], []byte{0, 0, 0, 0})
	if got := crc32.Checksum(zeroed, castagnoli); got != stored {
		return nil, errf("region table CRC mismatch at %d (got %#08x want %#08x)", at, got, stored)
	}

	count := binary.LittleEndian.Uint32(table[8:12])
	if count > maxRegionEntries {
		return nil, errf("region entry count %d exceeds %d", count, maxRegionEntries)
	}

	regions := make(map[guid]region, count)
	for i := uint32(0); i < count; i++ {
		base := regionTableHeaderSize + int(i)*regionTableEntrySize
		if base+regionTableEntrySize > regionTableSize {
			return nil, errf("region entry %d runs past the table", i)
		}
		// VHDX_REGION_TABLE_ENTRY: Guid[16], FileOffset u64, Length u32,
		// Required:1 + Reserved:31 (u32).
		var g guid
		copy(g[:], table[base:base+16])
		offset := binary.LittleEndian.Uint64(table[base+16 : base+24])
		length := binary.LittleEndian.Uint32(table[base+24 : base+28])
		flags := binary.LittleEndian.Uint32(table[base+28 : base+32])
		if flags&1 == 0 {
			// Not REQUIRED: not a region this parser relies on.
			continue
		}
		if offset > 1<<62 {
			return nil, errf("region offset %d implausible", offset)
		}
		regions[g] = region{offset: int64(offset), length: int64(length)}
	}
	return regions, nil
}

// readGeometry reads the metadata region and extracts the three items the
// BAT walk needs, validating each.
func readGeometry(r io.ReaderAt, meta region) (geo Geometry, err error) {
	items, err := readMetadata(r, meta)
	if err != nil {
		return geo, err
	}

	fileParams, ok := items[fileParametersGUID]
	if !ok || len(fileParams) < 4 {
		return geo, errf("missing or short File Parameters metadata item")
	}
	diskSize, ok := items[virtualDiskSizeGUID]
	if !ok || len(diskSize) < 8 {
		return geo, errf("missing or short Virtual Disk Size metadata item")
	}
	sectorSize, ok := items[logicalSectorSizeGUID]
	if !ok || len(sectorSize) < 4 {
		return geo, errf("missing or short Logical Sector Size metadata item")
	}

	geo.BlockSize = int64(binary.LittleEndian.Uint32(fileParams[0:4]))
	geo.VirtualSize = int64(binary.LittleEndian.Uint64(diskSize[0:8]))
	geo.SectorSize = int64(binary.LittleEndian.Uint32(sectorSize[0:4]))

	// Sanity-check before anything derives a size or an allocation from these.
	if geo.BlockSize < miB || geo.BlockSize > 256*miB || geo.BlockSize&(geo.BlockSize-1) != 0 {
		return geo, errf("implausible block size %d", geo.BlockSize)
	}
	if geo.SectorSize != 512 && geo.SectorSize != 4096 {
		return geo, errf("implausible logical sector size %d", geo.SectorSize)
	}
	if geo.VirtualSize <= 0 || geo.VirtualSize%geo.SectorSize != 0 {
		return geo, errf("implausible virtual disk size %d for sector size %d",
			geo.VirtualSize, geo.SectorSize)
	}

	geo.ChunkRatio = (maxSectorsPerBitmapBlock * geo.SectorSize) / geo.BlockSize
	if geo.ChunkRatio <= 0 {
		return geo, errf("degenerate chunk ratio for block size %d sector size %d",
			geo.BlockSize, geo.SectorSize)
	}
	geo.TotalBlocks = (geo.VirtualSize + geo.BlockSize - 1) / geo.BlockSize
	return geo, nil
}

// readMetadata returns the metadata region's items keyed by item GUID.
func readMetadata(r io.ReaderAt, meta region) (map[guid][]byte, error) {
	// VHDX_METADATA_TABLE_HEADER: Signature[8], Reserved u16,
	// EntryCount u16, Reserved2[20].
	header := make([]byte, 32)
	if _, err := r.ReadAt(header, meta.offset); err != nil {
		return nil, errf("read metadata header: %w", err)
	}
	if string(header[0:8]) != "metadata" {
		return nil, errf("bad metadata region signature")
	}
	count := binary.LittleEndian.Uint16(header[10:12])
	if count > maxMetadataEntries {
		return nil, errf("metadata entry count %d exceeds %d", count, maxMetadataEntries)
	}

	items := make(map[guid][]byte, count)
	for i := uint16(0); i < count; i++ {
		// VHDX_METADATA_TABLE_ENTRY: ItemId[16], Offset u32, Length u32,
		// flag bits u32, Reserved2 u32. Offset is relative to the region.
		entry := make([]byte, 32)
		entryAt := meta.offset + 32 + int64(i)*32
		if _, err := r.ReadAt(entry, entryAt); err != nil {
			return nil, errf("read metadata entry %d: %w", i, err)
		}
		var g guid
		copy(g[:], entry[0:16])
		itemOff := int64(binary.LittleEndian.Uint32(entry[16:20]))
		itemLen := int64(binary.LittleEndian.Uint32(entry[20:24]))
		if itemLen > maxMetadataItemBytes {
			return nil, errf("metadata item %d length %d exceeds %d-byte limit",
				i, itemLen, maxMetadataItemBytes)
		}
		if itemOff < 0 || itemLen < 0 || itemOff+itemLen > meta.length {
			return nil, errf("metadata item %d (offset %d length %d) runs past the %d-byte region",
				i, itemOff, itemLen, meta.length)
		}
		data := make([]byte, itemLen)
		if _, err := r.ReadAt(data, meta.offset+itemOff); err != nil {
			return nil, errf("read metadata item %d: %w", i, err)
		}
		items[g] = data
	}
	return items, nil
}

// blockRef is a present payload block: its index in the virtual disk and
// where its data sits in the file.
type blockRef struct {
	block      int64
	fileOffset int64
}

// bitmapRef is a sector-bitmap BAT entry for one chunk.
type bitmapRef struct {
	state      uint64
	fileOffset int64
}

// walkBAT walks the Block Allocation Table. Differencing BATs use a fixed
// chunk layout: ChunkRatio payload entries (zero-padded in the final
// incomplete chunk) followed by one sector-bitmap entry (MS-VHDX 2.3.2).
func walkBAT(r io.ReaderAt, bat region, geo Geometry) (
	fully, partial, zeros []blockRef, bitmaps map[int64]bitmapRef, err error) {

	chunks := (geo.TotalBlocks + geo.ChunkRatio - 1) / geo.ChunkRatio
	need := chunks * (geo.ChunkRatio + 1) * batEntrySize
	if need > bat.length {
		return nil, nil, nil, nil, errf(
			"BAT needs %d bytes for %d blocks in %d chunks but the region is %d bytes",
			need, geo.TotalBlocks, chunks, bat.length)
	}

	sr := io.NewSectionReader(r, bat.offset, need)
	br := bufio.NewReaderSize(sr, 64*kiB)
	buf := make([]byte, batEntrySize)
	readEntry := func() (uint64, error) {
		if _, err := io.ReadFull(br, buf); err != nil {
			return 0, errf("read BAT entry: %w", err)
		}
		return binary.LittleEndian.Uint64(buf), nil
	}

	bitmaps = make(map[int64]bitmapRef, chunks)
	for chunk := int64(0); chunk < chunks; chunk++ {
		start := chunk * geo.ChunkRatio
		end := start + geo.ChunkRatio
		if end > geo.TotalBlocks {
			end = geo.TotalBlocks
		}
		for idx := start; idx < end; idx++ {
			raw, err := readEntry()
			if err != nil {
				return nil, nil, nil, nil, err
			}
			ref := blockRef{block: idx, fileOffset: int64(raw & batFileOffsetMask)}
			switch raw & batStateMask {
			case blockFullyPresent:
				fully = append(fully, ref)
			case blockPartiallyPresent:
				partial = append(partial, ref)
			case blockZero:
				zeros = append(zeros, ref)
			case blockNotPresent, blockUndefined, blockUnmapped:
				// Resolved through the parent chain, nothing to copy.
			default:
				return nil, nil, nil, nil, errf("block %d has reserved BAT state %d",
					idx, raw&batStateMask)
			}
		}
		// Incomplete final chunk: remaining payload slots are present and zero.
		for idx := end; idx < start+geo.ChunkRatio; idx++ {
			if _, err := readEntry(); err != nil {
				return nil, nil, nil, nil, err
			}
		}
		raw, err := readEntry()
		if err != nil {
			return nil, nil, nil, nil, err
		}
		bitmaps[chunk] = bitmapRef{
			state:      raw & batStateMask,
			fileOffset: int64(raw & batFileOffsetMask),
		}
	}
	return fully, partial, zeros, bitmaps, nil
}

// bitmapRanges expands a partially-present block into the extents its
// sector bitmap marks as present, coalescing adjacent set sectors.
func bitmapRanges(r io.ReaderAt, blk blockRef, bitmaps map[int64]bitmapRef,
	geo Geometry) ([]Range, error) {

	chunk := blk.block / geo.ChunkRatio
	bm, ok := bitmaps[chunk]
	if !ok || bm.state != blockFullyPresent {
		return nil, errf("block %d is partially present but chunk %d has no valid sector bitmap",
			blk.block, chunk)
	}

	sectorsPerBlock := geo.BlockSize / geo.SectorSize
	bytesPerBlock := sectorsPerBlock / 8
	at := bm.fileOffset + (blk.block%geo.ChunkRatio)*bytesPerBlock

	bits := make([]byte, bytesPerBlock)
	if _, err := r.ReadAt(bits, at); err != nil {
		return nil, errf("read sector bitmap for block %d: %w", blk.block, err)
	}

	blockOff := blk.block * geo.BlockSize
	var ranges []Range
	for s := int64(0); s < sectorsPerBlock; {
		if bits[s/8]&(1<<uint(s%8)) == 0 {
			s++
			continue
		}
		start := s
		for s < sectorsPerBlock && bits[s/8]&(1<<uint(s%8)) != 0 {
			s++
		}
		off := blockOff + start*geo.SectorSize
		length := clampLength(off, (s-start)*geo.SectorSize, geo.VirtualSize)
		if length <= 0 {
			break
		}
		ranges = append(ranges, Range{
			VirtualOffset: off,
			FileOffset:    blk.fileOffset + start*geo.SectorSize,
			Length:        length,
		})
	}
	return ranges, nil
}

// sortByVirtualOffset orders ranges ascending for sequential target writes.
func sortByVirtualOffset(ranges []Range) {
	sort.Slice(ranges, func(i, j int) bool {
		return ranges[i].VirtualOffset < ranges[j].VirtualOffset
	})
}

// TotalBytes sums the length of the ranges.
func TotalBytes(ranges []Range) int64 {
	var n int64
	for _, r := range ranges {
		n += r.Length
	}
	return n
}

// DefaultCopyBuffer is the transfer size Copy uses when none is given.
const DefaultCopyBuffer = 2 * miB

// Copy transfers each range from src to dst, reading at the range's
// FileOffset and writing at its VirtualOffset, and returns the bytes
// written. Ranges with FileOffset == zeroFileOffset zero the target.
// Ranges outside the map are never written, so data that lives in the
// parent chain stays as the full copy left it.
func Copy(dst io.WriterAt, src io.ReaderAt, ranges []Range, bufSize int) (int64, error) {
	if bufSize <= 0 {
		bufSize = DefaultCopyBuffer
	}
	buf := make([]byte, bufSize)
	var written int64
	for _, rg := range ranges {
		for done := int64(0); done < rg.Length; {
			n := rg.Length - done
			if n > int64(len(buf)) {
				n = int64(len(buf))
			}
			chunk := buf[:n]
			if rg.FileOffset == zeroFileOffset {
				clear(chunk)
			} else {
				if _, err := src.ReadAt(chunk, rg.FileOffset+done); err != nil {
					return written, errf("read %d bytes at %d: %w", n, rg.FileOffset+done, err)
				}
			}
			if _, err := dst.WriteAt(chunk, rg.VirtualOffset+done); err != nil {
				return written, errf("write %d bytes at %d: %w", n, rg.VirtualOffset+done, err)
			}
			done += n
			written += n
		}
	}
	return written, nil
}
