package vhdx

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"strings"
	"testing"
)

// The fixtures below assemble spec-correct VHDX differencing disks so the
// parser is exercised against real structures (region table with CRC-32C,
// metadata region, interleaved BAT) rather than mocks.

const payloadStart = 4 * miB

// sbMarker in an entry's state field flags it as a sector-bitmap entry
// rather than a payload-block entry, both encode identically on disk, the
// distinction only documents intent in the fixture.
type entry struct {
	state  uint64
	offset int64
	isSB   bool
}

type builder struct {
	blockSize  int64
	sectorSize int64
	virtSize   int64
	entries    []entry
	payload    []byte

	// Overrides for malformed-input tests, zero means "derive correctly".
	metaLengthOverride int64
	// rawBAT writes declared entries without ChunkRatio padding (malformed tests).
	rawBAT bool
}

func newBuilder() *builder {
	return &builder{blockSize: 2 * miB, sectorSize: 512, virtSize: 2 * miB}
}

// addPayload appends MiB-aligned payload data and returns its absolute
// file offset. BAT entries can only encode MiB-aligned offsets.
func (b *builder) addPayload(data []byte) int64 {
	if rem := int64(len(b.payload)) % miB; rem != 0 {
		b.payload = append(b.payload, make([]byte, miB-rem)...)
	}
	off := payloadStart + int64(len(b.payload))
	b.payload = append(b.payload, data...)
	return off
}

func (b *builder) addBlock(state uint64, off int64) {
	b.entries = append(b.entries, entry{state: state, offset: off})
}

func (b *builder) addBitmap(state uint64, off int64) {
	b.entries = append(b.entries, entry{state: state, offset: off, isSB: true})
}

// batEntry packs state into bits 0-2 and the MiB-unit file offset into
// bits 20-63 (MS-VHDX 2.3.3.1).
func batEntry(state uint64, offset int64) uint64 {
	if offset%miB != 0 {
		panic("BAT offsets must be MiB-aligned")
	}
	return (state & batStateMask) | (uint64(offset/miB) << 20)
}

func (b *builder) build() []byte {
	const (
		metaRegionOffset = 2 * miB
		batRegionOffset  = 3 * miB
		itemDataBase     = 64 * kiB
	)

	batEntries := b.entries
	if !b.rawBAT {
		// Expand to the MS-VHDX chunk layout (payload slots padded to
		// ChunkRatio, then one sector-bitmap entry per chunk).
		batEntries = b.expandBAT()
	}
	bat := make([]byte, 0, len(batEntries)*batEntrySize)
	for _, e := range batEntries {
		bat = binary.LittleEndian.AppendUint64(bat, batEntry(e.state, e.offset))
	}
	batLength := roundUpMiB(int64(len(bat)))
	if batLength == 0 {
		batLength = miB
	}

	// Metadata region: 32-byte header, entry table, then item data at
	// itemDataBase (offsets are relative to the region start).
	type item struct {
		id   guid
		data []byte
	}
	items := []item{
		{fileParametersGUID, le32(uint32(b.blockSize), 0)},
		{virtualDiskSizeGUID, le64(uint64(b.virtSize))},
		{logicalSectorSizeGUID, le32(uint32(b.sectorSize))},
	}

	meta := make([]byte, 32)
	copy(meta[0:8], "metadata")
	binary.LittleEndian.PutUint16(meta[10:12], uint16(len(items)))

	entries := make([]byte, 0, len(items)*32)
	off := int64(itemDataBase)
	for _, it := range items {
		e := make([]byte, 32)
		copy(e[0:16], it.id[:])
		binary.LittleEndian.PutUint32(e[16:20], uint32(off))
		binary.LittleEndian.PutUint32(e[20:24], uint32(len(it.data)))
		entries = append(entries, e...)
		off += int64(len(it.data))
	}

	metaRegion := make([]byte, itemDataBase)
	copy(metaRegion, meta)
	copy(metaRegion[32:], entries)
	for _, it := range items {
		metaRegion = append(metaRegion, it.data...)
	}
	metaLength := roundUpMiB(int64(len(metaRegion)))
	if b.metaLengthOverride != 0 {
		metaLength = b.metaLengthOverride
	}

	regions := append(
		regionEntry(batRegionGUID, batRegionOffset, uint32(batLength)),
		regionEntry(metadataRegionGUID, metaRegionOffset, uint32(metaLength))...,
	)

	img := make([]byte, payloadStart+len(b.payload))
	copy(img[0:8], "vhdxfile")
	writeHeader(img, headerOffset)
	writeHeader(img, headerBackupOffset)
	writeRegionTable(img, regionTableOffset, regions)
	writeRegionTable(img, regionTableBackupOffset, regions)
	copy(img[metaRegionOffset:], metaRegion)
	copy(img[batRegionOffset:], bat)
	copy(img[payloadStart:], b.payload)
	return img
}

