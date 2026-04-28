package main

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseDDProgress(t *testing.T) {
	tests := []struct {
		name string
		line string
		want int64
	}{
		{"standard output", "1073741824 bytes (1.1 GB, 1.0 GiB) copied, 10.5 s, 102 MB/s", 1073741824},
		{"small copy", "8388608 bytes (8.4 MB, 8.0 MiB) copied, 0.1 s, 84 MB/s", 8388608},
		{"bytes only", "512 bytes copied", 512},
		{"no match", "dd: writing to '/dev/sda': No space left on device", 0},
		{"empty string", "", 0},
		{"clean line after CR split", "1048576 bytes (1.0 MB) copied", 1048576},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseDDProgress(tt.line)
			if got != tt.want {
				t.Errorf("parseDDProgress(%q) = %d, want %d", tt.line, got, tt.want)
			}
		})
	}
}

func TestSplitOnCRorLF(t *testing.T) {
	input := "1048576 bytes copied\r2097152 bytes copied\nfinal line"
	scanner := bufio.NewScanner(strings.NewReader(input))
	scanner.Split(splitOnCRorLF)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	want := []string{
		"1048576 bytes copied",
		"2097152 bytes copied",
		"final line",
	}
	if len(lines) != len(want) {
		t.Fatalf("got %d lines, want %d: %v", len(lines), len(want), lines)
	}
	for i, got := range lines {
		if got != want[i] {
			t.Errorf("line[%d] = %q, want %q", i, got, want[i])
		}
	}
}

func TestPortalRegex(t *testing.T) {
	valid := []string{
		"10.0.0.1:3260",
		"myhost.example.com:3260",
		"192.168.1.100:3261",
		"[2001:db8::1]:3260",
	}
	invalid := []string{
		"10.0.0.1",
		":3260",
		"host:abc",
		"host:",
		"",
		"host:3260; rm -rf /",
	}
	for _, s := range valid {
		if !portalRe.MatchString(s) {
			t.Errorf("portalRe should match %q", s)
		}
	}
	for _, s := range invalid {
		if portalRe.MatchString(s) {
			t.Errorf("portalRe should NOT match %q", s)
		}
	}
}

func TestIQNRegex(t *testing.T) {
	valid := []string{
		"iqn.2026-03.io.forklift:vm-123",
		"iqn.1991-05.com.microsoft:win-target",
		"iqn.2000-01.com.example:storage.disk1",
	}
	invalid := []string{
		"naa.5000c5000c5",
		"iqn.2026.io.forklift:vm",
		"iqn.2026-3.io.forklift:vm",
		"",
		"iqn.2026-03.io.forklift:vm; rm -rf /",
	}
	for _, s := range valid {
		if !iqnRe.MatchString(s) {
			t.Errorf("iqnRe should match %q", s)
		}
	}
	for _, s := range invalid {
		if iqnRe.MatchString(s) {
			t.Errorf("iqnRe should NOT match %q", s)
		}
	}
}

func TestDiskSpecUnmarshal(t *testing.T) {
	input := `[{"lunId":0,"volumePath":"/populatorblock"},{"lunId":1,"volumePath":"/data/disk.img"}]`
	var disks []DiskSpec
	if err := json.Unmarshal([]byte(input), &disks); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(disks) != 2 {
		t.Fatalf("expected 2 disks, got %d", len(disks))
	}
	if disks[0].LunID != 0 || disks[0].VolumePath != "/populatorblock" {
		t.Errorf("disk[0] = %+v", disks[0])
	}
	if disks[1].LunID != 1 || disks[1].VolumePath != "/data/disk.img" {
		t.Errorf("disk[1] = %+v", disks[1])
	}
}

func TestVerifyDiskNotEmpty(t *testing.T) {
	dir := t.TempDir()

	t.Run("all zeros fails", func(t *testing.T) {
		f := filepath.Join(dir, "zeros.raw")
		if err := os.WriteFile(f, make([]byte, 1024), 0600); err != nil {
			t.Fatal(err)
		}
		if err := verifyDiskNotEmpty(f, 0); err == nil {
			t.Error("expected error for all-zero disk")
		}
	})

	t.Run("MBR signature passes", func(t *testing.T) {
		buf := make([]byte, 1024)
		buf[510] = 0x55
		buf[511] = 0xAA
		f := filepath.Join(dir, "mbr.raw")
		if err := os.WriteFile(f, buf, 0600); err != nil {
			t.Fatal(err)
		}
		if err := verifyDiskNotEmpty(f, 0); err != nil {
			t.Errorf("unexpected error for MBR disk: %v", err)
		}
	})

	t.Run("GPT signature passes", func(t *testing.T) {
		buf := make([]byte, 1024)
		copy(buf[512:520], "EFI PART")
		f := filepath.Join(dir, "gpt.raw")
		if err := os.WriteFile(f, buf, 0600); err != nil {
			t.Fatal(err)
		}
		if err := verifyDiskNotEmpty(f, 0); err != nil {
			t.Errorf("unexpected error for GPT disk: %v", err)
		}
	})

	t.Run("non-zero random data passes", func(t *testing.T) {
		buf := make([]byte, 1024)
		buf[0] = 0xFF
		f := filepath.Join(dir, "random.raw")
		if err := os.WriteFile(f, buf, 0600); err != nil {
			t.Fatal(err)
		}
		if err := verifyDiskNotEmpty(f, 0); err != nil {
			t.Errorf("unexpected error for non-zero disk: %v", err)
		}
	})

	t.Run("missing file fails", func(t *testing.T) {
		if err := verifyDiskNotEmpty(filepath.Join(dir, "noexist"), 0); err == nil {
			t.Error("expected error for missing file")
		}
	})
}
