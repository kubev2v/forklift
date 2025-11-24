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
   cd cmd/vsphere-copy-offload-populator/certificate-tool
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

The certificate-tool supports flexible password management with three options:

#### Option 1: Password Files (Recommended for Production)
Specify paths to password files in your configuration:

```yaml
# Storage configuration
storage-password-file: #/path/to/storage-password.txt
storage-user: admin
storage-url: https://storage.example.com

# vSphere configuration  
vsphere-password-file: #/path/to/vsphere-password.txt
vsphere-user: administrator@vsphere.local
vsphere-url: https://vcenter.example.com
```

**Password File Format**: Each password file should contain only the password as plain text. Only trailing line breaks (newlines and carriage returns) will be automatically trimmed - spaces and tabs are preserved as they may be part of the password.

#### Option 2: Interactive Password Prompts (Recommended for Development)
If you leave the password file paths empty or unspecified, the tool will:

1. **Check for saved passwords**: Look for previously saved passwords in the `.passwords/` directory
2. **Prompt interactively**: Securely prompt you to enter passwords (input is hidden)
3. **Offer to save**: Ask if you want to save the password for future use

```yaml
# Leave password file paths empty for interactive prompts
storage-password-file: ""  # or omit entirely
vsphere-password-file: ""  # or omit entirely
```

When you run the tool, you'll see prompts like:
```
Enter storage password: [hidden input]
Would you like to save the storage password to .passwords for future use? (y/N): y
Password saved to .passwords/storage-password.txt

Enter vSphere password: [hidden input]
Would you like to save the vSphere password to .passwords for future use? (y/N): y
Password saved to .passwords/vsphere-password.txt
```

#### Option 3: Using Saved Passwords
Once passwords are saved in the `.passwords/` directory, the tool will detect and reuse them.

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
- `test-xcopy`: Runs the test according to the test plan yaml.

## Security Best Practices

1. **For Production**: Use explicit password files in secure locations with restricted permissions:
   ```bash
   chmod 600 /path/to/password-file.txt
   chown root:root /path/to/password-file.txt
   ```

2. **For Development**: Use interactive prompts with saved passwords in `.passwords/` directory:
   - The tool automatically sets secure permissions (0600 for files, 0700 for directory)
   - Add `.passwords/` to your `.gitignore` file

3. **Never commit password files to version control**:
   ```bash
   echo ".passwords/" >> .gitignore
   echo "*.password.txt" >> .gitignore
   ```

4. **Use different password storage methods for different environments**:
   - **Dev/Test**: Interactive prompts with local `.passwords/` storage
   - **CI/CD**: Environment variables or secure secret management
   - **Production**: Dedicated password files in secure locations

5. **Regularly rotate passwords** and update the corresponding files or saved passwords

New format:
```yaml
storage-password-file: .passwords/storage-password.txt
vsphere-password-file: .passwords/vsphere-password.txt
```