// expandBAT rewrites declared payload/bitmap entries into the fixed
// ChunkRatio+bitmap chunk layout walkBAT expects.
func (b *builder) expandBAT() []entry {
	chunkRatio := (maxSectorsPerBitmapBlock * b.sectorSize) / b.blockSize
	totalBlocks := (b.virtSize + b.blockSize - 1) / b.blockSize
	chunks := (totalBlocks + chunkRatio - 1) / chunkRatio

	var payloads, bitmaps []entry
	for _, e := range b.entries {
		if e.isSB {
			bitmaps = append(bitmaps, e)
		} else {
			payloads = append(payloads, e)
		}
	}
	for int64(len(payloads)) < totalBlocks {
		payloads = append(payloads, entry{state: blockNotPresent})
	}

	out := make([]entry, 0, chunks*(chunkRatio+1))
	bi := 0
	for c := int64(0); c < chunks; c++ {
		start := c * chunkRatio
		end := start + chunkRatio
		if end > totalBlocks {
			end = totalBlocks
		}
		for i := start; i < end; i++ {
			out = append(out, payloads[i])
		}
		for i := end; i < start+chunkRatio; i++ {
			out = append(out, entry{state: blockNotPresent})
		}
		if bi < len(bitmaps) {
			out = append(out, bitmaps[bi])
			bi++
		} else {
			out = append(out, entry{state: 0, isSB: true})
		}
	}
	return out
}

// writeHeader writes a fixture header (empty log, valid CRC, seq=1).
func writeHeader(img []byte, at int64) {
	writeHeaderSeq(img, at, 1)
}

func writeHeaderSeq(img []byte, at int64, seq uint64) {
	hdr := make([]byte, headerSize)
	copy(hdr[0:4], "head")
	binary.LittleEndian.PutUint64(hdr[8:16], seq)
	sealHeader(hdr)
	copy(img[at:], hdr)
}

// sealHeader writes CRC-32C with the checksum field zeroed during the hash.
func sealHeader(hdr []byte) {
	copy(hdr[4:8], []byte{0, 0, 0, 0})
	binary.LittleEndian.PutUint32(hdr[4:8], crc32.Checksum(hdr, castagnoli))
}

func writeRegionTable(img []byte, at int64, regions []byte) {
	table := make([]byte, regionTableSize)
	copy(table[0:4], "regi")
	binary.LittleEndian.PutUint32(table[8:12], uint32(len(regions)/regionTableEntrySize))
	copy(table[regionTableHeaderSize:], regions)
	// CRC-32C over the whole table with the checksum field still zero.
	binary.LittleEndian.PutUint32(table[4:8], crc32.Checksum(table, castagnoli))
	copy(img[at:], table)
}

// regionEntry builds a VHDX_REGION_TABLE_ENTRY: Guid[16], FileOffset u64,
// Length u32, Required:1 (u32).
func regionEntry(g guid, offset int64, length uint32) []byte {
	e := make([]byte, regionTableEntrySize)
	copy(e[0:16], g[:])
	binary.LittleEndian.PutUint64(e[16:24], uint64(offset))
	binary.LittleEndian.PutUint32(e[24:28], length)
	binary.LittleEndian.PutUint32(e[28:32], 1) // REQUIRED
	return e
}

func roundUpMiB(n int64) int64 { return (n + miB - 1) / miB * miB }

func le64(v uint64) []byte { return binary.LittleEndian.AppendUint64(nil, v) }

func le32(v ...uint32) []byte {
	out := make([]byte, 0, 4*len(v))
	for _, x := range v {
		out = binary.LittleEndian.AppendUint32(out, x)
	}
	return out
}

func parse(t *testing.T, img []byte) ([]Range, Geometry) {
	t.Helper()
	ranges, geo, err := ChangedRanges(bytes.NewReader(img))
	if err != nil {
		t.Fatalf("ChangedRanges: %v", err)
	}
	return ranges, geo
}

