# vSphere Copy Offload Populator

## Forklift Controller
When the feature flag `feature_copy_offload` is true (off by default), the controller
consult the storagemaps offload plugin configuration, to decided if VM disk from
VMWare could be copied by the storage backend(offloaded) into the newly created PVC.
When the controller creates the PVC for the v2v pod it will also create
a volume popoulator resource of type VSphereXcopyVolumePopulator and set
the filed `dataSourceRef` in the PVC to reference it.

## Populator Controller
Added a new populator controller for the resource VSPhereXcopyVolumePopulator

## VSphereXcopyVolumePopulator Resource
A new populator implementation under cmd/vsphere-copy-offload-populator
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
The folder vmkfstools-wrapper has a script to create a VIB to wrap the vmkfstools-wrapper.sh
to be a proxy perform vmkfstools commands and more.
The VIB should be installed on every ESXi that is connected to the datastores which
are holds migratable VMs.
See vmkfstools-wrapper/README.md for the installation of the tool using ansible

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
| IBM             | `flashsystem`                | |

If a storage provider wants their storage to be supported, they need
to implement a go package named after their product, and mutate main
package so their specific code path is initialized.
See [internal/populator/storage.go](internal/populator/storage.go)

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

# vSphere User Privileges

The vSphere user requires a role with the following privileges (a role named `StorageOffloader` is recommended):

* Global
  * Settings
* Datastore
  * Browse datastore
  * Low level file operations
* Host
   Configuration
     * Advanced settings
     * Query patch
     * Storage partition configuration

# Secret with storage provider credentials

Create a secret where the migration provider is setup, usually openshift-mtv
and put the credentials of the storage system. All of the provider are required
to have a secret with those required fields

| Key | Value | Mandatory | Default |
| --- | --- | --- | --- |
| STORAGE_HOSTNAME | ip/hostname | y | |
| STORAGE_USERNAME | string | y | |
| STORAGE_PASSWORD | string | y | |
| STORAGE_SKIP_SSL_VERIFICATION | true/false | n | false |

# Clone Methods: VIB vs SSH

The vsphere-copy-offload-populator supports two methods for executing vmkfstools clone operations on ESXi hosts:

## VIB Method (Default)
Uses a custom VIB (vSphere Installation Bundle) installed on ESXi hosts to expose vmkfstools operations via the vSphere API.

## SSH Method
Uses SSH to directly execute vmkfstools commands on ESXi hosts. This method is useful when VIB installation is not possible or preferred.

## Configuring Clone Method

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

# SSH Method Setup

When using the SSH method (`esxiCloneMethod: "ssh"`), SSH keys are **automatically generated** during the Provider reconciliation process. No manual key generation is required.

## 1. Automatic SSH Key Generation

SSH keys are automatically generated and stored when you create or update a vSphere Provider:

- **2048-bit RSA key pairs** are generated automatically
- Keys are stored in **separate Kubernetes secrets** in the Provider's namespace
- Keys are **automatically injected** into migration pods as needed

## 2. SSH Secret Names

SSH keys are stored in secrets with predictable names based on your vSphere Provider Name:

| Secret Type | Naming Pattern                             | Contains |
| --- |--------------------------------------------| --- |
| Private Key | `offload-ssh-keys-{provider-name}-private` | `private-key`: RSA private key in PEM format |
| Public Key | `offload-ssh-keys-{proider-name}-public`   | `public-key`: SSH public key in authorized_keys format |

**Example**: For a Provider with Name `vcenter-example`, the secrets would be:
- `offload-ssh-keys-vcenter-example-private`
- `offload-ssh-keys-vcenter-example-public`

## 3. Finding Your SSH Secrets

To find the SSH secrets for your vSphere Provider:

```bash
# List all SSH key secrets in the Provider namespace
oc get secrets -l app.kubernetes.io/component=ssh-keys -n openshift-mtv

# View a specific private key secret (replace with your actual secret name)
oc get secret offload-ssh-keys-vcenter-example-private -o yaml -n openshift-mtv

# View a specific public key secret (replace with your actual secret name)
oc get secret offload-ssh-keys-vcenter-example-public -o yaml -n openshift-mtv
```

## 4. Optional: Customizing SSH Keys

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

## 5. SSH Timeout Configuration

You can configure the SSH timeout by adding it to your Provider secret (the main storage credentials secret):

```bash
# Add SSH timeout to existing Provider secret
oc patch secret vsphere-credentials -p '{"data":{"SSH_TIMEOUT_SECONDS":"'$(echo -n "60" | base64)'"}}' -n openshift-mtv
```

## 6. ESXi Host Requirements

### SSH Service
SSH must be enabled on ESXi hosts for the SSH method to work:

```bash
# Via ESXi shell:
vim-cmd hostsvc/enable_ssh
vim-cmd hostsvc/start_ssh

# Via vSphere Client:
# Host → Configure → Services → SSH Client → Start
```

### SSH Setup Requirements
The SSH method requires:
1. **SSH service must be manually enabled** on ESXi hosts (see commands above)
2. **Manual SSH key deployment** - the system will provide instructions in logs if keys need to be installed
3. Once SSH and keys are configured, the system sets up secure command restrictions automatically

**Important**: SSH service enablement and initial key deployment must be done manually. The system will detect missing keys and provide step-by-step instructions in the logs.

## 7. Manual SSH Key Installation

