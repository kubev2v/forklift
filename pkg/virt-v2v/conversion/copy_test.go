package conversion

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kubev2v/forklift/pkg/virt-v2v/config"
	"github.com/kubev2v/forklift/pkg/virt-v2v/utils"
	kccopy "github.com/yaacov/kc-utils/pkg/copy"
	"go.uber.org/mock/gomock"
)

func TestParseLibvirtURL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		url        string
		host       string
		datacenter string
		insecure   bool
		wantErr    bool
	}{
		{
			name:       "vpx datacenter path",
			url:        "vpx://user@vcenter.example.com/Datacenter/Cluster/host.example.com",
			host:       "vcenter.example.com",
			datacenter: "Datacenter",
			insecure:   false,
		},
		{
			name:       "no_verify=1",
			url:        "vpx://user@vcenter/dc/host/esxi?no_verify=1",
			host:       "vcenter",
			datacenter: "dc",
			insecure:   true,
		},
		{
			name:       "cacert boilerplate is not insecure",
			url:        "vpx://user@vcenter/dc/host/esxi?cacert=/opt/ca-bundle.crt",
			host:       "vcenter",
			datacenter: "dc",
			insecure:   false,
		},
		{
			name:       "host with port",
			url:        "vpx://user@vcenter:8443/dc/host/esxi",
			host:       "vcenter:8443",
			datacenter: "dc",
			insecure:   false,
		},
		{
			name:       "esx empty path",
			url:        "esx://esxi.example.com",
			host:       "esxi.example.com",
			datacenter: "",
			insecure:   false,
		},
		{
			name:       "empty URL",
			url:        "",
			host:       "",
			datacenter: "",
			insecure:   false,
		},
		{
			name:    "invalid URL",
			url:     "http://[",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			host, datacenter, insecure, err := parseLibvirtURL(tt.url)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if host != tt.host {
				t.Errorf("host = %q, want %q", host, tt.host)
			}
			if datacenter != tt.datacenter {
				t.Errorf("datacenter = %q, want %q", datacenter, tt.datacenter)
			}
			if insecure != tt.insecure {
				t.Errorf("insecure = %v, want %v", insecure, tt.insecure)
			}
		})
	}
}