func mustFail(t *testing.T, img []byte, wantSubstr string) {
	t.Helper()
	_, _, err := ChangedRanges(bytes.NewReader(img))
	if err == nil {
		t.Fatalf("expected an error containing %q, got none", wantSubstr)
	}
	if !strings.Contains(err.Error(), wantSubstr) {
		t.Fatalf("error %q does not contain %q", err, wantSubstr)
	}
}

func wantRanges(t *testing.T, got []Range, want []Range) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d ranges, want %d\ngot:  %+v\nwant: %+v", len(got), len(want), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("range %d: got %+v want %+v", i, got[i], want[i])
		}
	}
}

// ---------------------------------------------------------------- tests

// TestCRC32CKnownAnswer pins the checksum to the published CRC-32C check
// value, so the region-table CRC is verified against the spec rather than
// against whatever the fixture builder happens to compute.
func TestCRC32CKnownAnswer(t *testing.T) {
	const want = 0xE3069283 // CRC-32C("123456789"), Castagnoli check value
	if got := crc32.Checksum([]byte("123456789"), castagnoli); got != want {
		t.Fatalf("CRC-32C check value: got %#08x want %#08x", got, want)
	}
}

func TestFullyPresentBlocks(t *testing.T) {
	b := newBuilder()
	b.virtSize = 6 * miB
	off1 := b.addPayload(bytes.Repeat([]byte{0xAA}, 2*miB))
	off2 := b.addPayload(bytes.Repeat([]byte{0xBB}, 2*miB))
	b.addBlock(blockFullyPresent, off1)
	b.addBlock(blockFullyPresent, off2)
	b.addBlock(blockNotPresent, 0)

	ranges, geo := parse(t, b.build())
	if geo.TotalBlocks != 3 {
		t.Errorf("TotalBlocks = %d, want 3", geo.TotalBlocks)
	}
	wantRanges(t, ranges, []Range{
		{VirtualOffset: 0, FileOffset: off1, Length: 2 * miB},
		{VirtualOffset: 2 * miB, FileOffset: off2, Length: 2 * miB},
	})
	if n := TotalBytes(ranges); n != 4*miB {
		t.Errorf("TotalBytes = %d, want %d", n, 4*miB)
	}
}

func TestPartiallyPresent512(t *testing.T) {
	b := newBuilder()
	// chunkRatio = 2^23 * 512 / 2MiB = 2048, so one block leaves a
	// trailing sector-bitmap entry.
	bitmap := make([]byte, (2*miB/512)/8) // 512 bytes
	bitmap[0] = 0b00000011                // sectors 0, 1
	bitmap[1] = 0b00000100                // sector 10
	bmOff := b.addPayload(bitmap)
	dataOff := b.addPayload(bytes.Repeat([]byte{0xCC}, 2*miB))
	b.addBlock(blockPartiallyPresent, dataOff)
	b.addBitmap(blockFullyPresent, bmOff)

	ranges, geo := parse(t, b.build())
	if geo.ChunkRatio != 2048 {
		t.Errorf("ChunkRatio = %d, want 2048", geo.ChunkRatio)
	}
	wantRanges(t, ranges, []Range{
		// sectors 0-1 coalesce into one 1 KiB range
		{VirtualOffset: 0, FileOffset: dataOff, Length: 1024},
		// sector 10 stands alone
		{VirtualOffset: 5120, FileOffset: dataOff + 5120, Length: 512},
	})
}

func TestSectorSize4096(t *testing.T) {
	b := newBuilder()
	b.sectorSize = 4096
	bitmap := make([]byte, (2*miB/4096)/8) // 64 bytes
	bitmap[0] = 0b00000011                 // sectors 0 and 1, 4096 B each
	bmOff := b.addPayload(bitmap)
	dataOff := b.addPayload(bytes.Repeat([]byte{0xCC}, 8*kiB))
	b.addBlock(blockPartiallyPresent, dataOff)
	b.addBitmap(blockFullyPresent, bmOff)

	ranges, geo := parse(t, b.build())
	if geo.SectorSize != 4096 {
		t.Errorf("SectorSize = %d, want 4096", geo.SectorSize)
	}
	if geo.ChunkRatio != 16384 {
		t.Errorf("ChunkRatio = %d, want 16384", geo.ChunkRatio)
	}
	wantRanges(t, ranges, []Range{
		{VirtualOffset: 0, FileOffset: dataOff, Length: 8 * kiB},
	})
}

