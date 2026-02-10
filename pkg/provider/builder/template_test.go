package builder

import (
	"testing"

	cnv "kubevirt.io/api/core/v1"
)

func TestRenderTemplate_BasicBIOS(t *testing.T) {
	values := &VMBuildValues{
		Name:         "test-vm",
		ID:           "i-0abc123",
		InstanceType: "m5.large",
		Sockets:      1,
		Cores:        2,
		MemoryMiB:    8192,
		IsUEFI:       false,
		HasACPI:      true,
		Serial:       "i-0abc123",
		InputBus:     "virtio",
		Disks: []DiskBuildValues{
			{Name: "disk-0", PVCName: "pvc-boot", Bus: "virtio", IsBootDisk: true, BootOrder: 1},
			{Name: "disk-1", PVCName: "pvc-data", Bus: "virtio"},
		},
		Networks: []NetworkBuildValues{
			{Name: "net-0", Type: "pod", Model: "virtio", BindingMethod: "masquerade"},
		},
	}

	tmpl := `runStrategy: Halted
template:
  spec:
    domain:
      resources:
        requests:
          memory: "{{ .MemoryMiB }}Mi"
      cpu:
        sockets: {{ .Sockets }}
        cores: {{ .Cores }}
      firmware:
        serial: "{{ .Serial }}"
        bootloader:
          {{- if .IsUEFI }}
          efi:
            secureBoot: false
          {{- else }}
          bios: {}
          {{- end }}
      devices:
        disks:
        {{- range .Disks }}
        - name: {{ .Name }}
          disk:
            bus: {{ .Bus }}
          {{- if .IsBootDisk }}
          bootOrder: 1
          {{- end }}
        {{- end }}
        interfaces:
        {{- range .Networks }}
        - name: {{ .Name }}
          model: {{ .Model }}
          {{- if eq .BindingMethod "masquerade" }}
          masquerade: {}
          {{- end }}
        {{- end }}
    networks:
    {{- range .Networks }}
    - name: {{ .Name }}
      {{- if eq .Type "pod" }}
      pod: {}
      {{- end }}
    {{- end }}
    volumes:
    {{- range .Disks }}
    - name: {{ .Name }}
      persistentVolumeClaim:
        claimName: "{{ .PVCName }}"
    {{- end }}
`

	spec, err := RenderTemplate(tmpl, values)
	if err != nil {
		t.Fatalf("RenderTemplate failed: %v", err)
	}

	// Check run strategy
	if spec.RunStrategy == nil || *spec.RunStrategy != cnv.RunStrategyHalted {
		t.Errorf("expected RunStrategy Halted, got %v", spec.RunStrategy)
	}

	// Check memory
	mem := spec.Template.Spec.Domain.Resources.Requests["memory"]
	if mem.String() != "8Gi" {
		t.Errorf("expected memory 8Gi, got %s", mem.String())
	}

	// Check CPU
	if spec.Template.Spec.Domain.CPU.Sockets != 1 {
		t.Errorf("expected 1 socket, got %d", spec.Template.Spec.Domain.CPU.Sockets)
	}
	if spec.Template.Spec.Domain.CPU.Cores != 2 {
		t.Errorf("expected 2 cores, got %d", spec.Template.Spec.Domain.CPU.Cores)
	}

	// Check firmware - BIOS
	if spec.Template.Spec.Domain.Firmware == nil {
		t.Fatal("expected firmware to be set")
	}
	if spec.Template.Spec.Domain.Firmware.Bootloader == nil || spec.Template.Spec.Domain.Firmware.Bootloader.BIOS == nil {
		t.Error("expected BIOS bootloader")
	}
	if spec.Template.Spec.Domain.Firmware.Serial != "i-0abc123" {
		t.Errorf("expected serial i-0abc123, got %s", spec.Template.Spec.Domain.Firmware.Serial)
	}

	// Check disks
	if len(spec.Template.Spec.Domain.Devices.Disks) != 2 {
		t.Fatalf("expected 2 disks, got %d", len(spec.Template.Spec.Domain.Devices.Disks))
	}
	if spec.Template.Spec.Domain.Devices.Disks[0].Name != "disk-0" {
		t.Errorf("expected disk-0, got %s", spec.Template.Spec.Domain.Devices.Disks[0].Name)
	}
	if spec.Template.Spec.Domain.Devices.Disks[0].BootOrder == nil || *spec.Template.Spec.Domain.Devices.Disks[0].BootOrder != 1 {
		t.Error("expected boot order 1 on first disk")
	}
	if spec.Template.Spec.Domain.Devices.Disks[1].BootOrder != nil {
		t.Error("expected no boot order on second disk")
	}

	// Check networks
	if len(spec.Template.Spec.Networks) != 1 {
		t.Fatalf("expected 1 network, got %d", len(spec.Template.Spec.Networks))
	}
	if spec.Template.Spec.Networks[0].Pod == nil {
		t.Error("expected pod network")
	}

	// Check volumes
	if len(spec.Template.Spec.Volumes) != 2 {
		t.Fatalf("expected 2 volumes, got %d", len(spec.Template.Spec.Volumes))
	}
	if spec.Template.Spec.Volumes[0].PersistentVolumeClaim.ClaimName != "pvc-boot" {
		t.Errorf("expected pvc-boot, got %s", spec.Template.Spec.Volumes[0].PersistentVolumeClaim.ClaimName)
	}

	// Check interfaces
	if len(spec.Template.Spec.Domain.Devices.Interfaces) != 1 {
		t.Fatalf("expected 1 interface, got %d", len(spec.Template.Spec.Domain.Devices.Interfaces))
	}
	if spec.Template.Spec.Domain.Devices.Interfaces[0].Masquerade == nil {
		t.Error("expected masquerade binding")
	}
}

