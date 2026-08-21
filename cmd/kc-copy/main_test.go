package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	kccopy "github.com/yaacov/kc-utils/pkg/copy"
)

func TestLoadInput(t *testing.T) {
	t.Parallel()

	validJSON := `{
		"host": "vcenter.example.com",
		"vm_name": "my-vm",
		"fingerprint": "aa:bb:cc"
	}`
	overrideJSON := `{
		"host": "vcenter.example.com",
		"datacenter": "DC1",
		"insecure": true,
		"ca_cert": "/etc/ca.pem",
		"vm_name": "my-vm",
		"fingerprint": "aa:bb:cc",
		"source_disks": ["[ds] vm/a.vmdk"],
		"target_dir": "/json/target",
		"workdir": "/json/work",
		"output_path": "/json/out.json",
		"copy_concurrency": 8
	}`

	tests := []struct {
		name            string
		inputFile       func(t *testing.T) string
		host            string
		datacenter      string
		insecure        bool
		caCert          string
		vmName          string
		fingerprint     string
		diskPath        string
		targetDir       string
		workdir         string
		outputPath      string
		copyConcurrency int
		want            kccopy.CopyInput
		wantErr         string
	}{
		{
			name: "JSON applies CLI defaults",
			inputFile: func(t *testing.T) string {
				t.Helper()
				path := filepath.Join(t.TempDir(), "input.json")
				if err := os.WriteFile(path, []byte(validJSON), 0o644); err != nil {
					t.Fatal(err)
				}
				return path
			},
			targetDir:       "/cli/target",
			workdir:         kccopy.DefaultWorkdir,
			outputPath:      "/cli/out.json",
			copyConcurrency: 4,
			want: kccopy.CopyInput{
				Host:            "vcenter.example.com",
				VMName:          "my-vm",
				Fingerprint:     "aa:bb:cc",
				TargetDir:       "/cli/target",
				Workdir:         kccopy.DefaultWorkdir,
				OutputPath:      "/cli/out.json",
				CopyConcurrency: 4,
			},
		},
		{
			name: "JSON keeps overrides",
			inputFile: func(t *testing.T) string {
				t.Helper()
				path := filepath.Join(t.TempDir(), "input.json")
				if err := os.WriteFile(path, []byte(overrideJSON), 0o644); err != nil {
					t.Fatal(err)
				}
				return path
			},
			targetDir:       "/cli/target",
			workdir:         kccopy.DefaultWorkdir,
			outputPath:      "/cli/out.json",
			copyConcurrency: 4,
			want: kccopy.CopyInput{
				Host:            "vcenter.example.com",
				Datacenter:      "DC1",
				Insecure:        true,
				CaCert:          "/etc/ca.pem",
				VMName:          "my-vm",
				Fingerprint:     "aa:bb:cc",
				SourceDisks:     []string{"[ds] vm/a.vmdk"},
				TargetDir:       "/json/target",
				Workdir:         "/json/work",
				OutputPath:      "/json/out.json",
				CopyConcurrency: 8,
			},
		},
		{
			name: "JSON uses CLI workdir when empty",
			inputFile: func(t *testing.T) string {
				t.Helper()
				path := filepath.Join(t.TempDir(), "input.json")
				if err := os.WriteFile(path, []byte(validJSON), 0o644); err != nil {
					t.Fatal(err)
				}
				return path
			},
			workdir:         "/cli/work",
			outputPath:      "copy-progress.json",
			copyConcurrency: kccopy.DefaultCopyConcurrency,
			want: kccopy.CopyInput{
				Host:            "vcenter.example.com",
				VMName:          "my-vm",
				Fingerprint:     "aa:bb:cc",
				Workdir:         "/cli/work",
				OutputPath:      "copy-progress.json",
				CopyConcurrency: kccopy.DefaultCopyConcurrency,
			},
		},
		{
			name:            "flag-derived input",
			host:            "vcenter.example.com",
			datacenter:      "DC1",
			insecure:        true,
			caCert:          "/etc/ca.pem",
			vmName:          "my-vm",
			fingerprint:     "aa:bb:cc",
			diskPath:        " [ds] vm/a.vmdk , [ds] vm/b.vmdk ",
			targetDir:       "/cli/target",
			workdir:         kccopy.DefaultWorkdir,
			outputPath:      "/cli/out.json",
			copyConcurrency: 2,
			want: kccopy.CopyInput{
				Host:            "vcenter.example.com",
				Datacenter:      "DC1",
				Insecure:        true,
				CaCert:          "/etc/ca.pem",
				VMName:          "my-vm",
				Fingerprint:     "aa:bb:cc",
				SourceDisks:     kccopy.SplitDiskPath(" [ds] vm/a.vmdk , [ds] vm/b.vmdk "),
				TargetDir:       "/cli/target",
				Workdir:         kccopy.DefaultWorkdir,
				OutputPath:      "/cli/out.json",
				CopyConcurrency: 2,
			},
		},
		{
			name: "JSON read failure",
			inputFile: func(t *testing.T) string {
				t.Helper()
				return filepath.Join(t.TempDir(), "missing.json")
			},
			wantErr: "read input:",
		},
		{
			name: "JSON parse failure",
			inputFile: func(t *testing.T) string {
				t.Helper()
				path := filepath.Join(t.TempDir(), "bad.json")
				if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
					t.Fatal(err)
				}
				return path
			},
			wantErr: "parse input JSON:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			inputFile := ""
			if tt.inputFile != nil {
				inputFile = tt.inputFile(t)
			}
			got, err := loadInput(inputFile, tt.host, tt.datacenter, tt.insecure, tt.caCert, tt.vmName, tt.fingerprint, tt.diskPath, tt.targetDir, tt.workdir, tt.outputPath, tt.copyConcurrency)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatal("expected error")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error %q, want substring %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.Host != tt.want.Host ||
				got.Datacenter != tt.want.Datacenter ||
				got.Insecure != tt.want.Insecure ||
				got.CaCert != tt.want.CaCert ||
				got.VMName != tt.want.VMName ||
				got.Fingerprint != tt.want.Fingerprint ||
				got.TargetDir != tt.want.TargetDir ||
				got.Workdir != tt.want.Workdir ||
				got.OutputPath != tt.want.OutputPath ||
				got.CopyConcurrency != tt.want.CopyConcurrency {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
			if len(got.SourceDisks) != len(tt.want.SourceDisks) {
				t.Fatalf("SourceDisks = %v, want %v", got.SourceDisks, tt.want.SourceDisks)
			}
			for i, path := range tt.want.SourceDisks {
				if got.SourceDisks[i] != path {
					t.Fatalf("SourceDisks = %v, want %v", got.SourceDisks, tt.want.SourceDisks)
				}
			}
		})
	}
}

func TestValidateInput(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   kccopy.CopyInput
		wantErr string
	}{
		{
			name: "valid",
			input: kccopy.CopyInput{
				Host:        "vcenter.example.com",
				VMName:      "my-vm",
				Fingerprint: "aa:bb:cc",
			},
		},
		{
			name: "missing host",
			input: kccopy.CopyInput{
				VMName:      "my-vm",
				Fingerprint: "aa:bb:cc",
			},
			wantErr: "--host is required",
		},
		{
			name: "missing vm-name",
			input: kccopy.CopyInput{
				Host:        "vcenter.example.com",
				Fingerprint: "aa:bb:cc",
			},
			wantErr: "--vm-name is required",
		},
		{
			name: "missing fingerprint",
			input: kccopy.CopyInput{
				Host:   "vcenter.example.com",
				VMName: "my-vm",
			},
			wantErr: "--fingerprint is required",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateInput(&tt.input)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error %q, want substring %q", err, tt.wantErr)
			}
		})
	}
}
