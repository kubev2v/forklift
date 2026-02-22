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
