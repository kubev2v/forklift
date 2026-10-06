package help

import "strings"

// Topic represents a built-in help topic that provides reference documentation
// for a domain-specific language or concept used across multiple commands.
type Topic struct {
	// Name is the topic identifier (e.g., "tsl", "karl")
	Name string `json:"name" yaml:"name"`
	// Short is a one-line description of the topic
	Short string `json:"short" yaml:"short"`
	// Content is the full reference text
	Content string `json:"content" yaml:"content"`
}

// topicRegistry holds all registered help topics.
var topicRegistry = []Topic{
	{
		Name:  "tsl",
		Short: "Tree Search Language (TSL) query syntax reference",
		Content: `Query Language (TSL) Syntax
==========================

TSL is used to filter inventory results with --query "where ..." and to select
VMs for migration plans with --vms "where ...".

Query Structure:
  [SELECT fields] WHERE condition [ORDER BY field [ASC|DESC]] [LIMIT n]
  For --vms flag: where <condition>

Operators:
  Comparison:     =  !=  <>  <  <=  >  >=
  Arithmetic:     +  -  *  /  %
  String match:   like (% wildcard), ilike (case-insensitive)
                  ~= (regex match), ~! (regex not match)
  Logical:        and, or, not
  Set/range:      in ['a','b'], not in ['a','b'], between X and Y
  Null checks:    is null, is not null

Array and Aggregate Functions:
  len(field)                    length of an array field
  sum(field[*].sub)             sum of numeric values in an array
  any(field[*].sub = 'value')   true if any element matches
  all(field[*].sub >= N)        true if all elements match

Array Access and SI Units:
  field[0]               index access (zero-based)
  field[*].sub           wildcard access across all elements
  field.sub              implicit traversal (same as field[*].sub)
  4Gi, 512Mi, 1Ti        SI unit suffixes (Ki, Mi, Gi, Ti, Pi)

Field Access:
  Dot notation for nested fields: parent.id, guest.distribution
  To discover all available fields for your provider, run:
    kubectl-mtv get inventory vm --provider <provider> --output json

VM Fields by Provider
---------------------

vSphere:
  Identity:    name, id, uuid, path, parent.id, parent.kind
  State:       powerState, connectionState
  Compute:     cpuCount, coresPerSocket, memoryMB
  Guest:       guestId, guestName, firmware, isTemplate
  Network:     ipAddress, hostName, host
  Storage:     storageUsed
  Security:    secureBoot, tpmEnabled, changeTrackingEnabled
  Disks:       len(disks), disks[*].capacity, disks[*].datastore.id,
               disks[*].datastore.name, disks[*].file, disks[*].shared
  NICs:        len(nics), nics[*].mac, nics[*].network.id
  Networks:    len(networks), networks[*].id, networks[*].kind
  Concerns:    len(concerns), concerns[*].category, concerns[*].assessment,
               concerns[*].label

oVirt / RHV:
  Identity:    name, id, path, cluster, host
  State:       status (up, down, ...)
  Compute:     cpuSockets, cpuCores, cpuThreads, memory (bytes)
  Guest:       osType, guestName, guest.distribution, guest.fullVersion
  Config:      haEnabled, stateless, placementPolicyAffinity, display
  Disks:       len(diskAttachments), diskAttachments[*].disk,
               diskAttachments[*].interface
  NICs:        len(nics), nics[*].name, nics[*].mac, nics[*].interface,
               nics[*].ipAddress, nics[*].profile
  Concerns:    len(concerns), concerns[*].category, concerns[*].assessment,
               concerns[*].label

OpenStack:
  Identity:    name, id, status
  Resources:   flavor.name, image.name, project.name
  Volumes:     len(attachedVolumes), attachedVolumes[*].ID

EC2 (PascalCase):
  Identity:    name, InstanceType, State.Name, PlatformDetails
  Placement:   Placement.AvailabilityZone
  Network:     PublicIpAddress, PrivateIpAddress, VpcId, SubnetId
  Snapshots:   State, VolumeId, VolumeSize, sizeHuman, Encrypted, Progress

Computed Fields (added by kubectl-mtv, available for all providers):
  criticalConcerns   count of critical migration concerns
  warningConcerns    count of warning migration concerns
  infoConcerns       count of informational migration concerns
  concernsHuman      human-readable concern summary
  memoryGB           memory in GB (converted from MB or bytes)
  storageUsedGB      storage used in GB
  diskCapacity       total disk capacity
  powerStateHuman    human-readable power state
  provider           provider name

Examples
--------

  Basic filtering:
    where name ~= 'prod-.*'
    where name like '%web%'
    where name in ['vm-01','vm-02','vm-03']

  By compute resources (vSphere):
    where powerState = 'poweredOn' and memoryMB > 4096
    where cpuCount > 4 and memoryMB > 8192
    where memoryMB between 2048 and 16384

  By compute resources (oVirt, memory in bytes):
    where status = 'up' and memory > 4Gi

  By guest OS:
    where guestId ~= 'rhel.*'                               (vSphere)
    where guest.distribution ~= 'Red Hat.*'                  (oVirt)

  By firmware and security:
    where firmware = 'efi'
    where isTemplate = false and secureBoot = true

  By disk and network configuration:
    where len(disks) > 1
    where len(disks) > 1 and cpuCount <= 8
    where len(nics) >= 2
    where any(disks[*].shared = true)

  Using the in operator (square brackets required):
    where guestId in ['rhel8_64Guest','rhel9_64Guest']
    where firmware in ['efi','bios']
    where guestId not in ['rhel8_64Guest','']

  Array element matching with any() (parentheses required for strings):
    where any(concerns[*].category = 'Critical')
    where any(concerns[*].category = 'Warning')
    where any(disks[*].datastore.id = 'datastore-12')

  Deep field access (dot notation, index, wildcard):
    where parent.kind = 'Folder'
    where disks[0].capacity > 50Gi
    where any(disks[*].datastore.id = 'datastore-17')
    where concerns[0].category = 'Critical'

  Aggregate functions (sum, all):
    where sum(disks[*].capacity) > 100Gi
    where all(disks[*].shared = false)

  Null checks:
    where ipAddress is null
    where ipAddress is not null

  Arithmetic expressions:
    where memoryMB / 1024 > 8

  Select with deep fields and functions:
    select name, disks[0].capacity, parent.kind where len(disks) > 1 limit 5
    select name, sum(disks[*].capacity) as totalDisk where len(disks) > 1 order by totalDisk desc limit 10

  By migration concerns:
    where criticalConcerns > 0
    where len(concerns) = 0

  By folder path:
    where path ~= '/Production/.*'
    where path like '/Datacenter/vm/Linux/%'

  Sorting and limiting:
    where memoryMB > 1024 order by memoryMB desc limit 10
    where powerState = 'poweredOn' order by name limit 50

  OpenStack:
    where status = 'ACTIVE' and flavor.name = 'm1.large'

  EC2:
    where State.Name = 'running' and InstanceType = 'm5.xlarge'
    where Placement.AvailabilityZone = 'us-east-1a'`,
	},
	{
		Name:  "karl",
		Short: "Kubernetes Affinity Rule Language (KARL) syntax reference",
		Content: `Affinity Syntax (KARL)
=====================

KARL is used by --target-affinity and --convertor-affinity flags in
create plan and patch plan to define Kubernetes pod affinity rules.

Syntax:
  RULE_TYPE pods(selector[,selector...]) on TOPOLOGY [weight=N]

Rule Types:
  REQUIRE  hard affinity     - pod MUST be placed with matching pods
  PREFER   soft affinity     - pod SHOULD be placed with matching pods (weight=1-100)
  AVOID    hard anti-affinity - pod MUST NOT be placed with matching pods
  REPEL    soft anti-affinity - pod SHOULD NOT be placed with matching pods (weight=1-100)

  REQUIRE and AVOID are strict: the scheduler will not place the pod if the
  rule cannot be satisfied. PREFER and REPEL are best-effort: the scheduler
  will try to honor them, with higher weight values taking priority.

Topology Keys:
  node     specific node (kubernetes.io/hostname)
  zone     availability zone (topology.kubernetes.io/zone)
  region   cloud region (topology.kubernetes.io/region)
  rack     rack location (topology.kubernetes.io/rack)

Label Selectors:
  Inside pods(...), use comma-separated selectors. All selectors are AND-ed.

  key=value            equality match
  key in [v1,v2,v3]   value in set
  key not in [v1,v2]  value not in set
  has key              label exists (any value)
  not has key          label does not exist

Examples
--------

  Basic co-location and anti-affinity:
    REQUIRE pods(app=database) on node
    AVOID pods(app=web) on node

  Soft affinity with weight:
    PREFER pods(app=cache) on zone weight=80
    REPEL pods(tier in [batch,worker]) on zone weight=50

  Multiple label selectors (AND-ed):
    REQUIRE pods(app=web,tier=frontend,has monitoring) on node

  Zone-aware placement:
    PREFER pods(app=api) on zone weight=100
    REPEL pods(app=api) on zone weight=50

  Using label sets:
    AVOID pods(env in [staging,dev]) on node
    REQUIRE pods(storage not in [ephemeral]) on node

  Convertor pod optimization (place near storage):
    --convertor-affinity "PREFER pods(app=storage-controller) on node weight=80"

  Target VM placement (co-locate with database):
    --target-affinity "REQUIRE pods(app=database) on node"

  Spread VMs across zones:
    --target-affinity "REPEL pods(app=myapp) on zone weight=50"`,
	},
	{
		Name:  "offload",
		Short: "Storage copy-offload (XCOPY) configuration reference",
		Content: `Storage Copy-Offload (XCOPY) Reference
======================================

Copy-offload delegates disk copying to the storage array instead of streaming
data through the cluster network, dramatically improving migration speed.

Prerequisites:
  kubectl mtv settings set --setting feature_copy_offload --value true

How It Works:
  1. StorageMap entry includes an OffloadPlugin with vendor + secret
  2. Controller creates a VSphereXcopyVolumePopulator per disk
  3. Populator pod uses VAAI/XCOPY to clone directly on the array
  4. Plans with mixed VDDK/offload pairs are rejected at validation

Supported Vendors
-----------------
  flashsystem    IBM FlashSystem (Spectrum Virtualize)
  vantara        Hitachi Vantara
  ontap          NetApp ONTAP (AFF/FAS)
  primera3par    HPE Primera / 3PAR / Alletra
  pureFlashArray Pure Storage FlashArray
  powerflex      Dell PowerFlex
  powermax       Dell PowerMax
  powerstore     Dell PowerStore
  infinibox      Infinidat InfiniBox

Secret Keys (Environment Variables)
------------------------------------
  Common (all vendors):
    STORAGE_HOSTNAME               storage array management API endpoint
    STORAGE_USERNAME               username (required unless using token)
    STORAGE_PASSWORD               password (required unless using token)
    STORAGE_SKIP_SSL_VERIFICATION  "true" to skip TLS (optional)

  Vendor-specific:
    STORAGE_TOKEN                  API token (pureFlashArray, alt to user/pass)
    PURE_CLUSTER_PREFIX            px_<8-char-cluster-uid> (pureFlashArray)
    POWERFLEX_SYSTEM_ID            system ID from vxflexos-config (powerflex)
    POWERMAX_SYMMETRIX_ID          Symmetrix array ID (powermax)
    POWERMAX_PORT_GROUP_NAME       port group for masking view (powermax)
    ONTAP_SVM                      SVM name (ontap)
    STORAGE_HTTP_TIMEOUT_SECONDS   HTTP timeout in seconds (optional)

Clone Methods
-------------
  vib (default)   Custom VIB on ESXi hosts, no SSH needed
  ssh             SSH to ESXi hosts with restricted commands

  Configure on provider:
    kubectl mtv patch provider --name my-vsphere --esxi-clone-method ssh

Creating Offload Credentials (Inline)
--------------------------------------
  kubectl mtv create mapping storage --name my-offload \
    --source my-vsphere --target host \
    --storage-pairs "datastore1:sc-block;offloadPlugin=vsphere;offloadVendor=flashsystem" \
    --offload-storage-username "admin" \
    --offload-storage-password "secret" \
    --offload-storage-endpoint "https://flashsystem.example.com:7443"

  Or reference an existing secret:
    --default-offload-secret my-storage-secret

  Additional inline flags:
    --offload-cacert @/path/to/ca.pem
    --offload-insecure-skip-tls

Examples
--------

  Basic FlashSystem mapping:
    kubectl mtv create mapping storage --name ibm-offload \
      --source vsphere-prod --target host \
      --storage-pairs "ibm-ds:flashsystem-sc;offloadPlugin=vsphere;offloadVendor=flashsystem" \
      --offload-storage-username admin \
      --offload-storage-password "$STORAGE_PASS" \
      --offload-storage-endpoint "https://flashsystem.company.com:7443"

  NetApp ONTAP:
    kubectl mtv create mapping storage --name ontap-offload \
      --source vsphere-prod --target host \
      --storage-pairs "netapp-nfs:trident-nas;offloadPlugin=vsphere;offloadVendor=ontap" \
      --offload-storage-username ontap-admin \
      --offload-storage-password "$STORAGE_PASS" \
      --offload-storage-endpoint "https://ontap-cluster.company.com"

  Pure FlashArray (with existing secret):
    kubectl create secret generic pure-creds -n openshift-mtv \
      --from-literal=STORAGE_HOSTNAME="pure-array.company.com" \
      --from-literal=STORAGE_TOKEN="$PURE_API_TOKEN" \
      --from-literal=PURE_CLUSTER_PREFIX="px_a1b2c3d4"

    kubectl mtv create mapping storage --name pure-offload \
      --source vsphere-prod --target host \
      --storage-pairs "pure-vvol:pure-block;offloadPlugin=vsphere;offloadVendor=pureFlashArray" \
      --default-offload-secret pure-creds

  Plan with offload mapping:
    kubectl mtv create plan --name fast-migration \
      --source vsphere-prod \
      --storage-mapping ibm-offload \
      --vms "where name ~= 'prod-.*'"

  Dedicated migration hosts (optional, for performance isolation):
    kubectl mtv create mapping storage --name offload-dedicated \
      --source vsphere-prod --target host \
      --storage-pairs "ds1:sc1;offloadPlugin=vsphere;offloadVendor=ontap" \
      --default-offload-migration-hosts "host-10+host-11" \
      --default-offload-secret my-storage-secret

Important Constraints
---------------------
  - A plan cannot mix VDDK and offload storage pairs (all or none)
  - Source VMDK and target PVC must be on the SAME physical storage array
  - ESXi hosts must have VAAI enabled
  - Works with vVol, RDM, and VMFS-backed disks`,
	},
}

// GetTopic returns a copy of the topic with the given name, or nil if not found.
// The lookup is case-insensitive.
func GetTopic(name string) *Topic {
	lower := strings.ToLower(name)
	for _, t := range topicRegistry {
		if t.Name == lower {
			copy := t
			return &copy
		}
	}
	return nil
}

// ListTopics returns a copy of all available help topics.
func ListTopics() []Topic {
	result := make([]Topic, len(topicRegistry))
	copy(result, topicRegistry)
	return result
}
