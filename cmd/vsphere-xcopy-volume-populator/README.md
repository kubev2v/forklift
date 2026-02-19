# vSphere XCOPY Volume-Populator

## Table of Contents

- [Forklift Controller](#forklift-controller)
- [Populator Controller](#populator-controller)
- [VSphereXcopyVolumePopulator Resource](#vspherexcopyvolumepopoulator-resource)
- [vmkfstools-wrapper](#vmkfstools-wrapper)
- [Setup Copy Offload](#setup-copy-offload)
- [Supported Storage Providers](#supported-storage-providers)
- [Secret with Storage Provider Credentials](#secret-with-storage-provider-credentials)
  - [Hitachi Vantara](#hitachi-vantara)
  - [NetApp ONTAP](#netapp-ontap)
  - [HPE Primera/3PAR](#hpe-primera3par)
  - [Pure FlashArray](#pure-flasharray)
  - [Dell PowerMax](#dell-powermax)
  - [Dell PowerFlex](#dell-powerflex)
- [Limitations](#limitations)
- [Matching PVC with DataStores](#matching-pvc-with-datastores-to-deduce-copy-offload-support)
- [vSphere User Privileges](#vsphere-user-privileges)
- [Clone Methods: VIB vs SSH](#clone-methods-vib-vs-ssh)
  - [Configuring Clone Method](#configuring-clone-method)
  - [VIB Method Setup](#vib-method-setup)
  - [SSH Method Setup](#ssh-method-setup)
- [Troubleshooting](#troubleshooting)
  - [vSphere/ESXi](#vsphereesxi)
  - [SSH Method](#ssh-method)
  - [NetApp](#netapp)

## Forklift Controller
When the feature flag `feature_copy_offload` is true (on by default), the controller
consult the storagemaps offload plugin configuration, to decided if VM disk from
VMWare could be copied by the storage backend(offloaded) into the newly created PVC.
When the controller creates the PVC for the v2v pod it will also create
a volume popoulator resource of type VSphereXcopyVolumePopulator and set
the filed `dataSourceRef` in the PVC to reference it.

## Populator Controller
Added a new populator controller for the resource VSPhereXcopyVolumePopulator

## VSphereXcopyVolumePopulator Resource
A new populator implementation under cmd/vsphere-xcopy-volume-populator
is a cli program that runs in a container that is responsible to perform
XCOPY to efficiently copy data from a VMDK to the target PVC. See the
flow chart below.
The populator uses the storage API (configurable) to map the PVC to an ESX 
then uses Vsphere API to call functions on the ESX to perform the actual
XCOPY command (provided that VAAI and accelerations is enabled on that
ESX).

Example of the new resource and a PVC referencing it:
```

apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: my-pvc
  namespace: default
spec:
  resources:
    requests:
      storage: 100000Mi
  dataSourceRef:
    apiGroup: forklift.konveyor.io
    kind: VSphereXcopyVolumePopulator
    name: vm-1-xcopy-1
  storageClassName: sc-1  
  volumeMode: Block
  volumeName: pvc-6dff02f2-de63-40ab-a534-3bd5a7b47f82
---
apiVersion: forklift.konveyor.io/v1beta1
kind: VSphereXcopyVolumePopulator
metadata:
  name: vm-1-xcopy-1 
  namespace: default
spec:
  secretRef: vantara-secret 
  storageVendorProduct: vantara
  targetPVC: my-pvc 
  vmdkPath: '[my-vsphere-ds] vm-1/vm-1.vmdk'
```

## vmkfstools-wrapper
An ESXi CLI extension that exposes the vmkfstools clone operation to API interaction.
The folder vmkfstools-wrapper has a script to create a VIB to wrap the vmkfstools_wrapper.sh
to be a proxy to perform vmkfstools commands and more.
The VIB should be installed on every ESXi that is connected to the datastores which
are holds migratable VMs.
Alternative, that wrapper can be invoked using SSH. See [SSH Method](#ssh-method)

## Setup Copy Offload

1. Verify the feature flag is enabled (it is enabled by default):
   ```bash
   oc get forkliftcontrollers.forklift.konveyor.io forklift-controller -n openshift-mtv -o jsonpath='{.spec.feature_copy_offload}'
   ```

   If you need to explicitly enable or disable it:
   ```bash
   # To enable (if not already enabled)
   oc patch forkliftcontrollers.forklift.konveyor.io forklift-controller --type merge -p '{"spec": {"feature_copy_offload": "true"}}' -n openshift-mtv

   # To disable (if you want to opt-out)
   oc patch forkliftcontrollers.forklift.konveyor.io forklift-controller --type merge -p '{"spec": {"feature_copy_offload": "false"}}' -n openshift-mtv
   ```

2. Create a `StorageMap` according to [this section](#matching-pvc)

3. Create a plan and make sure to edit the mapping section and set the name to the `StorageMap` previously created.

   Here is how the mapping part looks in a `Plan`:
   ```yaml
   apiVersion: forklift.konveyor.io/v1beta1
   kind: Plan
   metadata:
     name: my-plan
   spec:
     map:
       storage:
         apiVersion: forklift.konveyor.io/v1beta1
         kind: StorageMap
         name: copy-offload  # <-- This points to the StorageMap configured previously
         namespace: openshift-mtv
   ```


## Supported Storage Providers

The `storageVendorProduct` field in the `StorageMap` identifies which storage product to use for copy offload. Below is a list of supported providers and the corresponding values to use.

| Vendor          | `storageVendorProduct` value | More Info |
| --------------- | ---------------------------- |:---:|
| Hitachi Vantara | `vantara`                    | [Link](#hitachi-vantara) |
| NetApp          | `ontap`                      | [Link](#netapp-ontap) |
| HPE             | `primera3par`                | |
| Pure Storage    | `pureFlashArray`             | [Link](#pure-flasharray) |
| Dell            | `powerflex`                  | [Link](#dell-powerflex) |
| Dell            | `powermax`                   | [Link](#dell-powermax) |
| Dell            | `powerstore`                 | |
| Infinidat       | `infinibox`                  | |
| IBM             | `flashsystem`                | [Link](#ibm-flashsystem) |

If a storage provider wants their storage to be supported, they need
to implement a go package named after their product, and mutate main
package so their specific code path is initialized.
See [internal/populator/storage.go](internal/populator/storage.go)

## Secret with Storage Provider Credentials

Create a secret where the migration provider is setup, usually openshift-mtv
and put the credentials of the storage system. All of the providers are required
to have a secret with the following fields:

| Key | Value | Mandatory | Default |
| --- | --- | --- | --- |
| STORAGE_HOSTNAME | ip/hostname | y | |
| STORAGE_USERNAME | string | y* | |
| STORAGE_PASSWORD | string | y* | |
| STORAGE_TOKEN | string | n** | |
| STORAGE_SKIP_SSL_VERIFICATION | true/false | n | false |

\* For most storage vendors, `STORAGE_USERNAME` and `STORAGE_PASSWORD` are required. Pure FlashArray is an exception - see below.

\*\* `STORAGE_TOKEN` is only supported by Pure FlashArray. When provided, it replaces the need for username/password authentication.

Provider-specific entries in the secret are documented below:

### Hitachi Vantara

See [README](internal/vantara/README.md)

### NetApp ONTAP

| Key | Value | Description |
| --- | --- | --- |
| ONTAP_SVM | string | the SVM to use in all the client interactions. Can be taken from trident.netapp.io/v1/TridentBackend.config.ontap_config.svm resource field. |

### HPE Primera/3PAR

**Important**: For HPE Primera/3PAR, the `STORAGE_HOSTNAME` must include the full URL with protocol and the 3PAR's Web Services API (WSAPI) port. Use the 3PAR command `cli% showwsapi` to determine the correct WSAPI port. 3PAR systems default to port `8080` for both HTTP and HTTPS connections, Primera and Alletra 9000/MP default to port `443` (SSL/HTTPS). Depending on configured certificates you may need to skip SSL verification.

**Example secret:**

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: hpe-3par-secret
  namespace: openshift-mtv
type: Opaque
stringData:
  STORAGE_HOSTNAME: "https://192.168.1.1:8080"
  STORAGE_USERNAME: "admin"
  STORAGE_PASSWORD: "your-password"
```

### Pure FlashArray

Pure FlashArray supports two mutually exclusive authentication methods:

#### Token-Based Authentication (Recommended)

Token-based authentication allows you to reuse the same credentials as your Pure CSI driver deployment.

| Key | Value | Description | Required |
| --- | --- | --- | --- |
| STORAGE_TOKEN | string | API token for Pure FlashArray authentication. Can be extracted from the Pure CSI driver secret. | Yes (if using token auth) |
| PURE_CLUSTER_PREFIX | string | Cluster prefix is set in the StorageCluster resource. Get it with  `printf "px_%s" $(oc get storagecluster -A -o=jsonpath='{.items[0].status.clusterUid}'| head -c 8)` | Yes |

**How to obtain the token from Pure CSI driver:**

The Pure CSI driver stores the API token in a secret, typically named `pure-provisioner-secret` or similar. To extract the token:

```bash
# Find the Pure CSI driver secret
oc get secrets -n <pure-csi-namespace> | grep pure

# Extract the API token (adjust secret name and key as needed)
oc get secret pure-provisioner-secret -n <pure-csi-namespace> -o jsonpath='{.data.PureAPIToken}' | base64 -d
```

**Example secret with token authentication:**

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: pure-flasharray-secret
  namespace: openshift-mtv
type: Opaque
stringData:
  STORAGE_HOSTNAME: "flasharray.example.com"
  STORAGE_TOKEN: "your-api-token-here"
  PURE_CLUSTER_PREFIX: "px_12345678"
```

#### Username/Password Authentication (Legacy)

If you prefer username/password authentication or don't have access to the API token:

| Key | Value | Description | Required |
| --- | --- | --- | --- |
| STORAGE_USERNAME | string | Username for Pure FlashArray management API | Yes (if using username/password auth) |
| STORAGE_PASSWORD | string | Password for Pure FlashArray management API | Yes (if using username/password auth) |
| PURE_CLUSTER_PREFIX | string | Cluster prefix is set in the StorageCluster resource. Get it with  `printf "px_%s" $(oc get storagecluster -A -o=jsonpath='{.items[0].status.clusterUid}'| head -c 8)` | Yes |

**Example secret with username/password authentication:**

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: pure-flasharray-secret
  namespace: openshift-mtv
type: Opaque
stringData:
  STORAGE_HOSTNAME: "flasharray.example.com"
  STORAGE_USERNAME: "pureuser"
  STORAGE_PASSWORD: "your-password-here"
  PURE_CLUSTER_PREFIX: "px_12345678"
```

**Important Notes:**
- Authentication methods are mutually exclusive: if `STORAGE_TOKEN` is provided, it will be used and `STORAGE_USERNAME`/`STORAGE_PASSWORD` are ignored
- If `STORAGE_TOKEN` is not provided, both `STORAGE_USERNAME` and `STORAGE_PASSWORD` must be set
- Token-based authentication is recommended as it allows credential reuse with the Pure CSI driver

### Dell PowerFlex

| Key | Value | Description |
| --- | --- | --- |
| POWERFLEX_SYSTEM_ID | string | the system id of the storage array. Can be taken from `vxflexos-config` from the `vxflexos` namespace or the openshift-operators namespace. |

### Dell PowerMax

| Key | Value | Description |
| --- | --- | --- |
| POWERMAX_SYMMETRIX_ID | string | the symmetrix id of the storage array. Can be taken from the ConfigMap under the 'powermax' namespace, which the CSI driver uses. |
| POWERMAX_PORT_GROUP_NAME | string | the port group to use for masking view creation. |

### IBM FlashSystem

Prior to using IBM FlashSystem with the volume populator (and with Virtual Machines on Red Hat OpenShift in general), **vdisk protection must be disabled** on the connected IBM FlashSystem—either globally or for the specific child pools in use.

If vdisk protection is enabled, the populator will exit with an error and log that protection must be off.

You may use one of the following options with careful consideration:

**Option 1 — Disable globally (entire system)**

```bash
chsystem -vdiskprotectionenabled no
```

**Option 2 — Disable for a specific pool**  
Replace `<pool_name_or_id>` with the name or ID of the child pool:

```bash
chmdiskgrp -vdiskprotectionenabled no <pool_name_or_id>
```

See [Volume protection](https://www.ibm.com/docs/en/flashsystem-c200/9.1.1?topic=volumes-volume-protection) in IBM FlashSystem documentation to read about both methods.

For the full requirement and VM configuration details, see the IBM documentation: [Configuring a Virtual Machine on Red Hat OpenShift](https://www.ibm.com/docs/en/stg-block-csi-driver/1.13.0?topic=configuration-configuring-virtual-machine-openshift).

## Host Lease Management

To prevent overloading ESXi hosts during concurrent migrations, the vsphere-xcopy-volume-populator uses a distributed lease mechanism based on Kubernetes Lease objects.
This ensures that heavy operations like storage rescans are serialized per ESXi host. For more details on its configuration, behavior, and monitoring, refer to the [Host Lease Management documentation](docs/copy-offload-lease-management.md).

## Limitations
- A migration plan cannot mix VDDK mappings with copy-offload mappings.
  Because the migration controller copies disks **either** through CDI volumes
  (VDDK) **or** through Volume Populators (copy-offload), all storage pairs
  in the plan must **either** include copy-offload details (secret + product)
  **or** none of them must; otherwise the plan will fail.

This volume populator implementation is specific for performing XCOPY from a source vmdk
descriptor disk file to a target PVC; this also works if the underlying disk is
vVol or RDM. The way it works is by performing the XCOPY using vmkfstools on the target ESXi.


<a id="matching-pvc"></a>
## Matching PVC with DataStores to deduce copy-offload support
For XCOPY to be supported a source VMDK disk backing LUN (iSCSI or FC) must co exist
with the target PVC (backed by a LUN) on the same storage array.
When a user is picking a VM to migrate to OpenShift there is no direct indication
of that info, other then if the current storage mapping supports it or not.
The plan should know if a specific disk should use the XCOPY populator by matching
the source vmdk data-store with the storage class. The supported pair of such
mapping is specified in the migration plan storagemap object.

To detect those conditions this heuristics is used:
- locate the LUN where the vmdk disk is on iSCSI or FC
- the PVC CSI provisioner creates LUNs on the same system as the VMFS where vmdks are

An example `StorageMap` for copy offload: 
```yaml
apiVersion: forklift.konveyor.io/v1beta1
kind: StorageMap
metadata:
  name: copy-offload
  namespace: openshift-mtv
spec:
  map:
  - destination:
      storageClass: YOUR_STORAGE_CLASS  #1)
    offloadPlugin:
      vsphereXcopyConfig:
        secretRef: SECRET_WITH_ONTAP_CREDS #2)
        storageVendorProduct: ontap #3)
    source:
      id: DATASTORE_ID #4) eg datastore-18601
  provider:
    destination:
      apiVersion: forklift.konveyor.io/v1beta1
      kind: Provider
      name: host
      namespace: openshift-mtv
      uid: YOUR_HOST_PROVIDER_ID #5)
    source:
      apiVersion: forklift.konveyor.io/v1beta1
      kind: Provider
      name: YOUR_VSPHERE_PROVIDER_NAME #6)
      namespace: openshift-mtv
      uid: YOUR_VSPHERE_PROVIDER_ID  #7)

```

1. the storage class for the target PVC of the VM
2. secret with the storage provider credentials 
3. string that identifies the storage product.
4. datastore ID as set by vSphere 
5. host provider ID
6. vsphere provider name
7. vsphere provider id

## vSphere User Privileges

The vSphere user requires a role with the following privileges (a role named `StorageOffloader` is recommended):

- Global
  - Settings
- Datastore
  - Browse datastore
  - Low level file operations
- Host → Configuration
  - Advanced settings
  - Query patch
  - Storage partition configuration

## Clone Methods: VIB vs SSH

The vsphere-xcopy-volume-populator supports two methods for executing vmkfstools clone operations on ESXi hosts:

- **VIB Method (Default)**: Uses a custom VIB (vSphere Installation Bundle) installed on ESXi hosts to expose vmkfstools operations via the vSphere API.
- **SSH Method**: Uses SSH to directly execute vmkfstools commands on ESXi hosts. This method is useful when VIB installation is not possible or preferred.

### Configuring Clone Method

The clone method is configured in the Provider settings using the `esxiCloneMethod` key:

```yaml
apiVersion: forklift.konveyor.io/v1beta1
kind: Provider
metadata:
  name: my-vsphere-provider
  namespace: openshift-mtv
spec:
  type: vsphere
  url: https://vcenter.example.com
  secret:
    name: vsphere-credentials
    namespace: openshift-mtv
  settings:
    esxiCloneMethod: "vib"  # or "ssh". The default is "vib"
```

### VIB Method Setup

The VIB (vSphere Installation Bundle) must be installed on every ESXi host that will participate in copy-offload operations.
This is only required when using the VIB clone method (`esxiCloneMethod: "vib"` in Provider settings, which is the default).

### Prerequisites

- Podman or Docker installed on your local machine
- SSH access to ESXi hosts (root user)
- SSH private key for ESXi authentication (can reuse the same key as SSH clone method)
- vSphere credentials (optional, for auto-discovery of ESXi hosts)

### Installation Using Container Image

The easiest way to install the VIB is using the `vib-installer` utility included in the container image.

**Auto-discover ESXi hosts from vSphere:**

```bash
podman run -it --rm \
  --entrypoint /bin/vib-installer \
  -v $HOME/.ssh/id_rsa:/tmp/esxi_key:Z \
  -e GOVMOMI_USERNAME='administrator@vsphere.local' \
  -e GOVMOMI_PASSWORD='your-password' \
  -e GOVMOMI_HOSTNAME='vcenter.example.com' \
  -e GOVMOMI_INSECURE='true' \
  $( oc get deploy -n openshift-mtv forklift-volume-populator-controller -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="VSPHERE_XCOPY_VOLUME_POPULATOR_IMAGE")].value}') \
  --ssh-key-file /tmp/esxi_key \
  --datacenter MyDatacenter
```

**Or specify ESXi hosts manually:**

```bash
podman run -it --rm \
  --entrypoint /bin/vib-installer \
  -v $HOME/.ssh/id_rsa:/tmp/esxi_key:Z \
  $( oc get deploy -n openshift-mtv forklift-volume-populator-controller -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="VSPHERE_XCOPY_VOLUME_POPULATOR_IMAGE")].value}') \
  --ssh-key-file /tmp/esxi_key \
  --esxi-hosts 'esxi1.example.com,esxi2.example.com,esxi3.example.com'
```

Run `vib-installer --help` for all available flags. Flags match the main populator naming conventions and support environment variables (e.g., `SSH_KEY_FILE`, `ESXI_HOSTS`, `GOVMOMI_USERNAME`).

**Note**: For alternative installation methods using Ansible, see [vmkfstools-wrapper/README.md](vmkfstools-wrapper/README.md)


### SSH Method Setup

When using the SSH method (`esxiCloneMethod: "ssh"`), SSH keys are **automatically generated** during the Provider reconciliation process. No manual key generation is required.

#### Automatic SSH Key Generation

SSH keys are automatically generated and stored when you create or update a vSphere Provider:

- **2048-bit RSA key pairs** are generated automatically
- Keys are stored in **separate Kubernetes secrets** in the Provider's namespace
- Keys are **automatically injected** into migration pods as needed

#### SSH Secret Names

SSH keys are stored in secrets with predictable names based on your vSphere Provider Name:

| Secret Type | Naming Pattern                             | Contains |
| --- |--------------------------------------------| --- |
| Private Key | `offload-ssh-keys-{provider-name}-private` | `private-key`: RSA private key in PEM format |
| Public Key | `offload-ssh-keys-{proider-name}-public`   | `public-key`: SSH public key in authorized_keys format |

**Example**: For a Provider with Name `vcenter-example`, the secrets would be:
- `offload-ssh-keys-vcenter-example-private`
- `offload-ssh-keys-vcenter-example-public`

#### Finding Your SSH Secrets

To find the SSH secrets for your vSphere Provider:

```bash
# List all SSH key secrets in the Provider namespace
oc get secrets -l app.kubernetes.io/component=ssh-keys -n openshift-mtv

# View a specific private key secret (replace with your actual secret name)
oc get secret offload-ssh-keys-vcenter-example-private -o yaml -n openshift-mtv

# View a specific public key secret (replace with your actual secret name)
oc get secret offload-ssh-keys-vcenter-example-public -o yaml -n openshift-mtv
```

#### Optional: Customizing SSH Keys

If you need to replace the auto-generated keys with your own:

```bash
# Generate your own key pair (if needed)
ssh-keygen -t rsa -b 4096 -f custom_esxi_key -N ""

# Replace the private key secret (use your actual secret name)
oc create secret generic offload-ssh-keys-vcenter-example-private \
  --from-file=private-key=custom_esxi_key \
  --dry-run=client -o yaml | oc replace -f - -n openshift-mtv

# Replace the public key secret (use your actual secret name)
oc create secret generic offload-ssh-keys-vcenter-example-public \
  --from-file=public-key=custom_esxi_key.pub \
  --dry-run=client -o yaml | oc replace -f - -n openshift-mtv
```

#### SSH Timeout Configuration

You can configure the SSH timeout by adding it to your Provider secret (the main storage credentials secret):

```bash
# Add SSH timeout to existing Provider secret
oc patch secret vsphere-credentials -p '{"data":{"SSH_TIMEOUT_SECONDS":"'$(echo -n "60" | base64)'"}}' -n openshift-mtv
```

#### ESXi Host Requirements

**SSH Service**

SSH must be enabled on ESXi hosts for the SSH method to work:

```bash
# Via ESXi shell:
vim-cmd hostsvc/enable_ssh
vim-cmd hostsvc/start_ssh

# Via vSphere Client:
# Host → Configure → Services → SSH Client → Start
```

**SSH Setup Requirements**

The SSH method requires:
1. **SSH service must be manually enabled** on ESXi hosts (see commands above)
2. **Manual SSH key deployment** - the system will provide instructions in logs if keys need to be installed
3. Once SSH and keys are configured, the system sets up secure command restrictions automatically

**Important**: SSH service enablement and initial key deployment must be done manually. The system will detect missing keys and provide step-by-step instructions in the logs.

#### Manual SSH Key Installation

You can prepare your hosts for the SSH method by manually adding the restricted SSH key to your ESXi hosts using the following steps. 

**Note**: When the ESXi is not ready, the SSHNotReady condition will show which ESXi hosts are not ready and give the exact instructions to follow
to fix the situation.

If you want to configure SSH keys prior to the first migration:

**Step 1: Extract the Public Key**

First, get the public key from the auto-generated secret:

```bash
# List SSH key secrets to find the right one
oc get secrets -l app.kubernetes.io/component=ssh-keys -n openshift-mtv

# Extract the public key (replace with your actual secret name)
oc get secret offload-ssh-keys-vcenter-example-public \
  -o jsonpath='{.data.public-key}' -n openshift-mtv | base64 -d > esxi_public_key.pub

# View the public key
cat esxi_public_key.pub
```

**Step 2: Prepare the Restricted Key Entry**

The system requires command restrictions for security. Create the restricted key entry:

```bash
# The public key needs to be prefixed with command restrictions
# The system now uses dynamic datastore routing - a single key works for all datastores
echo 'command="sh -c '\''DS=$(echo \"$SSH_ORIGINAL_COMMAND\" | sed -n \"s|.*/vmfs/volumes/\\([^/]*\\)/.*|\\1|p\"); exec sh /vmfs/volumes/$DS/secure-vmkfstools-wrapper.sh'\''",no-port-forwarding,no-agent-forwarding,no-X11-forwarding '$(cat esxi_public_key.pub) > restricted_key.pub

# View the final restricted key
cat restricted_key.pub
```

**Step 3: Install the Key on ESXi Host**

Connect to each ESXi host and install the key:

```bash
# If you have network access from your local machine to the ESXi host:
# Copy the restricted key directly (replace with your ESXi IP)
cat restricted_key.pub | ssh root@esxi-host-ip \
  'cat >> /etc/ssh/keys-root/authorized_keys'
```

**Step 4: Verify Installation**

Test the SSH key installation:

```bash
# Test SSH connection using the private key
# Extract private key from secret first
oc get secret offload-ssh-keys-vcenter-example-private \
  -o jsonpath='{.data.private-key}' -n openshift-mtv | base64 -d > esxi_private_key

# Set proper permissions
chmod 600 esxi_private_key

# Test connection
ssh -i esxi_private_key root@esxi-host-ip

# If successful, you should be connected with restricted commands
# Try a test command (should be restricted to the secure script)
```

**Step 5: Cleanup Local Files**

After installation, clean up the key files:

```bash
# Remove local key files for security
rm -f esxi_public_key.pub restricted_key.pub esxi_private_key
```

**Important Notes**

- The public key must include command restrictions for security
- The system uses dynamic datastore routing - the inline shell command automatically detects the datastore from the SSH command and routes to the correct script location
- A single SSH key works for all datastores - no need to hardcode datastore paths
- Each ESXi host in your migration environment needs the key installed
- SSH service must be enabled on all target ESXi hosts

#### Security Considerations

**SSH Key Security**
- Store SSH private keys securely in Kubernetes secrets
- Use separate key pairs for different environments
- Rotate keys periodically
- Consider using shorter-lived keys for enhanced security

**ESXi Access Control**
- Commands are restricted to vmkfstools operations only

#### SSH Method Advantages

- **No VIB Installation**: Doesn't require custom VIB deployment on ESXi hosts
- **Standard SSH**: Uses standard ESXi SSH service (no custom components)
- **Security**: Uses secure key-based authentication with command restrictions
- **Compatibility**: Works with any ESXi version that supports SSH
- **Flexibility**: Easier to troubleshoot and monitor SSH connections


## Troubleshooting

### vSphere/ESXi
- Sometimes remote ESXi execution can fail with SOAP error with no apparent root cause message
  Since VSphere is invoking some SOAP/Rest endpoints on the ESXi, those can fail because of
  standard error reasons and vanish after the next try. If the popoulator fails the migration
  can be restarted. We may want to restart/retry that populator or restart the migration.

- VIB issues
  If the vib is installed but the /etc/init.d/hostd did not restart then the vmkfstools namespace in esxcli is either not updated or doesn't exist. If it doesn't exist, it means that is the first time usage, probably right after the first use.
  The error returned by the remote esxcli invocation is:
  ```
  CLI Fault: The object or item referred to could not be found. <obj xmlns:xsd="http://www.w3.org/2001/XMLSchema" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns="urn:vim25" versionId="5.0" xsi:type="LocalizedMethodFault"><fault xsi:type="NotFound"></fault><localizedMessage>The object or item referred to could not be found.</localizedMessage></obj>
  ```

    resolution: ssh into the ESXi and run `/etc/init.d/hostd restart`. Wait for few seconds till the ESX renews the connection with vSphere.

- **Error**: Migration failures with multiple disks 

  **Cause**: ESXi has a configurable limit on the number of SCSI LUN IDs it will discover during storage rescans, controlled by the `Disk.MaxLUN` parameter (default: 1024). During copy-offload migrations, each VM disk creates a new PVC that provisions a new LUN on the storage array. When the total number of LUN IDs (existing LUNs + newly created LUNs for migration) approaches or exceeds the Disk.MaxLUN value, newly created LUNs with higher LUN IDs become invisible to the ESXi host. This causes vmkfstools clone operations to fail because the target LUN cannot be discovered during storage rescans.

  **Symptoms**:
  - Migration failures increase as the number of disks per VM increases
  - Inconsistent behavior - migrations sometimes succeed, sometimes fail for the same VMs
  - Failures are more common when:
    - Migrating VMs with 10+ disks
    - Running multiple concurrent migrations (e.g., 4 VMs with 5+ disks each)
    - Storage array already has many existing LUNs consuming lower LUN IDs

  **Why This Happens**:
  2. ESXi must perform storage rescans to discover newly created LUNs for XCOPY operations
  3. Disk.MaxLUN determines which LUN IDs the scan attempts to discover (default: LUN IDs 0-1023)
  4. If a new LUN gets assigned ID ≥ 1024, it's invisible to ESXi

  **Resolution**: Increase the `Disk.MaxLUN` value on each ESXi host to allow discovery of higher LUN IDs.

  **How to Configure**:

  1. Navigate to: **vSphere Web Client → Host → Configure → System → Advanced System Settings**
  2. Search for: `Disk.MaxLUN`
  3. Edit the value:
     - Current default: `1024` (LUN IDs 0-1023)
     - Recommended for copy-offload: `2048` or higher
     - Maximum supported: `16384` (LUN IDs 0-16383)
  4. Click **OK**
  5. Reboot the host (recommended for the change to take full effect)

  **Important Notes**:
  - Storage arrays vary in LUN ID allocation: NetApp ONTAP uses sequential from 0 (higher risk), Pure FlashArray starts at 254 descending (lower initial risk, jumps to 255+ after 254 LUNs), Dell PowerStore uses sequential with wraparound (moderate risk). Mature arrays with many existing LUNs are at higher risk.
  - This change should be applied to **all ESXi hosts** that will participate in copy-offload migrations
  - Higher values extend storage rescan times slightly
  - The value specifies the LUN ID *after* the last one you want to discover (e.g., to discover LUN IDs 0-2047, set Disk.MaxLUN to 2048)
  - **Caution**: If reducing Disk.MaxLUN from a previously higher value, ensure no LUNs with IDs above the new value are in use

  **References**:
  - [Broadcom KB 342823: Changing the Disk.MaxLUN parameter on ESXi Hosts](https://knowledge.broadcom.com/external/article/342823/changing-the-diskmaxlun-parameter-on-esx.html)
  - [Broadcom KB 323129: Troubleshooting LUN connectivity issues on ESXi hosts](https://knowledge.broadcom.com/external/article/323129/troubleshooting-lun-connectivity-issues.html)

### SSH Method

**Error**: `manual SSH key configuration required` or `failed to connect via SSH`
  
  **Causes and Solutions**:
  1. **SSH service disabled**: Manually enable SSH on the ESXi host using the commands in section 6
  2. **SSH keys not deployed**: Follow the manual instructions provided in the pod logs
  3. **Network connectivity**: Verify ESXi management network is accessible from migration pods
  4. **Timeout issues**: Increase `SSH_TIMEOUT_SECONDS` in the Provider secret (default: 30)
  
  **Verification steps**:
  ```bash
  # Check if SSH service is running on ESXi
  vim-cmd hostsvc/get_ssh_status
  
  # Manually test SSH connectivity from a migration pod
  ssh -i /path/to/private_key root@esxi-host-ip
  ```

- **Error**: `failed to start vmkfstools clone` or `task execution failed`
  
  **Causes and Solutions**:
  1. **Insufficient privileges**: Ensure vSphere user has required privileges (see vSphere User Privileges section)
  2. **Command restrictions**: The secure script may not have been deployed properly
  3. **Datastore access**: Verify the ESXi host has access to both source and target datastores
  
  **Debugging**:
  ```bash
  # Check available vmkfstools commands via SSH
  ssh root@esxi-host 'which vmkfstools'
  ssh root@esxi-host 'vmkfstools --help'
  ```

- **Error**: `SSH connection timeout` or `context deadline exceeded`
  
  **Solutions**:
  1. Increase `SSH_TIMEOUT_SECONDS` in the Provider secret
  2. Check network latency between migration pods and ESXi hosts
  3. Verify ESXi host is not overloaded
  4. Consider using dedicated migration network

- **Security warnings**: `Manual SSH key configuration required`
  
  This is expected behavior when SSH keys aren't configured yet. The system will:
  1. Detect that SSH key authentication isn't working
  2. Provide detailed instructions in the logs for manual key installation
  3. Once keys are installed, use secure key-based authentication with command restrictions
  4. Restrict commands to vmkfstools operations only

### NetApp

**Error**: `cannot derive SVM to use; please specify SVM in config file`

This is a configuration issue with Ontap and could be fixed by specifying a default SVM using vserver commands on the ontap server:

```bash
# show current config for an SVM
vserver show -vserver ${NAME_OF_SVM}
...
```

Try to set a mgmt interface for the SVM and put that hostname in the STORAGE_HOSTNAME

