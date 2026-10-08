package base

import (
	"context"
	"fmt"
	"net"
	"path"
	"sort"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"github.com/kubev2v/forklift/pkg/settings"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// QualifiedMultusNetworkName returns the Multus NetworkName for a NAD. It is
// qualified as "namespace/name" when the global setting forces qualified names
// or the NAD and target VM are in different namespaces, and the bare name when
// they share a namespace.
func QualifiedMultusNetworkName(nadNamespace, nadName, targetVMNamespace string) string {
	if settings.Settings.MultusNetworkNameAlwaysQualified || nadNamespace != targetVMNamespace {
		return path.Join(nadNamespace, nadName)
	}
	return nadName
}

// SortedIPv4First returns a copy of items with IPv4 addresses before IPv6.
// ipOf extracts the IP string from each element.
func SortedIPv4First[T any](items []T, ipOf func(T) string) []T {
	sorted := make([]T, len(items))
	copy(sorted, items)
	sort.SliceStable(sorted, func(i, j int) bool {
		ipI := net.ParseIP(ipOf(sorted[i]))
		ipJ := net.ParseIP(ipOf(sorted[j]))
		return ipI != nil && ipI.To4() != nil && (ipJ == nil || ipJ.To4() == nil)
	})
	return sorted
}

// HasMultipleIPsPerMAC returns true when any MAC address appears more than
// once in the given (mac, ip) pairs. Callers should pre-filter to only
// include manual-origin, non-link-local addresses.
func HasMultipleIPsPerMAC(macs []string) bool {
	count := make(map[string]int, len(macs))
	for _, mac := range macs {
		count[mac]++
	}
	for _, c := range count {
		if c > 1 {
			return true
		}
	}
	return false
}

type NICRef struct {
	MAC       string
	NetworkID string
}

// NICRefsFromKeys pairs each MAC with its resolved lookup key. macs and keys
// must be the same length and share NIC order. Returns an error if they differ.
func NICRefsFromKeys(macs []string, keys []string) ([]NICRef, error) {
	if len(macs) != len(keys) {
		return nil, fmt.Errorf("macs and keys length mismatch: %d != %d", len(macs), len(keys))
	}
	refs := make([]NICRef, len(macs))
	for i := range macs {
		refs[i] = NICRef{MAC: macs[i], NetworkID: keys[i]}
	}
	return refs, nil
}

// ResolveNICModes returns a MAC->mode map based on pre-resolved NetworkPairs and NADPool
// allocation. pairsBySource must be keyed by each NIC's resolved network key.
func ResolveNICModes(nics []NICRef, pairsBySource map[string][]api.NetworkPair, preserveStaticIPs bool) map[string]string {
	modes := map[string]string{}
	if pairsBySource == nil {
		for _, nic := range nics {
			if preserveStaticIPs {
				modes[nic.MAC] = string(api.NetworkIPModePreserve)
			} else {
				modes[nic.MAC] = string(api.NetworkIPModeNone)
			}
		}
		return modes
	}
	pool := NewNADPool()
	for _, nic := range nics {
		pairs := pairsBySource[nic.NetworkID]
		if len(pairs) == 0 {
			continue
		}
		pair, allocated := AllocateNetwork(pool, pairs)
		if !allocated {
			// Example: net-1 is mapped to [nad-a, nad-b] but the VM has 3 NICs on net-1.
			// The first two NICs claim nad-a and nad-b, the third NIC has no NAD left.
			// It won't appear in the mode map, so mapMacStaticIps includes it in static
			// IPs by default (backward compat). ValidateNetworkDuplicates warns about this.
			continue
		}
		mode := string(pair.NetworkIPMode)
		if pair.Destination.Type == Ignored {
			mode = string(api.NetworkIPModeNone)
		} else if mode == "" {
			if preserveStaticIPs {
				mode = string(api.NetworkIPModePreserve)
			} else {
				mode = string(api.NetworkIPModeNone)
			}
		}
		modes[nic.MAC] = mode
	}
	return modes
}

func HasPreserveMode(modes map[string]string) bool {
	for _, mode := range modes {
		if mode == string(api.NetworkIPModePreserve) {
			return true
		}
	}
	return false
}

func HasDHCPMode(modes map[string]string) bool {
	for _, mode := range modes {
		if mode == string(api.NetworkIPModeDHCP) {
			return true
		}
	}
	return false
}

// Network destination types.
const (
	Pod     = "pod"
	Multus  = "multus"
	Ignored = "ignored"
)

// FindAllMappingsForNICRef returns all NetworkPairs whose Source matches the given NIC ref.
func FindAllMappingsForNICRef(nicRef ref.Ref, networkMap *api.NetworkMap) []api.NetworkPair {
	if networkMap == nil {
		return nil
	}
	if nicRef.ID != "" {
		return networkMap.FindAllNetworks(nicRef.ID)
	}
	if nicRef.Type != "" {
		return networkMap.FindAllNetworksByType(nicRef.Type)
	}
	if nicRef.Name != "" {
		return networkMap.FindAllNetworksByNameAndNamespace(nicRef.Namespace, nicRef.Name)
	}
	return nil
}

// NADPool tracks NAD assignments within a single VM to ensure no NAD
// is used twice. Create one per VM via NewNADPool().
type NADPool struct {
	used map[string]bool
}

// NewNADPool creates a NADPool for tracking NAD assignments on one VM.
func NewNADPool() *NADPool {
	return &NADPool{
		used: make(map[string]bool),
	}
}

func nadKey(dest api.DestinationNetwork) string {
	if dest.Namespace == "" {
		return dest.Name
	}
	return dest.Namespace + "/" + dest.Name
}

// Allocate picks the first Multus NAD not yet used on this VM.
// pairsForSource are pre-filtered by source network (matched by ID or
// name), so every pair shares the same source. Only pass Multus pairs;
// for mixed-type routing use AllocateNetwork.
func (p *NADPool) Allocate(pairsForSource []api.NetworkPair) (api.NetworkPair, bool) {
	for _, pair := range pairsForSource {
		key := nadKey(pair.Destination)
		if !p.used[key] {
			p.used[key] = true
			return pair, true
		}
	}
	return api.NetworkPair{}, false
}

// AllocateNetwork picks a destination for one NIC from pre-filtered
// pairs (already matched to the NIC's source network by ID or name).
// Non-Multus destinations pass through directly; Multus destinations go
// through the NADPool for deduplication.
func AllocateNetwork(pool *NADPool, pairsForSource []api.NetworkPair) (api.NetworkPair, bool) {
	var nadPairs []api.NetworkPair
	for _, pair := range pairsForSource {
		// Only Multus NADs need deduplication via the pool.
		if pair.Destination.Type != Multus {
			return pair, true
		}
		nadPairs = append(nadPairs, pair)
	}
	return pool.Allocate(nadPairs)
}

// ValidateNetworkDuplicates checks whether more than one NIC resolves to the
// pod network or more than one NIC resolves to the same Multus NAD name.
// With NAD pool mapping, duplicate NADs are only flagged when the pool for a
// source network is exhausted (NIC count exceeds available NADs).
func ValidateNetworkDuplicates(nicRefs []ref.Ref, networkMap *api.NetworkMap) (foundNadDup bool, foundPodDup bool) {
	if networkMap == nil {
		return
	}

	pool := NewNADPool()
	podCount := 0

	for _, nicRef := range nicRefs {
		pairsForSource := FindAllMappingsForNICRef(nicRef, networkMap)
		pair, allocated := AllocateNetwork(pool, pairsForSource)
		if !allocated {
			if len(pairsForSource) > 0 {
				foundNadDup = true
			}
			continue
		}
		if pair.Destination.Type == Pod {
			podCount++
		}
	}

	foundPodDup = podCount > 1
	return
}

// ValidatePodNetworkDuplicates reports whether more than one NIC resolves
// to the pod network. Duplicate Multus NAD assignments are allowed.
func ValidatePodNetworkDuplicates(nicRefs []ref.Ref, networkMap *api.NetworkMap) bool {
	if networkMap == nil {
		return false
	}

	pool := NewNADPool()
	podCount := 0
	for _, nicRef := range nicRefs {
		pairs := FindAllMappingsForNICRef(nicRef, networkMap)
		if len(pairs) == 0 {
			continue
		}
		pair, ok := AllocateNetwork(pool, pairs)
		if !ok {
			continue
		}
		if pair.Destination.Type == Pod {
			podCount++
		}
	}
	return podCount > 1
}

// CollectPodNetworkMACs returns MAC addresses for all NICs mapped to Pod networks.
// Callers should skip this when the destination namespace uses UDN (l2bridge binding)
// because the masquerade-specific IPv6 config must not be applied to UDN interfaces.
func CollectPodNetworkMACs[N any](nicKeys []string, pairsBySourceKey map[string][]api.NetworkPair, nics []N, nicToRef func(N) (mac string)) []string {
	var podMacs []string
	pool := NewNADPool()

	for i, nic := range nics {
		if i >= len(nicKeys) {
			break
		}
		pairs := pairsBySourceKey[nicKeys[i]]
		if len(pairs) == 0 {
			continue
		}
		// Use the same allocation logic as mapNetworks
		pair, ok := AllocateNetwork(pool, pairs)
		if !ok {
			continue
		}
		if pair.Destination.Type == Pod {
			podMacs = append(podMacs, nicToRef(nic))
		}
	}
	return podMacs
}

// PodNetworkHasIPv6 checks the active OpenShift cluster Pod-network CIDRs.
func PodNetworkHasIPv6(
	ctx context.Context,
	client k8sclient.Client,
) (bool, error) {
	network := &unstructured.Unstructured{}
	network.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "config.openshift.io",
		Version: "v1",
		Kind:    "Network",
	})

	if err := client.Get(
		ctx, k8sclient.ObjectKey{Name: "cluster"}, network,
	); err != nil {
		return false, fmt.Errorf("get destination Network/cluster: %w", err)
	}

	entries, found, err := unstructured.NestedSlice(
		network.Object, "status", "clusterNetwork",
	)
	if err != nil {
		return false, fmt.Errorf("read status.clusterNetwork: %w", err)
	}
	if !found || len(entries) == 0 {
		return false, fmt.Errorf("network/cluster has no active Pod-network CIDRs")
	}

	hasIPv6 := false
	for i, entry := range entries {
		pool, ok := entry.(map[string]interface{})
		if !ok {
			return false, fmt.Errorf(
				"status.clusterNetwork[%d] is not an object", i,
			)
		}
		cidr, ok := pool["cidr"].(string)
		if !ok {
			return false, fmt.Errorf(
				"status.clusterNetwork[%d].cidr is missing or invalid", i,
			)
		}

		ip, _, err := net.ParseCIDR(cidr)
		if err != nil {
			return false, fmt.Errorf("parse Pod-network CIDR %q: %w", cidr, err)
		}
		if ip.To4() == nil {
			hasIPv6 = true
		}
	}
	return hasIPv6, nil
}