func TestNotPresentSkipped(t *testing.T) {
	b := newBuilder()
	b.addBlock(blockNotPresent, 0)
	ranges, _ := parse(t, b.build())
	if len(ranges) != 0 {
		t.Fatalf("expected no ranges, got %+v", ranges)
	}
}

// TestParentResolvedStatesSkipped covers states that must resolve through
// the parent chain (leave the target untouched).
func TestParentResolvedStatesSkipped(t *testing.T) {
	for name, state := range map[string]uint64{
		"undefined": blockUndefined,
		"unmapped":  blockUnmapped,
	} {
		t.Run(name, func(t *testing.T) {
			b := newBuilder()
			b.addBlock(state, 0)
			ranges, _ := parse(t, b.build())
			if len(ranges) != 0 {
				t.Fatalf("state %d produced ranges %+v, want none", state, ranges)
			}
		})
	}
}

// TestZeroBlockClearsTarget reports PAYLOAD_BLOCK_ZERO as an explicit
// zero-fill range so a prior full copy's stale data is overwritten.
func TestZeroBlockClearsTarget(t *testing.T) {
	b := newBuilder()
	b.addBlock(blockZero, 0)
	ranges, _ := parse(t, b.build())
	wantRanges(t, ranges, []Range{
		{VirtualOffset: 0, FileOffset: zeroFileOffset, Length: 2 * miB},
	})

	dst := &writerAt{buf: bytes.Repeat([]byte{0xFF}, 2*miB)}
	written, err := Copy(dst, bytes.NewReader(b.build()), ranges, 0)
	if err != nil {
		t.Fatalf("Copy: %v", err)
	}
	if written != 2*miB {
		t.Fatalf("written = %d, want %d", written, 2*miB)
	}
	if !bytes.Equal(dst.buf, make([]byte, 2*miB)) {
		t.Fatal("ZERO block did not clear the target")
	}
}

func TestVirtualSizeClampsFinalBlock(t *testing.T) {
	b := newBuilder()
	b.virtSize = 3 * miB // not block-aligned
	off1 := b.addPayload(bytes.Repeat([]byte{0xAA}, 2*miB))
	off2 := b.addPayload(bytes.Repeat([]byte{0xBB}, 2*miB))
	b.addBlock(blockFullyPresent, off1)
	b.addBlock(blockFullyPresent, off2)

	ranges, geo := parse(t, b.build())
	if geo.VirtualSize != 3*miB {
		t.Fatalf("VirtualSize = %d, want %d", geo.VirtualSize, 3*miB)
	}
	wantRanges(t, ranges, []Range{
		{VirtualOffset: 0, FileOffset: off1, Length: 2 * miB},
		{VirtualOffset: 2 * miB, FileOffset: off2, Length: 1 * miB},
	})
}

func TestPartialBlockMissingBitmapRejected(t *testing.T) {
	b := newBuilder()
	dataOff := b.addPayload(bytes.Repeat([]byte{0xCC}, 2*miB))
	b.addBlock(blockPartiallyPresent, dataOff)
	// No bitmap entry — expandBAT inserts a zero-state bitmap.
	mustFail(t, b.build(), "no valid sector bitmap")
}

func TestActiveLogRejected(t *testing.T) {
	b := newBuilder()
	b.addBlock(blockNotPresent, 0)
	img := b.build()
	// Non-zero LogGuid on the current header marks an active log.
	active := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	copy(img[headerOffset+48:headerOffset+64], active)
	sealHeader(img[headerOffset : headerOffset+headerSize])
	copy(img[headerBackupOffset+48:headerBackupOffset+64], active)
	sealHeader(img[headerBackupOffset : headerBackupOffset+headerSize])
	mustFail(t, img, "active VHDX log")
}

func TestAllocatedInactiveLogAccepted(t *testing.T) {
	// LogLength=1MiB with LogGuid=0 is normal (allocated, inactive).
	b := newBuilder()
	off := b.addPayload(bytes.Repeat([]byte{0xAB}, 2*miB))
	b.addBlock(blockFullyPresent, off)
	img := b.build()
	binary.LittleEndian.PutUint32(img[headerOffset+68:headerOffset+72], uint32(miB))
	sealHeader(img[headerOffset : headerOffset+headerSize])
	binary.LittleEndian.PutUint32(img[headerBackupOffset+68:headerBackupOffset+72], uint32(miB))
	sealHeader(img[headerBackupOffset : headerBackupOffset+headerSize])
	ranges, _ := parse(t, img)
	wantRanges(t, ranges, []Range{
		{VirtualOffset: 0, FileOffset: off, Length: 2 * miB},
	})
}

