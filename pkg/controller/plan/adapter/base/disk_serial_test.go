package base

import (
	"testing"
)

func TestDiskSerial_UsesSourceID(t *testing.T) {
	serial := DiskSerial("6000C29a-1234-5678-abcd-000000000001", "vm-42", 0)
	if len(serial) > MaxDiskSerialLength {
		t.Errorf("serial too long: %d > %d", len(serial), MaxDiskSerialLength)
	}
	// Hyphens preserved, then truncated to 20 — matches what QEMU would
	// produce from the raw UUID, keeping /dev/disk/by-id/ stable.
	if serial != "6000C29a-1234-5678-a" {
		t.Errorf("unexpected serial: %q", serial)
	}
}

func TestDiskSerial_SourceIDShort(t *testing.T) {
	serial := DiskSerial("disk-123", "vm-42", 0)
	if serial != "disk-123" {
		t.Errorf("expected %q, got %q", "disk-123", serial)
	}
}

func TestDiskSerial_FallbackWhenEmpty(t *testing.T) {
	serial := DiskSerial("", "vm-42", 0)
	if len(serial) != MaxDiskSerialLength {
		t.Errorf("fallback serial length: %d, want %d", len(serial), MaxDiskSerialLength)
	}
	// Deterministic: same inputs → same output
	serial2 := DiskSerial("", "vm-42", 0)
	if serial != serial2 {
		t.Errorf("fallback not deterministic: %q vs %q", serial, serial2)
	}
}

func TestDiskSerial_FallbackWhenSourceIDSanitizesToEmpty(t *testing.T) {
	got := DiskSerial("@@@", "vm-42", 0)
	want := DiskSerial("", "vm-42", 0)
	if got != want {
		t.Errorf("sanitized-empty fallback = %q, want %q", got, want)
	}
}

func TestDiskSerial_FallbackDiffersByDiskKey(t *testing.T) {
	s0 := DiskSerial("", "vm-42", 0)
	s1 := DiskSerial("", "vm-42", 1)
	if s0 == s1 {
		t.Error("fallback should differ by disk key")
	}
}

func TestDiskSerial_FallbackDiffersByVM(t *testing.T) {
	s1 := DiskSerial("", "vm-1", 0)
	s2 := DiskSerial("", "vm-2", 0)
	if s1 == s2 {
		t.Error("fallback should differ by VM ID")
	}
}

func TestSanitizeSerial(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"uuid preserves hyphens", "6000C29a-1234-5678-abcd-000000000001", "6000C29a-1234-5678-abcd-000000000001"},
		{"plain alphanumeric", "abc123", "abc123"},
		{"special chars stripped", "disk@#$%^&*()", "disk"},
		{"empty", "", ""},
		{"spaces stripped", "disk 01", "disk01"},
		{"braces stripped", "{12345678-abcd}", "12345678-abcd"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeSerial(tt.input)
			if got != tt.expected {
				t.Errorf("sanitizeSerial(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	if truncate("abcdef", 3) != "abc" {
		t.Error("truncate failed")
	}
	if truncate("abc", 10) != "abc" {
		t.Error("truncate should not pad")
	}
}
