# Debugging

This documentation provides an overview of the `Makefile` used to build and debug the `forklift-controller` project. The `Makefile` defines a set of default variables used for debugging, and includes targets to build and run the application using the `dlv` (Delve) debugger.

## Variables

### List of Variables for Controller

Important:
- **ROLE**: The role of the process being run. Defaults to `main` alternative `inventory`.
- **VIRT_V2V_IMAGE**: The image used for virtualization conversion. Defaults to `quay.io/virt-v2v/forklift-virt-v2v:latest`.
- **API_HOST**: The host address of the API for the inventory. Defaults to `localhost`.
- **API_PORT**: The port on which the API is served. Defaults to `443`.
- **DLV_PORT**: The port on which the Delve debugger will listen. Defaults to `5432`

Misc:
- **BLOCK_OVERHEAD**: The block storage overhead percentage. Defaults to `0`.
- **FILESYSTEM_OVERHEAD**: The filesystem overhead percentage. Defaults to `10`.
- **MAX_VM_INFLIGHT**: Maximum number of VMs that can be migrated in parallel. Defaults to `2`.
- **CLEANUP_RETRIES**: Number of retries during resource cleanup. Defaults to `10`.
- **SNAPSHOT_STATUS_CHECK_RATE_SECONDS**: The rate in seconds to check snapshot status. Defaults to `10`.
- **SNAPSHOT_REMOVAL_TIMEOUT_MINUTES**: The timeout in minutes for snapshot removal. Defaults to `120`.
- **VDDK_JOB_ACTIVE_DEADLINE**: The deadline in seconds for the VDDK job to remain active. Defaults to `300`.
- **PRECOPY_INTERVAL**: The interval in seconds for precopying data. Defaults to `60`.
- **OPENSHIFT**: Boolean indicating if the environment is OpenShift. Defaults to `true`.
- **METRICS_PORT**: The port on which metrics are exposed. Defaults to `8081`.
- **KUBEVIRT_CLIENT_GO_SCHEME_REGISTRATION_VERSION**: The version used for KubeVirt client registration. Defaults to `v1`.
- **FEATURE_VSPHERE_INCREMENTAL_BACKUP**: Boolean to enable the vSphere incremental backup feature. Defaults to `true`.
- **VSPHERE_OS_MAP**: The vSphere OS map. Defaults to `forklift-virt-customize`.
- **OVIRT_OS_MAP**: The oVirt OS map. Defaults to `forklift-ovirt-osmap`.
- **VIRT_CUSTOMIZE_MAP**: The Virt Customize map. Defaults to `forklift-virt-customize`.

## Build and Debug Targets

The `Makefile` contains two primary targets for building and debugging the application: `build-debug-%` and `debug-%`.

### `build-debug-%`

This target compiles the Go project for the specified command (e.g., `forklift-controller`) and outputs the binary to the `bin/` directory. It also includes the `-N -l` flags for disabling optimizations and inlining, making the binary more suitable for debugging.

Usage:
```bash
make build-debug-<command>
```

## Example Usage

To build and debug the `forklift-controller` command:

1. Run the build and debug process using the following command:
```bash
make debug-forklift-controller
```

2. Delve will start in headless mode, and you can connect to the debugger using a Go debugging client, pointing it to the specified `DLV_PORT` (default: `5432`).

To override any of the default variables (e.g., changing `DLV_PORT`), you can specify them on the command line:

```bash
make debug-forklift-controller DLV_PORT=5555 API_HOST="forklift-inventory-openshift-mtv.apps.yourcluster.local"
```