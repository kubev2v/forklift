# HyperV OVF Generator

Lightweight Windows tool that generates OVF metadata files for HyperV VMs.

## Overview

This tool runs **locally on a Windows HyperV host** and:
1. Queries HyperV for VM metadata (CPU, RAM, disks, NICs, guest OS)
2. Generates OVF files next to VHDX disk files
3. Uses KVP Exchange for guest OS detection (no credentials needed)

## Extracting the Tool

The Windows binary is included in the HyperV provider server container image.

### From Container Image

```bash
# Create temporary container
podman create --name tmp quay.io/kubev2v/forklift-hyperv-provider-server:latest

# Extract the tool
podman cp tmp:/usr/share/forklift/tools/ovf-generator.exe ./

# Cleanup
podman rm tmp
```

### From Running Pod

```bash
kubectl cp <namespace>/<hyperv-pod>:/usr/share/forklift/tools/ovf-generator.exe ./ovf-generator.exe
```

## Usage

Copy `ovf-generator.exe` to your Windows HyperV host and run:

```powershell
# Show help
.\ovf-generator.exe -help

# Generate OVF for all VMs
.\ovf-generator.exe

# Generate OVF only for VMs with disks under a specific path
.\ovf-generator.exe -path "C:\Hyper-V\Virtual Hard Disks"
```

## Help Output

```
ovf-generator v1.0.0 - HyperV OVF Metadata Generator

DESCRIPTION
  Generates OVF metadata files for HyperV VMs to enable migration to
  OpenShift Virtualization via Forklift.

  This tool runs locally on a Windows HyperV host and:
  1. Queries HyperV for VM metadata (CPU, RAM, disks, NICs)
  2. Detects guest OS via KVP Exchange (Integration Services)
  3. Generates OVF files next to VHDX disk files

USAGE
  ovf-generator [flags]

FLAGS
  -path <directory>   Only process VMs with disks under this path.
                      If not specified, processes all VMs.
  -help               Show this help information.
```

## Requirements

- Windows Server with HyperV
- PowerShell (built-in)
- VMs can be **on or off**
  - **Running VMs:** Guest OS auto-detected via Integration Services (KVP Exchange)
  - **Stopped VMs:** Guest OS defaults to "Unknown" (can be manually edited in OVF)

## Output

For each VM, generates an OVF file in the same directory as the first VHDX:

**Single-disk VMs:**
```
C:\Hyper-V\Virtual Hard Disks\
├── vm1\
│   ├── vm1.vhdx
│   └── vm1.ovf      ← Generated
└── vm2\
    ├── vm2.vhdx
    └── vm2.ovf      ← Generated
```

**Multi-disk VMs:**
```
C:\VMs\
└── win-2019\
    ├── win-2019.vhdx     ← Disk 1
    ├── 2nd_disk.vhdx     ← Disk 2
    └── win-2019.ovf      ← Generated, references both disks by filename
```

**Important:** All VHDX files for a VM must be in the same directory as the OVF file. The OVF references disks by filename only (not full paths).

## Example Output

```
Querying local HyperV for VMs...
Found 2 VM(s)

Processing VM: win-2019-vm
  Disks: C:\VMs\win-2019-vm\win-2019-vm.vhdx
  OVF file written to: C:\VMs\win-2019-vm\win-2019-vm.ovf

Processing VM: rhel9-vm
  Disks: C:\VMs\rhel9-vm\rhel9-vm.vhdx, C:\VMs\rhel9-vm\data.vhdx
  OVF file written to: C:\VMs\rhel9-vm\rhel9-vm.ovf

Generated 2 OVF file(s)
```

### CI Integration

The tests are designed to run in CI without requiring Windows or HyperV:

1. **No external dependencies**: Tests use `MockPSExecutor` to simulate PowerShell
2. **Example inputs/outputs**: Tests include realistic PowerShell JSON responses
3. **Cross-platform**: Tests run on Linux CI runners

#### Running in CI

```yaml
# Example GitHub Actions workflow
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.21'
      - name: Test OVF Generator
        run: |
          cd cmd/ovf-generator
          go test -v ./...
```

### Test Coverage

The tests cover:

- **VM listing**: Parsing PowerShell output, empty results, errors
- **VM info parsing**: Single/multi disk VMs, memory/CPU extraction
- **Guest OS detection**: Windows/Linux VMs, offline VMs, WMI errors
- **Disk path extraction**: Arrays, single objects, empty paths
- **Path filtering**: Processing only VMs in specified directories

### Adding New Test Cases

To add test cases for new PowerShell commands:

```go
// 1. Define expected output as a constant
const myCommandOutput = `{
  "Key": "value"
}`

// 2. Register mock response in test
mockExecutor.AddResponse("My-PowerShell-Command", myCommandOutput, nil)

// 3. Call the function under test
result, err := generator.myFunction()

// 4. Assert expectations
Expect(err).ToNot(HaveOccurred())
Expect(result["Key"]).To(Equal("value"))
```

## Notes

- **Guest OS detection** requires VMs to be running with Integration Services (enabled by default on Windows 10+ and modern Linux)
- VMs are **not shut down** - tool only reads metadata
- OVF files reference the VHDX by filename (relative path)
- If guest OS cannot be detected, it defaults to "Unknown" in the OVF

### Checking Integration Services

On HyperV host:
```powershell
# Check Integration Services state for all VMs
Get-VM | Select-Object Name, IntegrationServicesState
```

Inside Windows guest VM:
```powershell
# Check if Integration Services are running
Get-Service -Name "Hyper-V*"
```

Inside Linux guest VM:
```bash
# Check if hv_utils module is loaded
lsmod | grep hv_utils

# Check Hyper-V daemon services
systemctl status hv-kvp-daemon.service
systemctl status hv-vss-daemon.service
```
