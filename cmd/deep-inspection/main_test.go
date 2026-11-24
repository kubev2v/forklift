package main

import (
	"os"
	"path/filepath"
	"testing"
)

// withSecretDir points secretDir at dir for the duration of the test and
// restores the original value afterward, since secretDir is a package-level
// variable shared across loadProviderCredentials calls.
func withSecretDir(t *testing.T, dir string) {
	t.Helper()
	original := secretDir
	secretDir = dir
	t.Cleanup(func() { secretDir = original })
}

func writeSecretFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write secret file %s: %v", name, err)
	}
}

func TestLoadProviderCredentials_ReadsCACertPathWhenPresent(t *testing.T) {
	dir := t.TempDir()
	withSecretDir(t, dir)
	writeSecretFile(t, dir, "url", "https://vcenter.example.com")
	writeSecretFile(t, dir, "user", "admin")
	writeSecretFile(t, dir, "password", "secret")
	writeSecretFile(t, dir, "ca.crt", "-----BEGIN CERTIFICATE-----\nfake\n-----END CERTIFICATE-----")

	creds, err := loadProviderCredentials()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := filepath.Join(dir, "ca.crt")
	if creds.TLSCACert != want {
		t.Fatalf("expected TLSCACert %q, got %q", want, creds.TLSCACert)
	}
}

func TestLoadProviderCredentials_NoCACertFileMeansEmptyTLSCACert(t *testing.T) {
	dir := t.TempDir()
	withSecretDir(t, dir)
	writeSecretFile(t, dir, "url", "https://vcenter.example.com")
	writeSecretFile(t, dir, "user", "admin")
	writeSecretFile(t, dir, "password", "secret")

	creds, err := loadProviderCredentials()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if creds.TLSCACert != "" {
		t.Fatalf("expected empty TLSCACert when no ca.crt file exists, got %q", creds.TLSCACert)
	}
}

func TestLoadProviderCredentials_StillReadsFingerprintAlongsideCACert(t *testing.T) {
	dir := t.TempDir()
	withSecretDir(t, dir)
	writeSecretFile(t, dir, "url", "https://vcenter.example.com")
	writeSecretFile(t, dir, "user", "admin")
	writeSecretFile(t, dir, "password", "secret")
	writeSecretFile(t, dir, "fingerprint", "AA:BB:CC")
	writeSecretFile(t, dir, "ca.crt", "-----BEGIN CERTIFICATE-----\nfake\n-----END CERTIFICATE-----")

	creds, err := loadProviderCredentials()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if creds.TLSThumbprint != "AA:BB:CC" {
		t.Fatalf("expected TLSThumbprint AA:BB:CC, got %q", creds.TLSThumbprint)
	}
	if creds.TLSCACert == "" {
		t.Fatal("expected TLSCACert to still be set alongside fingerprint")
	}
}