func TestRenderTemplate_UEFIWithSMM(t *testing.T) {
	values := &VMBuildValues{
		Sockets:   1,
		Cores:     4,
		MemoryMiB: 16384,
		IsUEFI:    true,
		HasACPI:   true,
		HasSMM:    true,
		Serial:    "i-0def456",
		InputBus:  "virtio",
		Disks: []DiskBuildValues{
			{Name: "disk-0", PVCName: "boot-pvc", Bus: "virtio", IsBootDisk: true, BootOrder: 1},
		},
		Networks: []NetworkBuildValues{
			{Name: "net-0", Type: "pod", Model: "virtio", BindingMethod: "masquerade"},
		},
	}

	tmpl := `runStrategy: Halted
template:
  spec:
    domain:
      resources:
        requests:
          memory: "{{ .MemoryMiB }}Mi"
      cpu:
        sockets: {{ .Sockets }}
        cores: {{ .Cores }}
      firmware:
        serial: "{{ .Serial }}"
        bootloader:
          {{- if .IsUEFI }}
          efi:
            secureBoot: false
          {{- else }}
          bios: {}
          {{- end }}
      features:
        acpi: {}
        {{- if .HasSMM }}
        smm:
          enabled: true
        {{- end }}
      devices:
        disks:
        {{- range .Disks }}
        - name: {{ .Name }}
          disk:
            bus: {{ .Bus }}
          {{- if .IsBootDisk }}
          bootOrder: 1
          {{- end }}
        {{- end }}
        interfaces:
        {{- range .Networks }}
        - name: {{ .Name }}
          model: {{ .Model }}
          masquerade: {}
        {{- end }}
    networks:
    {{- range .Networks }}
    - name: {{ .Name }}
      pod: {}
    {{- end }}
    volumes:
    {{- range .Disks }}
    - name: {{ .Name }}
      persistentVolumeClaim:
        claimName: "{{ .PVCName }}"
    {{- end }}
`

	spec, err := RenderTemplate(tmpl, values)
	if err != nil {
		t.Fatalf("RenderTemplate failed: %v", err)
	}

	// Check UEFI firmware
	if spec.Template.Spec.Domain.Firmware.Bootloader.EFI == nil {
		t.Fatal("expected EFI bootloader")
	}

	// Check SMM feature
	if spec.Template.Spec.Domain.Features == nil {
		t.Fatal("expected features to be set")
	}
	if spec.Template.Spec.Domain.Features.SMM == nil {
		t.Fatal("expected SMM feature")
	}
}

