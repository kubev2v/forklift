# vSphere XCOPY Volume Populator E2E Tests

This directory contains end-to-end tests for the vSphere XCOPY volume populator component in Forklift. The tests validate copy-offload functionality for VM disk migration from VMware vSphere to OpenShift using XCOPY technology.

## Overview

The vSphere XCOPY volume populator leverages storage array copy-offload capabilities to dramatically accelerate VM disk migrations. Instead of copying data through the host, XCOPY operations are performed directly on the storage array, reducing migration time from hours to minutes for large volumes.

### What This Test Framework Does

1.  **Creates test VMs** in VMware vSphere with configurable OS types and hardware
2.  **Utilizes pre-configured storage** with XCOPY-capable backends (NetApp ONTAP, Hitachi Vantara)
3.  **Migrates VM disks** from vSphere to OpenShift using copy-offload technology
4.  **Verifies XCOPY usage** by inspecting populator resources and logs
5.  **Includes steps for validating data integrity** to ensure successful migration
6.  **Generates comprehensive logs** for troubleshooting

## Prerequisites

### Required Tools

-   **Go** (1.19+) - for running the test framework
-   **Ansible** (2.9+) - for VM provisioning and management
-   **OpenShift CLI (oc)** - for OpenShift operations

### Required Access

-   **VMware vSphere** with administrative privileges
-   **OpenShift cluster** with Forklift operator installed
-   **Storage array** with XCOPY support (NetApp ONTAP or Hitachi Vantara)
-   **Network connectivity** between all components

### Storage Requirements

The storage backend must support XCOPY operations:
-   **NetApp ONTAP** - iSCSI or FC volumes
-   **Hitachi Vantara** - compatible storage arrays
-   **Shared storage** between vSphere and OpenShift

## Directory Structure

```
e2e-tests/
├── README.md                     # This file
├── tests/                        # Go test files for different disk types
│   ├── test_base_migration.go    # Main test framework logic
│   └── logs/                     # Test execution logs
├── helpers/                      # Go helper libraries
│   ├── common.go                 # Common utilities and logging
│   └── openshift.go              # OpenShift client operations
├── ansible/                      # VM provisioning automation
│   ├── setup-vm.yml              # VM creation playbook
│   ├── teardown-vm.yml           # VM cleanup playbook
│   └── inventory.ini.template    # Ansible inventory template
└── config/                       # Configuration files
    └── test-config.env           # Example configuration
```

## Quick Start

### 1. Clone and Setup

```bash
# Navigate to the e2e-tests directory
cd cmd/vsphere-xcopy-volume-populator/e2e-tests

# Create a local configuration file, which is ignored by git
cp config/test-config.env config/test-config.env.local
```

### 2. Configure Environment

Edit `config/test-config.env.local` with your environment details:

```bash
# VMware vSphere
export VSPHERE_HOST="vcenter.example.com"
export VSPHERE_USERNAME="administrator@vsphere.local"
export VSPHERE_PASSWORD="password"
export VSPHERE_DATACENTER="Datacenter"
export VSPHERE_DATASTORE="netapp-iscsi-ds"  # XCOPY-capable storage

# OpenShift
export OCP_API_URL="https://api.cluster.example.com:6443"
export OCP_USERNAME="admin"
export OCP_PASSWORD="password"
export OCP_STORAGE_CLASS="netapp-iscsi-sc"

# Storage Array
export STORAGE_VENDOR_PRODUCT="ontap"
export STORAGE_HOSTNAME="netapp.example.com"
export STORAGE_USERNAME="admin"
export STORAGE_PASSWORD="password"
```

### 3. Run Tests

```bash
# Run the complete test suite
go test -v ./tests/

# Run a specific test for a disk type
go test -v -run '^TestCopyOffloadThin$'

# Run with debug logging
DEBUG_MODE=true go test -v ./tests/
```

## Containerized Testing with Podman or Docker

You can run the E2E tests in a containerized environment using Podman or Docker. This simplifies setup by providing all necessary dependencies in a pre-built image.

### 1. Build the Container Image

From the project root directory (`forklift/`):

```bash
podman build -t forklift-e2e-tests -f cmd/vsphere-xcopy-volume-populator/e2e-tests/Dockerfile .
```

### 2. Run the Tests

You can run the tests by passing your environment variables to the `podman run` command. You can either pass them directly or use a `.env` file.

**Important**: If you are using the remote execution feature (`SSH_HOST`), you must mount your SSH private key into the container so it can connect to the remote host.

#### Using a Configuration File (Recommended)

