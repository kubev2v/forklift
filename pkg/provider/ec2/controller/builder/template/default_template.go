package template

// DefaultVMTemplate is the Go text/template that produces a KubeVirt VirtualMachineSpec
// YAML from VMBuildValues.
const DefaultVMTemplate = `---
runStrategy: {{ .RunStrategy }}
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
        inputs:
        {{- if .InputBus }}
        - type: tablet
          name: tablet
          bus: {{ .InputBus }}
        {{- end }}
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
          {{- if .MACAddress }}
          macAddress: "{{ .MACAddress }}"
          {{- end }}
          {{- if .IsUDNPod }}
          binding:
            name: l2bridge
          {{- else if eq .BindingMethod "masquerade" }}
          masquerade: {}
          {{- else if eq .BindingMethod "bridge" }}
          bridge: {}
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
    {{- if .NodeSelector }}
    nodeSelector:
      {{- range $key, $value := .NodeSelector }}
      {{ $key }}: "{{ $value }}"
      {{- end }}
    {{- end }}
`
