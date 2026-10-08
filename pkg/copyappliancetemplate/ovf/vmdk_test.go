package ovf

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestDiskCapacity(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "disk-0.vmdk")

	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	const sectors = 20 * 1024 * 1024 // 10 GiB
	header := make([]byte, 512)
	binary.LittleEndian.PutUint32(header[0:4], 0x564d444b) // VMDK magic
	binary.LittleEndian.PutUint32(header[8:12], 1<<16)     // stream-optimized (compressed)
	binary.LittleEndian.PutUint64(header[12:20], sectors)
	if _, err := f.Write(header); err != nil {
		t.Fatal(err)
	}

	got, err := DiskCapacity(path)
	if err != nil {
		t.Fatal(err)
	}
	want := int64(sectors) * 512
	if got != want {
		t.Fatalf("capacity = %d, want %d", got, want)
	}
}
