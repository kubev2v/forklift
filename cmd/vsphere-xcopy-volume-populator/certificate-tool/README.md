# Certificate Tool

A command-line utility to automate VM creation, Kubernetes populator setup, and end-to-end testing of volume copy (xcopy) workflows for various storage backends.

---

## Table of Contents

- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Configuration](#configuration)
- [Usage Examples](#usage-examples)




---

## Prerequisites

- Linux/macOS with Bash
- [Go](https://golang.org/) ≥1.18 in your `PATH`
- [`kubectl`](https://kubernetes.io/docs/tasks/tools/) in your `PATH` (or will be installed to `$(GOPATH)/bin`)
- [`yq`](https://github.com/mikefarah/yq) for parsing YAML
- **A running Kubernetes / OCP cluster** with:
    - Your **CSI driver** already installed
    - At least one functional **StorageClass** corresponding to that CSI driver
- **A prepared VM in vSphere**, with:
    - OS already installed and booted
    - Network connectivity and valid credentials
    - Accessible datastore path or name (so you can point `--vmdk-path` at its disk)
---

## Installation

1. **Clone the repo**
```bash
   git clone https://github.com/kubev2v/forklift.git
   cd cmd/vsphere-xcopy-volume-populator/certificate-tool 
```
2. **Build the binary**
```bash
   make build
   # outputs: ./certificate-tool
```

## Configuration

1. **Edit the static config:**
```bash
   vi assets/config/static_values.yaml
```

2. **Edit the .env file**
```bash
   vi assets/config/.env
```
3. **Ensure these environment variables are set (either in .env or your shell):**
```bash
   STORAGE_USERNAME
   STORAGE_PASSWORD
   STORAGE_HOSTNAME
   GOVMOMI_USERNAME
   GOVMOMI_PASSWORD
   GOVMOMI_HOSTNAME
```
The Makefile will load CONFIG_FILE (static_values.yaml) and .env automatically.

4. **Get a  vmdk file (you can use your own or run the following script)**
```bash
  podman pull quay.io/amitos/fedora-vmdk
  podman create --name fedora-vmdk-container quay.io/amitos/fedora-vmdk:latest
  podman cp fedora-vmdk-container:/disk.img ./fedora.vmdk
  podman rm fedora-vmdk-container

```
Update your Makefile variable
Set the VMDK_PATH variable in your Makefile (or environment) to point to this file. For example:
```bash
  VMDK_PATH = ./fedora.vmdk
```

### Password Configuration

**IMPORTANT**: Passwords are now read from files instead of being stored directly in the configuration file. This provides better security by allowing you to:
- Store password files in secure locations
- Use different file permissions for password files
- Avoid committing passwords to version control

The configuration file should specify paths to password files:

```yaml
# Storage configuration
storage-password-file: /path/to/storage-password.txt
storage-user: admin
storage-url: https://storage.example.com

# vSphere configuration  
vsphere-password-file: /path/to/vsphere-password.txt
vsphere-user: administrator@vsphere.local
vsphere-url: https://vcenter.example.com
```

**Password File Format**: Each password file should contain only the password as plain text. Only trailing line breaks (newlines and carriage returns) will be automatically trimmed - spaces and tabs are preserved as they may be part of the password.

Example password file:
```
mySecretPassword123
```

## Usage Example

**Preperation**

test-xcopy requires two parameters to run, path to the config file and the test plan file.
The Makefile uses the following default locations:
Config file: ./assets/config/static_values.yaml
Test plan: ./assets/manifests/examples/example-test-plan.yaml
Edit these files, filling out the missing parameters and defining the required test plan.
You are now ready to run the tests.

**End-to-end xcopy test**
   ```bash
    make test-xcopy
   ```

Runs the complete xcopy test workflow.

## Commands

- `prepare`: Sets up the Kubernetes environment (namespace, RBAC, secrets)
- `test-xcopy`: Creates test environment with PVC and CR instance
- `destroy-vms`: Destroys test VMs

## Security Best Practices

1. Store password files in secure locations with restricted permissions:
   ```bash
   chmod 600 /path/to/password-file.txt
   ```

2. Never commit password files to version control

3. Use different password files for different environments (dev, staging, production)

4. Consider using Kubernetes secrets or external secret management systems for production deployments

## Migration from Old Configuration

If you have an existing configuration with passwords directly in the YAML file, you need to:

1. Create separate files for each password
2. Update the configuration to use `-password-file` instead of `-password` fields
3. Remove the passwords from the YAML configuration file

Old format:
```yaml
storage-password: myStoragePassword
vsphere-password: myVSpherePassword
```

New format:
```yaml
storage-password-file: /secure/storage-password.txt
vsphere-password-file: /secure/vsphere-password.txt
```