func TestCurrentHeaderIgnoresStaleActiveLog(t *testing.T) {
	// Older primary has an active log; newer backup is clean — accept.
	b := newBuilder()
	off := b.addPayload(bytes.Repeat([]byte{0xCD}, 2*miB))
	b.addBlock(blockFullyPresent, off)
	img := b.build()
	writeHeaderSeq(img, headerOffset, 10)
	copy(img[headerOffset+48:headerOffset+64], []byte{
		1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16,
	})
	sealHeader(img[headerOffset : headerOffset+headerSize])
	writeHeaderSeq(img, headerBackupOffset, 11)
	ranges, _ := parse(t, img)
	wantRanges(t, ranges, []Range{
		{VirtualOffset: 0, FileOffset: off, Length: 2 * miB},
	})
}

func TestCurrentHeaderRejectsActiveLogOnNewerHeader(t *testing.T) {
	b := newBuilder()
	b.addBlock(blockNotPresent, 0)
	img := b.build()
	writeHeaderSeq(img, headerOffset, 10) // clean, older
	writeHeaderSeq(img, headerBackupOffset, 11)
	copy(img[headerBackupOffset+48:headerBackupOffset+64], []byte{
		1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16,
	})
	sealHeader(img[headerBackupOffset : headerBackupOffset+headerSize])
	mustFail(t, img, "active VHDX log")
}

func TestOversizedMetadataItemRejected(t *testing.T) {
	// Craft a metadata entry whose Length exceeds 1 MiB but still fits in
	// a large declared region so only the item-size check fires.
	b := newBuilder()
	b.addBlock(blockNotPresent, 0)
	b.metaLengthOverride = 4 * miB
	img := b.build()
	// First metadata entry Length is at region+32+20.
	const metaRegionOffset = 2 * miB
	entryLenAt := metaRegionOffset + 32 + 20
	binary.LittleEndian.PutUint32(img[entryLenAt:entryLenAt+4], uint32(2*miB))
	mustFail(t, img, "exceeds")
}

func TestCorruptRegionTableRejected(t *testing.T) {
	b := newBuilder()
	off := b.addPayload(bytes.Repeat([]byte{0xAA}, 2*miB))
	b.addBlock(blockFullyPresent, off)
	img := b.build()
	// Break both copies, a single corrupt table must fall through to the
	// backup, which TestBackupRegionTableUsed covers.
	img[regionTableOffset+4] ^= 0xFF
	img[regionTableBackupOffset+4] ^= 0xFF
	mustFail(t, img, "no valid region table")
}

// TestBackupRegionTableUsed proves the 256 KiB fallback actually works.
// The Python parser had the same fallback but no test reached it.
func TestBackupRegionTableUsed(t *testing.T) {
	b := newBuilder()
	off := b.addPayload(bytes.Repeat([]byte{0xAA}, 2*miB))
	b.addBlock(blockFullyPresent, off)
	img := b.build()
	img[regionTableOffset+4] ^= 0xFF // corrupt primary only

	ranges, _ := parse(t, img)
	wantRanges(t, ranges, []Range{
		{VirtualOffset: 0, FileOffset: off, Length: 2 * miB},
	})
}

// TestMetadataItemOutsideRegionRejected guards the bounds check that the
// Python parser could not enforce: it decoded the region-table Length as a
// u64, absorbing the adjacent Required/Reserved dword, so every declared
// region length came back ~4 GiB too large and this check never fired.
func TestMetadataItemOutsideRegionRejected(t *testing.T) {
	b := newBuilder()
	b.addBlock(blockNotPresent, 0)
	// Declare a metadata region far smaller than the items it holds, the
	// first item sits at 64 KiB, past this length.
	b.metaLengthOverride = 4 * kiB
	mustFail(t, b.build(), "runs past")
}

func TestReservedBATStateRejected(t *testing.T) {
	b := newBuilder()
	b.addBlock(4, 0) // 4 and 5 are reserved in MS-VHDX 2.3.3.1
	mustFail(t, b.build(), "reserved BAT state")
}