Create a `test-config.env` file (you can copy `cmd/vsphere-xcopy-volume-populator/e2e-tests/config/test-config.env` as a template). Then, assuming your private key is at `~/.ssh/id_rsa`, run:

```bash
podman run --rm \
  --env-file=path/to/your/test-config.env \
  -v ~/.ssh/id_rsa:/home/runner/.ssh/id_rsa:Z \
  forklift-e2e-tests
```
*Note: The `:Z` flag is for SELinux systems and may not be required on all platforms.*

#### Passing Environment Variables Directly

```bash
podman run --rm \
  -v ~/.ssh/id_rsa:/home/runner/.ssh/id_rsa:Z \
  -e VSPHERE_HOST="vcenter.example.com" \
  -e VSPHERE_USERNAME="administrator@vsphere.local" \
  # ... other variables
  forklift-e2e-tests
```

## Configuration

### Environment Variables

The test framework uses environment variables for configuration. Key variables include:

#### VMware vSphere Configuration
-   `VSPHERE_HOST` - vCenter server hostname
-   `VSPHERE_USERNAME` - vSphere username
-   `VSPHERE_PASSWORD` - vSphere password
-   `VSPHERE_DATACENTER` - Target datacenter
-   `VSPHERE_DATASTORE` - XCOPY-capable datastore
-   `VSPHERE_NETWORK` - VM network

#### OpenShift Configuration
-   `OCP_API_URL` - OpenShift API endpoint
-   `OCP_USERNAME` - OpenShift username
-   `OCP_PASSWORD` - OpenShift password
-   `OCP_NAMESPACE` - Forklift namespace (default: openshift-mtv)
-   `OCP_STORAGE_CLASS` - Storage class for migrated volumes

#### Storage Configuration
-   `STORAGE_VENDOR_PRODUCT` - Storage vendor (ontap, vantara)
-   `STORAGE_HOSTNAME` - Storage array hostname
-   `STORAGE_USERNAME` - Storage admin username
-   `STORAGE_PASSWORD` - Storage admin password

#### Test VM Configuration
-   `VM_OS_TYPE` - Operating system type (see supported OS types).
-   `VM_TEMPLATE_NAME` - VM template name to clone from (optional).
-   `VM_DISK_SIZE_GB` - VM disk size in GB (default: 20).
-   `VM_MEMORY_MB` - VM memory in MB (default: 2048).
-   `VM_CPU_COUNT` - Number of vCPUs (default: 2).

### Supported Operating Systems

The framework supports multiple operating systems with intelligent defaults:

#### Linux Distributions
-   `linux-centos7` - CentOS 7
-   `linux-centos8` - CentOS 8
-   `linux-rhel7` - Red Hat Enterprise Linux 7
-   `linux-rhel8` - Red Hat Enterprise Linux 8
-   `linux-rhel9` - Red Hat Enterprise Linux 9
-   `linux-ubuntu1804` - Ubuntu 18.04 LTS
-   `linux-ubuntu2004` - Ubuntu 20.04 LTS
-   `linux-ubuntu2204` - Ubuntu 22.04 LTS

#### Windows Versions
-   `windows-2016` - Windows Server 2016
-   `windows-2019` - Windows Server 2019
-   `windows-2022` - Windows Server 2022
-   `windows-10` - Windows 10
-   `windows-11` - Windows 11

## Test Workflow

### 1. Prerequisites Validation
-   Checks required tools (ansible, oc)
-   Validates environment variables
-   Verifies project structure

### 2. VM Provisioning
-   Creates test VM in vSphere using Ansible
-   Supports template cloning or ISO installation
-   Configures hardware based on OS type

### 3. OpenShift Environment Setup
-   Authenticates to OpenShift cluster
-   Creates necessary resources and configurations
-   Sets up storage classes and providers

### 4. Migration Execution
-   Creates Forklift migration plan
-   Executes copy-offload migration
-   Monitors progress and logs

### 5. XCOPY Verification
-   Checks for the creation of `VSphereXcopyVolumePopulator` resources.
-   Validates XCOPY usage by inspecting populator and controller logs.

### 6. Data Integrity Validation
-   Boots migrated VM in OpenShift
-   Verifies VM functionality

### 7. Cleanup
-   Removes test VMs and resources
-   Saves logs to the `tests/logs` directory

## VM Creation Methods

The framework supports three VM creation methods:

### 1. Template Cloning (Recommended)
```bash
export VM_TEMPLATE_NAME="rhel8-template"
```
-   Fastest method for testing
-   Requires pre-existing templates
-   Consistent and reliable