func TestBuildCopyInput(t *testing.T) {
	t.Parallel()
	cfg := &config.AppConfig{
		LibvirtUrl:  "vpx://user@vcenter/dc/host/esxi?no_verify=1",
		VmName:      "my-vm",
		Fingerprint: "aa:bb:cc",
		Workdir:     "/var/tmp/v2v",
	}
	in, err := BuildCopyInput(cfg, []string{"[ds] vm/a.vmdk"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if in.Host != "vcenter" {
		t.Errorf("Host = %q, want vcenter", in.Host)
	}
	if in.Datacenter != "dc" {
		t.Errorf("Datacenter = %q, want dc", in.Datacenter)
	}
	if !in.Insecure {
		t.Error("Insecure = false, want true")
	}
	if in.CaCert != "" {
		t.Errorf("CaCert = %q, want empty", in.CaCert)
	}
	if in.VMName != "my-vm" || in.Fingerprint != "aa:bb:cc" {
		t.Errorf("unexpected identity fields: %+v", in)
	}
	if len(in.SourceDisks) != 1 || in.SourceDisks[0] != "[ds] vm/a.vmdk" {
		t.Errorf("SourceDisks = %v", in.SourceDisks)
	}
	if in.Workdir != "/var/tmp/v2v" {
		t.Errorf("Workdir = %q", in.Workdir)
	}

	data, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	if raw["insecure"] != true {
		t.Fatalf("JSON insecure = %v, want true", raw["insecure"])
	}
	if _, ok := raw["ca_cert"]; ok {
		t.Fatalf("ca_cert should be omitted when empty: %v", raw)
	}
}

func TestBuildCopyInputCaCertAndDefaultWorkdir(t *testing.T) {
	t.Parallel()
	cfg := &config.AppConfig{
		LibvirtUrl:  "esx://esxi.example.com",
		VmName:      "vm",
		Fingerprint: "ff",
	}
	in, err := BuildCopyInput(cfg, nil, kcCopyCaCert)
	if err != nil {
		t.Fatal(err)
	}
	if in.CaCert != kcCopyCaCert {
		t.Errorf("CaCert = %q, want %s", in.CaCert, kcCopyCaCert)
	}
	if in.Workdir != config.V2vOutputDir {
		t.Errorf("Workdir = %q, want default %s", in.Workdir, config.V2vOutputDir)
	}
	if in.Datacenter != "" {
		t.Errorf("Datacenter = %q, want empty for esx", in.Datacenter)
	}
	if in.Insecure {
		t.Error("Insecure should be false without no_verify")
	}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	v, ok := raw["source_disks"]
	if !ok {
		t.Fatalf("source_disks should be present (upstream tag has no omitempty): %v", raw)
	}
	if v != nil {
		t.Fatalf("source_disks = %v, want null", v)
	}
	if raw["ca_cert"] != kcCopyCaCert {
		t.Fatalf("JSON ca_cert = %v", raw["ca_cert"])
	}
}

func TestBuildCopyInputInvalidLibvirtURL(t *testing.T) {
	t.Parallel()
	cfg := &config.AppConfig{LibvirtUrl: "http://["}
	_, err := BuildCopyInput(cfg, nil, "")
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestSplitDiskPath(t *testing.T) {
	t.Parallel()
	if got := kccopy.SplitDiskPath(""); got != nil {
		t.Errorf("empty = %v, want nil", got)
	}
	got := kccopy.SplitDiskPath(" [ds] vm/a.vmdk , [ds] vm/b.vmdk ")
	if len(got) != 2 || got[0] != "[ds] vm/a.vmdk" || got[1] != "[ds] vm/b.vmdk" {
		t.Errorf("got %v", got)
	}
}

func TestRunKcCopyWritesInputAndExecs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFS := utils.NewMockFileSystem(ctrl)
	mockBuilder := utils.NewMockCommandBuilder(ctrl)
	mockExec := utils.NewMockCommandExecutor(ctrl)

	workdir := t.TempDir()
	c := &Conversion{
		AppConfig: &config.AppConfig{
			LibvirtUrl:  "vpx://user@vcenter/dc/cluster/esxi?no_verify=1",
			VmName:      "my-vm",
			Fingerprint: "aa:bb",
			Workdir:     workdir,
			Source:      config.VSPHERE,
		},
		CommandBuilder: mockBuilder,
		fileSystem:     mockFS,
	}

	mockFS.EXPECT().Stat(kcCopyCaCert).Return(nil, os.ErrNotExist)
	inputPath := filepath.Join(workdir, copyInputFileName)
	mockFS.EXPECT().WriteFile(inputPath, gomock.Any(), os.FileMode(0644)).DoAndReturn(
		func(name string, data []byte, perm os.FileMode) error {
			var in kccopy.CopyInput
			if err := json.Unmarshal(data, &in); err != nil {
				t.Fatalf("invalid JSON: %v", err)
			}
			if in.Host != "vcenter" || in.Datacenter != "dc" || !in.Insecure {
				t.Fatalf("unexpected input: %+v", in)
			}
			if len(in.SourceDisks) != 0 {
				t.Fatalf("SourceDisks = %v, want omitted/empty so kc-copy copies every NFC disk", in.SourceDisks)
			}
			if in.CaCert != "" {
				t.Fatalf("CaCert should be empty when secret is missing")
			}
			return nil
		},
	)

	outputPath := filepath.Join(workdir, copyOutputFile)
	mockBuilder.EXPECT().New(kcCopyBin).Return(mockBuilder)
	mockBuilder.EXPECT().AddArg("--input", inputPath).Return(mockBuilder)
	mockBuilder.EXPECT().AddArg("--output", outputPath).Return(mockBuilder)
	mockBuilder.EXPECT().Build().Return(mockExec)
	mockExec.EXPECT().SetStdout(os.Stdout)
	mockExec.EXPECT().SetStderr(os.Stderr)
	mockExec.EXPECT().Run().Return(nil)

	if err := c.RunKcCopy(); err != nil {
		t.Fatal(err)
	}
}

func TestRunKcCopySetsCaCertWhenMounted(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFS := utils.NewMockFileSystem(ctrl)
	mockBuilder := utils.NewMockCommandBuilder(ctrl)
	mockExec := utils.NewMockCommandExecutor(ctrl)

	c := &Conversion{
		AppConfig: &config.AppConfig{
			LibvirtUrl:  "esx://esxi.example.com",
			VmName:      "vm",
			Fingerprint: "ff",
			Workdir:     "/var/tmp/v2v",
			Source:      config.VSPHERE,
		},
		CommandBuilder: mockBuilder,
		fileSystem:     mockFS,
	}

	mockFS.EXPECT().Stat(kcCopyCaCert).Return(&utils.MockFileInfo{}, nil)
	mockFS.EXPECT().WriteFile(filepath.Join("/var/tmp/v2v", copyInputFileName), gomock.Any(), os.FileMode(0644)).DoAndReturn(
		func(name string, data []byte, perm os.FileMode) error {
			var in kccopy.CopyInput
			if err := json.Unmarshal(data, &in); err != nil {
				return err
			}
			if in.CaCert != kcCopyCaCert {
				t.Fatalf("CaCert = %q, want %s", in.CaCert, kcCopyCaCert)
			}
			return nil
		},
	)
	mockBuilder.EXPECT().New(kcCopyBin).Return(mockBuilder)
	mockBuilder.EXPECT().AddArg("--input", gomock.Any()).Return(mockBuilder)
	mockBuilder.EXPECT().AddArg("--output", gomock.Any()).Return(mockBuilder)
	mockBuilder.EXPECT().Build().Return(mockExec)
	mockExec.EXPECT().SetStdout(os.Stdout)
	mockExec.EXPECT().SetStderr(os.Stderr)
	mockExec.EXPECT().Run().Return(nil)

	if err := c.RunKcCopy(); err != nil {
		t.Fatal(err)
	}
}

func TestRunKcCopyRequiresFingerprint(t *testing.T) {
	t.Parallel()
	c := &Conversion{
		AppConfig: &config.AppConfig{
			LibvirtUrl: "vpx://user@vcenter/dc/host/esxi",
			VmName:     "vm",
		},
	}
	err := c.RunKcCopy()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), config.EnvFingerprintName) {
		t.Fatalf("error %q does not reference %s", err, config.EnvFingerprintName)
	}
}