func TestRenderTemplate_CPUFeatures(t *testing.T) {
	values := &VMBuildValues{
		Sockets:   1,
		Cores:     96,
		MemoryMiB: 393216,
		Serial:    "i-metal001",
		IsUEFI:    false,
		HasACPI:   true,
		InputBus:  "virtio",
		CPUFeatures: []CPUFeatureBuildValues{
			{Name: "vmx", Policy: "optional"},
			{Name: "svm", Policy: "optional"},
		},
		Disks: []DiskBuildValues{
			{Name: "disk-0", PVCName: "boot", Bus: "virtio", IsBootDisk: true, BootOrder: 1},
		},
		Networks: []NetworkBuildValues{
			{Name: "net-0", Type: "pod", Model: "virtio", BindingMethod: "masquerade"},
		},
	}

	tmpl := `runStrategy: Halted
template:
  spec:
    domain:
      resources:
        requests:
          memory: "{{ .MemoryMiB }}Mi"
      cpu:
        sockets: {{ .Sockets }}
        cores: {{ .Cores }}
        {{- if .CPUFeatures }}
        features:
        {{- range .CPUFeatures }}
        - name: {{ .Name }}
          policy: {{ .Policy }}
        {{- end }}
        {{- end }}
      firmware:
        serial: "{{ .Serial }}"
        bootloader:
          bios: {}
      devices:
        disks:
        {{- range .Disks }}
        - name: {{ .Name }}
          disk:
            bus: {{ .Bus }}
          {{- if .IsBootDisk }}
          bootOrder: 1
          {{- end }}
        {{- end }}
        interfaces:
        {{- range .Networks }}
        - name: {{ .Name }}
          model: {{ .Model }}
          masquerade: {}
        {{- end }}
    networks:
    {{- range .Networks }}
    - name: {{ .Name }}
      pod: {}
    {{- end }}
    volumes:
    {{- range .Disks }}
    - name: {{ .Name }}
      persistentVolumeClaim:
        claimName: "{{ .PVCName }}"
    {{- end }}
`

	spec, err := RenderTemplate(tmpl, values)
	if err != nil {
		t.Fatalf("RenderTemplate failed: %v", err)
	}

	// Check CPU features
	if len(spec.Template.Spec.Domain.CPU.Features) != 2 {
		t.Fatalf("expected 2 CPU features, got %d", len(spec.Template.Spec.Domain.CPU.Features))
	}
	if spec.Template.Spec.Domain.CPU.Features[0].Name != "vmx" {
		t.Errorf("expected vmx feature, got %s", spec.Template.Spec.Domain.CPU.Features[0].Name)
	}
	if spec.Template.Spec.Domain.CPU.Features[0].Policy != "optional" {
		t.Errorf("expected optional policy, got %s", spec.Template.Spec.Domain.CPU.Features[0].Policy)
	}
}

func TestRenderTemplate_MultusNetwork(t *testing.T) {
	values := &VMBuildValues{
		Sockets:   1,
		Cores:     2,
		MemoryMiB: 4096,
		Serial:    "i-test",
		InputBus:  "virtio",
		Disks: []DiskBuildValues{
			{Name: "disk-0", PVCName: "boot", Bus: "virtio", IsBootDisk: true, BootOrder: 1},
		},
		Networks: []NetworkBuildValues{
			{Name: "net-0", Type: "multus", MultusName: "default/my-net", Model: "virtio", BindingMethod: "bridge"},
		},
	}

	tmpl := `runStrategy: Halted
template:
  spec:
    domain:
      resources:
        requests:
          memory: "{{ .MemoryMiB }}Mi"
      cpu:
        sockets: {{ .Sockets }}
        cores: {{ .Cores }}
      firmware:
        serial: "{{ .Serial }}"
        bootloader:
          bios: {}
      devices:
        disks:
        {{- range .Disks }}
        - name: {{ .Name }}
          disk:
            bus: {{ .Bus }}
          {{- if .IsBootDisk }}
          bootOrder: 1
          {{- end }}
        {{- end }}
        interfaces:
        {{- range .Networks }}
        - name: {{ .Name }}
          model: {{ .Model }}
          {{- if eq .BindingMethod "bridge" }}
          bridge: {}
          {{- else }}
          masquerade: {}
          {{- end }}
        {{- end }}
    networks:
    {{- range .Networks }}
    - name: {{ .Name }}
      {{- if eq .Type "pod" }}
      pod: {}
      {{- else if eq .Type "multus" }}
      multus:
        networkName: "{{ .MultusName }}"
      {{- end }}
    {{- end }}
    volumes:
    {{- range .Disks }}
    - name: {{ .Name }}
      persistentVolumeClaim:
        claimName: "{{ .PVCName }}"
    {{- end }}
`

	spec, err := RenderTemplate(tmpl, values)
	if err != nil {
		t.Fatalf("RenderTemplate failed: %v", err)
	}

	// Check multus network
	if len(spec.Template.Spec.Networks) != 1 {
		t.Fatalf("expected 1 network, got %d", len(spec.Template.Spec.Networks))
	}
	if spec.Template.Spec.Networks[0].Multus == nil {
		t.Fatal("expected multus network")
	}
	if spec.Template.Spec.Networks[0].Multus.NetworkName != "default/my-net" {
		t.Errorf("expected default/my-net, got %s", spec.Template.Spec.Networks[0].Multus.NetworkName)
	}

	// Check bridge binding
	if spec.Template.Spec.Domain.Devices.Interfaces[0].Bridge == nil {
		t.Error("expected bridge binding")
	}
}