func TestImplausibleGeometryRejected(t *testing.T) {
	tests := map[string]struct {
		mutate func(*builder)
		want   string
	}{
		"block size not a power of two": {
			func(b *builder) { b.blockSize = 3 * miB }, "implausible block size",
		},
		"block size too small": {
			func(b *builder) { b.blockSize = 512 }, "implausible block size",
		},
		"sector size unsupported": {
			func(b *builder) { b.sectorSize = 1024 }, "implausible logical sector size",
		},
		"virtual size not sector aligned": {
			func(b *builder) { b.virtSize = 2*miB + 1 }, "implausible virtual disk size",
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			b := newBuilder()
			b.addBlock(blockNotPresent, 0)
			tc.mutate(b)
			mustFail(t, b.build(), tc.want)
		})
	}
}

// TestTruncatedBATRejected makes sure a BAT region too short for the
// declared virtual size is caught before the walk reads past it.
func TestTruncatedBATRejected(t *testing.T) {
	b := newBuilder()
	// 1 TiB at 2 MiB blocks needs 524288 payload entries plus 256 bitmap
	// entries: 4196352 bytes of BAT, against the 1 MiB region the builder
	// emits. Without the up-front size check the walk would read ~4 MiB
	// past the region before noticing.
	b.virtSize = 1024 * 1024 * miB
	b.rawBAT = true
	b.addBlock(blockNotPresent, 0)
	mustFail(t, b.build(), "BAT needs")
}

// writerAt adapts a byte slice to io.WriterAt so Copy can be tested
// without touching a block device.
type writerAt struct{ buf []byte }

func (w *writerAt) WriteAt(p []byte, off int64) (int, error) {
	copy(w.buf[off:], p)
	return len(p), nil
}

// TestCopyWritesOnlyMappedRanges is the end-to-end assertion the Python
// suite made against a scratch file: present extents land at the right
// target offsets and everything else keeps its prior contents.
func TestCopyWritesOnlyMappedRanges(t *testing.T) {
	b := newBuilder()
	bitmap := make([]byte, (2*miB/512)/8)
	bitmap[0] = 0b00000011 // sectors 0, 1
	bitmap[1] = 0b00000100 // sector 10
	bmOff := b.addPayload(bitmap)
	dataOff := b.addPayload(bytes.Repeat([]byte{0xCC}, 2*miB))
	b.addBlock(blockPartiallyPresent, dataOff)
	b.addBitmap(blockFullyPresent, bmOff)
	img := b.build()

	ranges, _ := parse(t, img)
	// Target pre-filled with 0xFF, standing in for parent-chain data the
	// full copy already wrote.
	dst := &writerAt{buf: bytes.Repeat([]byte{0xFF}, 2*miB)}
	written, err := Copy(dst, bytes.NewReader(img), ranges, 0)
	if err != nil {
		t.Fatalf("Copy: %v", err)
	}
	if want := int64(1024 + 512); written != want {
		t.Fatalf("written = %d, want %d", written, want)
	}

	assertSpan := func(from, to int, want byte, label string) {
		t.Helper()
		for i := from; i < to; i++ {
			if dst.buf[i] != want {
				t.Fatalf("%s: byte %d = %#x, want %#x", label, i, dst.buf[i], want)
			}
		}
	}
	assertSpan(0, 1024, 0xCC, "sectors 0-1 copied")
	assertSpan(1024, 5120, 0xFF, "gap left untouched")
	assertSpan(5120, 5632, 0xCC, "sector 10 copied")
	assertSpan(5632, 2*miB, 0xFF, "tail left untouched")
}

// TestCopyHonoursBufferSize exercises the chunking loop by forcing many
// small transfers across a range larger than the buffer.
func TestCopyHonoursBufferSize(t *testing.T) {
	b := newBuilder()
	off := b.addPayload(bytes.Repeat([]byte{0x5A}, 2*miB))
	b.addBlock(blockFullyPresent, off)
	img := b.build()

	ranges, _ := parse(t, img)
	dst := &writerAt{buf: make([]byte, 2*miB)}
	written, err := Copy(dst, bytes.NewReader(img), ranges, 4096)
	if err != nil {
		t.Fatalf("Copy: %v", err)
	}
	if written != 2*miB {
		t.Fatalf("written = %d, want %d", written, 2*miB)
	}
	if !bytes.Equal(dst.buf, bytes.Repeat([]byte{0x5A}, 2*miB)) {
		t.Fatal("chunked copy produced wrong contents")
	}
}