### 2. ISO Installation
```bash
export VM_ISO_PATH="[datastore] iso/rhel-8.7-x86_64-dvd.iso"
```
-   Creates VM from ISO image
-   Requires longer setup time
-   Useful when templates unavailable

### 3. Empty VM Creation
```bash
# Leave both VM_TEMPLATE_NAME and VM_ISO_PATH empty
```
-   Creates empty VM for advanced testing
-   Requires manual OS installation
-   For specialized test scenarios

## Logging and Artifacts

### Log Files
-   Test execution logs: `tests/logs/test_YYYYMMDD_HHMMSS.log`
-   Ansible output is captured within the main test log.
-   Migration details are printed to the log upon failure.

## Troubleshooting

### Common Issues

#### VM Creation Fails
```bash
# Check vSphere connectivity
ansible-playbook -i localhost, ansible/setup-vm.yml --check

# Verify datastore accessibility
# Check VM template existence
# Validate network configuration
```

#### Migration Timeout
```bash
# Increase timeout values for plan validation and migration execution
export PLAN_TIMEOUT="600s"      # 10 minutes
export MIGRATION_TIMEOUT="3600s"  # 1 hour
```

#### XCOPY Not Detected
```bash
# Verify storage array support
# Check iSCSI multipath configuration
# Validate storage class settings
# Ensure shared storage between vSphere and OpenShift
```

### Debug Mode
Enable debug logging for detailed troubleshooting:
```bash
export DEBUG_MODE="true"
go test -v ./tests/
```

## Integration with CI/CD

### GitHub Actions
```yaml
- name: Run XCOPY E2E Tests
  run: |
    cd cmd/vsphere-xcopy-volume-populator/e2e-tests
    go test -v ./tests/
  env:
    VSPHERE_HOST: ${{ secrets.VSPHERE_HOST }}
    VSPHERE_USERNAME: ${{ secrets.VSPHERE_USERNAME }}
    VSPHERE_PASSWORD: ${{ secrets.VSPHERE_PASSWORD }}
    # ... other environment variables
```

## Contributing

### Adding New OS Support

1.  Add OS configuration to `ansible/setup-vm.yml`:
```yaml
os_configs:
  linux-alma9:
    guest_id: "rhel9_64Guest"
```

2.  Update documentation in this README

### Test Development

1.  Follow Go testing conventions
2.  Use the common helper functions
3.  Add comprehensive logging
4.  Include cleanup procedures

## Support and Resources

### Documentation
-   [Forklift Migration Toolkit](https://forklift-docs.konveyor.io/)
-   [vSphere XCOPY Documentation](../README.md)

### Community
-   [Forklift GitHub Repository](https://github.com/kubev2v/forklift)
-   [Konveyor Community](https://www.konveyor.io/community/)

---

For questions or issues with the E2E testing framework, please [open an issue](https://github.com/kubev2v/forklift/issues) with the `e2e-tests` label.

## Remote Execution with SSH

For environments where `kubeconfig` is on a remote server (e.g., a bastion host), the test framework can execute `oc` commands on that server via SSH. This avoids copying sensitive credentials to your local machine.

### Configuration

To enable remote execution, set the following environment variables in your `config/test-config.env.local` file:

-   `SSH_HOST`: The hostname or IP address of the remote server (e.g., `bastion.example.com`).
-   `SSH_USER`: The username for the SSH connection (e.g., `your_user`).
-   `SSH_KEY_PATH`: (Optional) The absolute path to your SSH private key for authentication.
-   `REMOTE_KUBECONFIG_PATH` (Optional): The absolute path to the `kubeconfig` file on the remote server. If not specified, the default location (`~/.kube/config`) is used.

**Example Configuration:**
```bash
# --- Remote Execution via SSH (Optional) ---
export SSH_HOST="bastion.example.com"
export SSH_USER="your_user"
export SSH_KEY_PATH="/home/user/.ssh/id_rsa_bastion"
export REMOTE_KUBECONFIG_PATH="/home/your_user/secrets/ocp-prod.kubeconfig"
```

### Prerequisites

1.  **Passwordless SSH**: Your local machine must have key-based SSH authentication configured to access the remote server without a password.
2.  **`oc` CLI on Remote Host**: The `oc` binary must be installed and available in the remote user's `PATH`.
3.  **Valid Kubeconfig**: The `kubeconfig` file on the remote server must be correctly configured to access the target OpenShift cluster.

When remote execution is enabled, all `oc` commands are tunneled through SSH, while `ansible-playbook` and other test commands continue to run locally.