func TestRunKcCopyInvalidLibvirtURL(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFS := utils.NewMockFileSystem(ctrl)
	c := &Conversion{
		AppConfig: &config.AppConfig{
			LibvirtUrl:  "http://[",
			VmName:      "vm",
			Fingerprint: "ff",
			Workdir:     t.TempDir(),
		},
		fileSystem: mockFS,
	}
	mockFS.EXPECT().Stat(kcCopyCaCert).Return(nil, os.ErrNotExist)

	err := c.RunKcCopy()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "parse libvirt URL") {
		t.Fatalf("error %q does not wrap parse context", err)
	}
}

func TestConvert(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *config.AppConfig
		disks   []*Disk
		setup   func(t *testing.T, c *Conversion, mockFS *utils.MockFileSystem, mockBuilder *utils.MockCommandBuilder)
		wantErr string
	}{
		{
			name: "in-place routes to RunInPlaceConversion",
			cfg: &config.AppConfig{
				IsInPlace:        true,
				OverlayEnabled:   false,
				Source:           config.VSPHERE,
				IsLocalMigration: false,
			},
			disks: []*Disk{{Link: "/var/tmp/v2v/vm-sda"}},
			setup: func(t *testing.T, c *Conversion, mockFS *utils.MockFileSystem, mockBuilder *utils.MockCommandBuilder) {
				t.Helper()
				mockExec := utils.NewMockCommandExecutor(gomock.NewController(t))
				mockBuilder.EXPECT().New("virt-v2v-in-place").Return(mockBuilder)
				mockBuilder.EXPECT().AddFlag("-v").Return(mockBuilder)
				mockBuilder.EXPECT().AddFlag("-x").Return(mockBuilder)
				mockBuilder.EXPECT().AddArg("-i", "disk").Return(mockBuilder)
				mockBuilder.EXPECT().AddArg("--root", "first").Return(mockBuilder)
				mockBuilder.EXPECT().AddPositional("/var/tmp/v2v/vm-sda").Return(mockBuilder)
				mockBuilder.EXPECT().Build().Return(mockExec)
				mockExec.EXPECT().SetStdout(os.Stdout)
				mockExec.EXPECT().SetStderr(os.Stderr)
				mockExec.EXPECT().Run().Return(nil)
			},
		},
		{
			name: "non-vSphere routes to RunVirtV2v",
			cfg: &config.AppConfig{
				Source:    config.OVA,
				DiskPath:  "/path/to/disk.ova",
				Workdir:   "/var/tmp/v2v",
				NewVmName: "new-vm",
			},
			setup: expectVirtV2vOVA,
		},
		{
			name: "vSphere remote routes to RunKcCopy",
			cfg: &config.AppConfig{
				Source:           config.VSPHERE,
				IsLocalMigration: false,
				LibvirtUrl:       "vpx://user@vcenter/dc/host/esxi",
				VmName:           "my-vm",
				Fingerprint:      "aa:bb",
				Workdir:          "/var/tmp/v2v",
			},
			setup:   expectKcCopy,
			wantErr: "failed to get domain XML",
		},
		{
			name: "vSphere local routes to RunKcCopy",
			cfg: &config.AppConfig{
				Source:           config.VSPHERE,
				IsLocalMigration: true,
				LibvirtUrl:       "vpx://user@vcenter/dc/host/esxi",
				VmName:           "my-vm",
				Fingerprint:      "aa:bb",
				Workdir:          "/var/tmp/v2v",
			},
			setup:   expectKcCopy,
			wantErr: "failed to get domain XML",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockFS := utils.NewMockFileSystem(ctrl)
			mockBuilder := utils.NewMockCommandBuilder(ctrl)
			c := &Conversion{
				AppConfig:      tt.cfg,
				Disks:          tt.disks,
				CommandBuilder: mockBuilder,
				fileSystem:     mockFS,
			}
			tt.setup(t, c, mockFS, mockBuilder)

			err := c.Convert()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Convert() unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("Convert() expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Convert() error %q, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func expectKcCopy(t *testing.T, c *Conversion, mockFS *utils.MockFileSystem, mockBuilder *utils.MockCommandBuilder) {
	t.Helper()
	ctrl := gomock.NewController(t)
	mockExec := utils.NewMockCommandExecutor(ctrl)
	workdir := c.Workdir
	if workdir == "" {
		workdir = config.V2vOutputDir
	}
	mockFS.EXPECT().Stat(kcCopyCaCert).Return(nil, os.ErrNotExist)
	mockFS.EXPECT().WriteFile(filepath.Join(workdir, copyInputFileName), gomock.Any(), os.FileMode(0644)).Return(nil)
	mockBuilder.EXPECT().New(kcCopyBin).Return(mockBuilder)
	mockBuilder.EXPECT().AddArg("--input", filepath.Join(workdir, copyInputFileName)).Return(mockBuilder)
	mockBuilder.EXPECT().AddArg("--output", filepath.Join(workdir, copyOutputFile)).Return(mockBuilder)
	mockBuilder.EXPECT().Build().Return(mockExec)
	mockExec.EXPECT().SetStdout(os.Stdout)
	mockExec.EXPECT().SetStderr(os.Stderr)
	mockExec.EXPECT().Run().Return(nil)
}

func expectVirtV2vOVA(t *testing.T, c *Conversion, mockFS *utils.MockFileSystem, mockBuilder *utils.MockCommandBuilder) {
	t.Helper()
	ctrl := gomock.NewController(t)
	v2vExec := utils.NewMockCommandExecutor(ctrl)
	monitorExec := utils.NewMockCommandExecutor(ctrl)

	mockBuilder.EXPECT().New("virt-v2v").Return(mockBuilder)
	mockBuilder.EXPECT().AddFlag("-v").Return(mockBuilder)
	mockBuilder.EXPECT().AddFlag("-x").Return(mockBuilder)
	mockBuilder.EXPECT().AddArg("-o", "kubevirt").Return(mockBuilder)
	mockBuilder.EXPECT().AddArg("-os", c.Workdir).Return(mockBuilder)
	mockBuilder.EXPECT().AddArg("-on", c.NewVmName).Return(mockBuilder)
	mockBuilder.EXPECT().AddArg("-i", "ova").Return(mockBuilder)
	mockBuilder.EXPECT().AddPositional(c.DiskPath).Return(mockBuilder)
	mockBuilder.EXPECT().Build().Return(v2vExec)
	mockBuilder.EXPECT().New("/usr/local/bin/virt-v2v-monitor").Return(mockBuilder)
	mockBuilder.EXPECT().Build().Return(monitorExec)

	monitorExec.EXPECT().SetStdout(os.Stdout)
	monitorExec.EXPECT().SetStderr(os.Stderr)
	monitorExec.EXPECT().SetStdin(gomock.Any())
	v2vExec.EXPECT().SetStdout(gomock.Any())
	v2vExec.EXPECT().SetStderr(gomock.Any())
	monitorExec.EXPECT().Start().Return(nil)
	v2vExec.EXPECT().Run().Return(nil)
	monitorExec.EXPECT().Wait().Return(nil)
}
