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

2. **Either edit the .env file**
```bash
   vi assets/config/.env
```
3. **Ensure these environment variables are set (either in .env or your shell):**
```bash
   STORAGE_USER
   STORAGE_PASSWORD
   STORAGE_URL
   VSPHERE_USER
   VSPHERE_PASSWORD
   VSPHERE_URL
```
The Makefile will load CONFIG_FILE (static_values.yaml) and .env automatically.

## Usage Example
**Prepare Your Cluster**
   ```bash
    make prepare
   ```
Creates the necessary OpenShift (OCP) objects to test the vSphere populator (mimicking some MTV behavior).

**Create VM**
   ```bash
    make create-vm
   ```
Creates a VM in your configured vSphere environment.
The VM remains powered off and no data is written to it.

**End-to-end xcopy test**
   ```bash
    make test-xcopy
   ```

Runs the complete xcopy test workflow.



