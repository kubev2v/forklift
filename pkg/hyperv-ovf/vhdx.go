package hypervovf

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
)

const (
	regionTableOffset1     = 0x30000
	regionTableOffset2     = 0x40000
	regionTableSignature   = 0x69676572
	metadataTableSignature = uint64(0x617461646174656d)
)

var metadataRegionGUID = []byte{
	0x06, 0xA2, 0x7C, 0x8B, 0x90, 0x47, 0x9A, 0x4B,
	0xB8, 0xFE, 0x57, 0x5F, 0x05, 0x0F, 0x88, 0x6E,
}

var virtualDiskSizeGUID = []byte{
	0x24, 0x42, 0xA5, 0x2F, 0x1B, 0xCD, 0x76, 0x48,
	0xB2, 0x11, 0x5D, 0xBE, 0xD8, 0x3B, 0xF4, 0xB8,
}

type regionTableHeader struct {
	Signature  uint32
	Checksum   uint32
	EntryCount uint32
	Reserved   uint32
}

type regionTableEntry struct {
	GUID       [16]byte
	FileOffset uint64
	Length     uint32
	Required   uint32
}

type metadataTableHeader struct {
	Signature  uint64
	Reserved   uint16
	EntryCount uint16
	_          [20]byte
}

type metadataTableEntry struct {
	ItemID   [16]byte
	Offset   uint32
	Length   uint32
	Flags    uint32
	Reserved uint32
}

// GetVHDXVirtualSize reads the virtual disk size from a VHDX file
func GetVHDXVirtualSize(path string) (uint64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	// Verify signature
	sig := make([]byte, 8)
	if _, err := f.Read(sig); err != nil {
		return 0, fmt.Errorf("read signature: %w", err)
	}
	if string(sig) != "vhdxfile" {
		return 0, errors.New("not a valid VHDX file: invalid signature")
	}

	metaOff, metaLen, err := findMetadataRegion(f)
	if err != nil {
		return 0, fmt.Errorf("locate metadata region: %w", err)
	}

	size, err := readVirtualDiskSizeFromMetadata(f, metaOff, metaLen)
	if err != nil {
		return 0, fmt.Errorf("read virtual disk size: %w", err)
	}

	return size, nil
}

func findMetadataRegion(r io.ReadSeeker) (uint64, uint32, error) {
	offsets := []int64{regionTableOffset1, regionTableOffset2}
	for _, off := range offsets {
		if _, err := r.Seek(off, io.SeekStart); err != nil {
			continue
		}

		var header regionTableHeader
		if err := binary.Read(r, binary.LittleEndian, &header); err != nil {
			continue
		}

		if header.Signature != regionTableSignature {
			continue
		}

		if header.EntryCount == 0 || header.EntryCount > 1024 {
			continue
		}

		for i := uint32(0); i < header.EntryCount; i++ {
			var entry regionTableEntry
			if err := binary.Read(r, binary.LittleEndian, &entry); err != nil {
				break
			}

			if bytes.Equal(entry.GUID[:], metadataRegionGUID) {
				if entry.Length == 0 {
					return 0, 0, errors.New("metadata region has zero length")
				}
				return entry.FileOffset, entry.Length, nil
			}
		}
	}

	return 0, 0, errors.New("metadata region not found in known region-table locations")
}

func readVirtualDiskSizeFromMetadata(r io.ReadSeeker, metaOffset uint64, metaLen uint32) (uint64, error) {
	if _, err := r.Seek(int64(metaOffset), io.SeekStart); err != nil {
		return 0, fmt.Errorf("seek to metadata region: %w", err)
	}

	var header metadataTableHeader
	if err := binary.Read(r, binary.LittleEndian, &header); err != nil {
		return 0, fmt.Errorf("read metadata header: %w", err)
	}

	if header.Signature != metadataTableSignature {
		return 0, fmt.Errorf("invalid metadata signature: 0x%x", header.Signature)
	}

	if header.EntryCount == 0 || header.EntryCount > 2048 {
		return 0, fmt.Errorf("metadata entry count out of range: %d", header.EntryCount)
	}

	entries := make([]metadataTableEntry, header.EntryCount)
	for i := 0; i < int(header.EntryCount); i++ {
		if err := binary.Read(r, binary.LittleEndian, &entries[i]); err != nil {
			return 0, fmt.Errorf("read metadata entry %d: %w", i, err)
		}
	}

	for i, e := range entries {
		if bytes.Equal(e.ItemID[:], virtualDiskSizeGUID) {
			dataStart := uint64(metaOffset) + uint64(e.Offset)
			if e.Length < 8 {
				return 0, fmt.Errorf("virtual disk size entry length too small: %d", e.Length)
			}
			if dataStart+uint64(e.Length) > uint64(metaOffset)+uint64(metaLen) {
				return 0, fmt.Errorf("virtual disk size entry out of metadata bounds (entry %d)", i)
			}
			if _, err := r.Seek(int64(dataStart), io.SeekStart); err != nil {
				return 0, fmt.Errorf("seek to virtual size entry: %w", err)
			}
			var vs uint64
			if err := binary.Read(r, binary.LittleEndian, &vs); err != nil {
				return 0, fmt.Errorf("read virtual size value: %w", err)
			}
			return vs, nil
		}
	}

	return 0, errors.New("virtual disk size metadata not found")
}
