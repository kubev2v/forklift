package resourceid

import (
	"fmt"
	"strings"
)

// Separator is the delimiter used in qualified IDs: {resourceGroup}--{name}.
const Separator = "--"

// Azure resource ID path segments.
const (
	subscriptionsKey  = "subscriptions"
	resourceGroupsKey = "resourceGroups"
	providersKey      = "providers"
)

// Well-known Azure resource providers and types.
const (
	ComputeProvider = "Microsoft.Compute"
	NetworkProvider = "Microsoft.Network"

	VMType           = "virtualMachines"
	DiskType         = "disks"
	NICType          = "networkInterfaces"
	VNetType         = "virtualNetworks"
	SubnetType       = "subnets"
	SnapshotType     = "snapshots"
	DiskAccessType   = "diskAccesses"
	ResourceGroupAll = ""
)

// Parsed holds the components of a parsed Azure resource ID.
type Parsed struct {
	Subscription  string
	ResourceGroup string
	Provider      string // e.g. "Microsoft.Compute"
	ResourceType  string // e.g. "virtualMachines"
	Name          string
	SubType       string // nested type, e.g. "subnets" in .../virtualNetworks/vnet1/subnets/sub1
	SubName       string // nested name
}

// Parse breaks a full Azure resource ID into its components.
//
//	/subscriptions/{sub}/resourceGroups/{rg}/providers/{provider}/{type}/{name}
//	/subscriptions/{sub}/resourceGroups/{rg}/providers/{provider}/{type}/{name}/{subType}/{subName}
func Parse(id string) (Parsed, error) {
	segments := splitAndClean(id)

	subIdx := indexOf(segments, subscriptionsKey)
	rgIdx := indexOf(segments, resourceGroupsKey)
	provIdx := indexOf(segments, providersKey)

	if subIdx < 0 || subIdx+1 >= len(segments) {
		return Parsed{}, fmt.Errorf("missing subscriptions segment in %q", id)
	}
	if rgIdx < 0 || rgIdx+1 >= len(segments) {
		return Parsed{}, fmt.Errorf("missing resourceGroups segment in %q", id)
	}
	if provIdx < 0 || provIdx+2 >= len(segments) {
		return Parsed{}, fmt.Errorf("missing providers segment in %q", id)
	}

	p := Parsed{
		Subscription:  segments[subIdx+1],
		ResourceGroup: segments[rgIdx+1],
		Provider:      segments[provIdx+1],
		ResourceType:  segments[provIdx+2],
	}

	if provIdx+3 < len(segments) {
		p.Name = segments[provIdx+3]
	}
	// Nested resource: .../virtualNetworks/vnet1/subnets/sub1
	if provIdx+5 < len(segments) {
		p.SubType = segments[provIdx+4]
		p.SubName = segments[provIdx+5]
	}

	return p, nil
}

// Build constructs a full Azure resource ID from components.
func Build(subscription, resourceGroup, provider, resourceType, name string) string {
	return fmt.Sprintf(
		"/subscriptions/%s/resourceGroups/%s/providers/%s/%s/%s",
		subscription, resourceGroup, provider, resourceType, name,
	)
}

// BuildNested constructs a nested Azure resource ID (e.g. a subnet within a VNet).
func BuildNested(subscription, resourceGroup, provider, parentType, parentName, childType, childName string) string {
	return fmt.Sprintf(
		"/subscriptions/%s/resourceGroups/%s/providers/%s/%s/%s/%s/%s",
		subscription, resourceGroup, provider, parentType, parentName, childType, childName,
	)
}

// MaxQualifiedIDLen is the Kubernetes resource-name length limit (DNS-1123).
const MaxQualifiedIDLen = 253

// QualifiedID converts a full Azure ARM resource ID into a Kubernetes-friendly
// short identifier in the format {resourceGroup}--{name}.
// Returns an error if the resource group contains the separator (making the
// result ambiguous) or if the result exceeds the Kubernetes name length limit.
func QualifiedID(armID string) (string, error) {
	p, err := Parse(armID)
	if err != nil {
		return "", fmt.Errorf("cannot build qualified ID: %w", err)
	}
	name := p.Name
	if p.SubName != "" {
		name = p.SubName
	}
	if strings.Contains(p.ResourceGroup, Separator) {
		return "", fmt.Errorf(
			"resource group %q contains separator %q, qualified ID would be ambiguous",
			p.ResourceGroup, Separator)
	}
	qid := p.ResourceGroup + Separator + name
	if len(qid) > MaxQualifiedIDLen {
		return "", fmt.Errorf(
			"qualified ID %q is %d characters, exceeds Kubernetes limit of %d",
			qid, len(qid), MaxQualifiedIDLen)
	}
	return qid, nil
}

// SplitQualifiedID breaks a qualified ID ({resourceGroup}--{name}) into its
// resource-group and name components. Splits on the first occurrence of "--"
// so resource names containing "--" are handled correctly.
func SplitQualifiedID(qualifiedID string) (resourceGroup, name string) {
	idx := strings.Index(qualifiedID, Separator)
	if idx < 0 {
		return "", qualifiedID
	}
	return qualifiedID[:idx], qualifiedID[idx+len(Separator):]
}

// Name extracts the resource name (last meaningful path segment) from a full ID.
func Name(id string) string {
	segments := splitAndClean(id)
	if len(segments) == 0 {
		return ""
	}
	return segments[len(segments)-1]
}

func splitAndClean(id string) []string {
	parts := strings.Split(id, "/")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func indexOf(segments []string, key string) int {
	for i, s := range segments {
		if strings.EqualFold(s, key) {
			return i
		}
	}
	return -1
}