func TestRenderTemplate_NodeSelector(t *testing.T) {
	values := &VMBuildValues{
		Sockets:   1,
		Cores:     2,
		MemoryMiB: 4096,
		Serial:    "i-test",
		InputBus:  "virtio",
		NodeSelector: map[string]string{
			"topology.kubernetes.io/zone": "us-east-1a",
		},
		Disks: []DiskBuildValues{
			{Name: "disk-0", PVCName: "boot", Bus: "virtio", IsBootDisk: true, BootOrder: 1},
		},
		Networks: []NetworkBuildValues{
			{Name: "net-0", Type: "pod", Model: "virtio", BindingMethod: "masquerade"},
		},
	}

	tmpl := `runStrategy: Halted
template:
  spec:
    domain:
      resources:
        requests:
          memory: "{{ .MemoryMiB }}Mi"
      cpu:
        sockets: {{ .Sockets }}
        cores: {{ .Cores }}
      firmware:
        serial: "{{ .Serial }}"
        bootloader:
          bios: {}
      devices:
        disks:
        {{- range .Disks }}
        - name: {{ .Name }}
          disk:
            bus: {{ .Bus }}
          {{- if .IsBootDisk }}
          bootOrder: 1
          {{- end }}
        {{- end }}
        interfaces:
        {{- range .Networks }}
        - name: {{ .Name }}
          masquerade: {}
        {{- end }}
    networks:
    {{- range .Networks }}
    - name: {{ .Name }}
      pod: {}
    {{- end }}
    volumes:
    {{- range .Disks }}
    - name: {{ .Name }}
      persistentVolumeClaim:
        claimName: "{{ .PVCName }}"
    {{- end }}
    {{- if .NodeSelector }}
    nodeSelector:
      {{- range $key, $value := .NodeSelector }}
      {{ $key }}: "{{ $value }}"
      {{- end }}
    {{- end }}
`

	spec, err := RenderTemplate(tmpl, values)
	if err != nil {
		t.Fatalf("RenderTemplate failed: %v", err)
	}

	if spec.Template.Spec.NodeSelector == nil {
		t.Fatal("expected node selector to be set")
	}
	if spec.Template.Spec.NodeSelector["topology.kubernetes.io/zone"] != "us-east-1a" {
		t.Errorf("expected zone us-east-1a, got %s", spec.Template.Spec.NodeSelector["topology.kubernetes.io/zone"])
	}
}

func TestRenderTemplate_InvalidTemplate(t *testing.T) {
	values := &VMBuildValues{}
	_, err := RenderTemplate("{{ .NonExistent | badFunc }}", values)
	if err == nil {
		t.Error("expected error for invalid template")
	}
}

func TestRenderTemplate_InvalidYAML(t *testing.T) {
	values := &VMBuildValues{}
	// This will render to invalid YAML that can't be unmarshaled into VirtualMachineSpec
	_, err := RenderTemplate("not: [valid: yaml: here", values)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}