You can prepare your hosts for the SSH method by manually adding the restricted SSH key to your ESXi hosts using the following steps. 

**Note**: A simpler approach is to let one migration fail and follow the instructions in the populator pod logs - they will have the exact key and datastore path filled in for you.

If you want to configure SSH keys prior to the first migration:

### Step 1: Extract the Public Key

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

### Step 2: Prepare the Restricted Key Entry

The system requires command restrictions for security. Create the restricted key entry:

```bash
# The public key needs to be prefixed with command restrictions
# The script path will be: /vmfs/volumes/{datastore-name}/secure-vmkfstools-wrapper.py
# Replace {datastore-name} with your actual datastore name
echo 'command="python /vmfs/volumes/datastore1/secure-vmkfstools-wrapper.py",no-port-forwarding,no-agent-forwarding,no-X11-forwarding '$(cat esxi_public_key.pub) > restricted_key.pub

# View the final restricted key
cat restricted_key.pub
```

### Step 3: Install the Key on ESXi Host

Connect to each ESXi host and install the key:

```bash
# SSH to the ESXi host as root
ssh root@esxi-host-ip

# Add the restricted public key to authorized_keys
# Copy the content from restricted_key.pub and paste it into the file
vi /etc/ssh/keys-root/authorized_keys
```

### Step 4: Alternative - One-Command Installation

If you have network access from your local machine to the ESXi host:

```bash
# Copy the restricted key directly (replace with your ESXi IP)
cat restricted_key.pub | ssh root@esxi-host-ip \
  'cat >> /etc/ssh/keys-root/authorized_keys'
```

### Step 5: Verify Installation

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

### Step 6: Cleanup Local Files

After installation, clean up the key files:

```bash
# Remove local key files for security
rm -f esxi_public_key.pub restricted_key.pub esxi_private_key
```

### Important Notes

- The public key must include command restrictions for security
- The command path in the restrictions must match the secure script path: `/vmfs/volumes/{datastore-name}/secure-vmkfstools-wrapper.py`
- Each ESXi host in your migration environment needs the key installed
- SSH service must be enabled on all target ESXi hosts

## 8. Security Considerations

### SSH Key Security
- Store SSH private keys securely in Kubernetes secrets
- Use separate key pairs for different environments
- Rotate keys periodically
- Consider using shorter-lived keys for enhanced security

### ESXi Access Control
- Commands are restricted to vmkfstools operations only

## 9. SSH Method Advantages

- **No VIB Installation**: Doesn't require custom VIB deployment on ESXi hosts
- **Standard SSH**: Uses standard ESXi SSH service (no custom components)
- **Security**: Uses secure key-based authentication with command restrictions
- **Compatibility**: Works with any ESXi version that supports SSH
- **Flexibility**: Easier to troubleshoot and monitor SSH connections

Provider specific entries in the secret shall be documented below:

## Hitachi Vantara
- see [README](internal/vantara/README.md)

## NetApp ONTAP

| Key | Value | Description |
| --- | --- | --- |
| ONTAP_SVM | string | the SVM to use in all the client interactions. Can be taken from trident.netapp.io/v1/TridentBackend.config.ontap_config.svm resource field. |


## Pure FlashArray

| Key | Value | Description |
| --- | --- | --- |
| PURE_CLUSTER_PREFIX | string | Cluster prefix is set in the StorageCluster resource. Get it with  `printf "px_%.8s" $(oc get storagecluster -A -o=jsonpath='{.items[?(@.spec.cloudStorage.provider=="pure")].status.clusterUid}')` |


## Dell PowerMax

| Key | Value | Description |
| --- | --- | --- |
| POWERMAX_SYMMETRIX_ID | string | the symmetrix id of the storage array. Can be taken from the ConfigMap under the 'powermax' namespace, which the CSI driver uses. |
| POWERMAX_PORT_GROUP_NAME | string | the port group to use for masking view creation. |


## Dell PowerFlex

| Key | Value | Description |
| --- | --- | --- |
| POWERFLEX_SYSTEM_ID | string | the system id of the storage array. Can be taken from `vxflexos-config` from the `vxflexos` namespace or the openshift-operators namespace. |


# Setup copy offload
- Set the feature flag
  `oc patch forkliftcontrollers.forklift.konveyor.io forklift-controller --type merge -p '{"spec": {"feature_copy_offload": "true"}}' -n openshift-mtv`
- Set the volume-populator image (should be unnecessary in 2.8.5)
  `oc set env -n openshift-mtv deployment forklift-volume-populator-controller --all VSPHERE_XCOPY_VOLUME_POPULATOR_IMAGE=quay.io/kubev2v/vsphere-copy-offload-populator`
- Create a `StorageMap` according to [this section](#matching-pvc)
- Create a plan and make sure to edit the mapping section and set the name to the `StorageMap` previously created
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

# Troubleshooting

## vSphere/ESXi
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

## SSH Method
- **Error**: `manual SSH key configuration required` or `failed to connect via SSH`
  
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

## NetApp
- Error `cannot derive SVM to use; please specify SVM in config file`
  This is a configuration issue with Ontap and could be fixed by specifying a default
  SVM using vserver commands on the ontap server:
  ```
  # show current config for an SVM
  vserver show -vserver ${NAME_OF_SVM}
  ...
  ```
  Try to set a mgmt interface for the SVM and put that hostname in the STORAGE_HOSTNAME

