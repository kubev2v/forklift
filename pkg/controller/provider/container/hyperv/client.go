package hyperv

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dustin/go-humanize"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	hvutil "github.com/kubev2v/forklift/pkg/controller/hyperv"
	types "github.com/kubev2v/forklift/pkg/controller/provider/model/hyperv/types"
	"github.com/kubev2v/forklift/pkg/lib/hyperv/driver"
	ps "github.com/kubev2v/forklift/pkg/lib/hyperv/powershell"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	core "k8s.io/api/core/v1"
)

var clientLog = logging.WithName("client|hyperv")

// Not found error.
type NotFound struct {
}

func (e *NotFound) Error() string {
	return "not found."
}

// Storage constants
const (
	StorageTypeSMB        = "SMB"
	StorageNamePrefixSMB  = "SMB: "
	StorageNameDefaultSMB = "hyperv-storage"
)

// longCommandTimeout is used for scripts that iterate all VMs on a node
// (combined list+details, batch detail collection). With 80-100 VMs per
// node, the default 60s is insufficient for WinRM to return the output.
const longCommandTimeout = 5 * time.Minute

const (
	VMGenerationGen1 = 1
	VMGenerationGen2 = 2
)

type securityInfo struct {
	TpmEnabled bool `json:"TpmEnabled"`
	SecureBoot bool `json:"SecureBoot"`
}

// batchVMDetail holds the merged result of BatchGetVMHardware and BatchGetVMGuest for one VM.
// JSON tags match PowerShell output format (PascalCase), not the model types.
type batchVMDetail struct {
	Security      securityInfo `json:"Security"`
	HasCheckpoint bool         `json:"HasCheckpoint"`
	Disks         []struct {
		Path           string `json:"Path"`
		Capacity       int64  `json:"Capacity"`
		RCTEnabled     bool   `json:"RCTEnabled"`
		ControllerType int    `json:"CT"`
		ControllerNum  int    `json:"CN"`
		ControllerLoc  int    `json:"CL"`
	} `json:"Disks"`
	NICs []struct {
		Name       string `json:"Name"`
		MACAddress string `json:"MAC"`
		SwitchName string `json:"Switch"`
		VlanId     int    `json:"Vlan"`
	} `json:"NICs"`
	GuestOS       string `json:"GuestOS"`
	GuestNetworks []struct {
		MAC     string   `json:"MAC"`
		IPs     []string `json:"IPs"`
		Subnets []string `json:"Subnets"`
		DHCP    bool     `json:"DHCP"`
		GW      []string `json:"GW"`
		DNS     []string `json:"DNS"`
	} `json:"GuestNetworks"`
}

// Cached cluster metadata, fetched once per collection cycle.
type clusterCache struct {
	cluster *driver.ClusterData
	nodes   []driver.ClusterNodeData
}

// Client talks directly to HyperV host via WinRM.
type Client struct {
	driver           driver.HyperVDriver
	Secret           *core.Secret
	Log              logging.LevelLogger
	provider         *api.Provider
	smbUrl           string
	smbMountPath     string
	smbWindowsPrefix string
	cache            *clusterCache
	vmCached         bool
	vmCache          []types.VM
	netCached        bool
	netCache         []types.Network
	// LightMode skips expensive per-disk Get-VHD calls during initial staging.
	// Disk capacity and RCT are populated on the first refresh cycle.
	LightMode bool
	// localInfoOnce guards lazy initialization of localInfo so that concurrent
	// goroutines (VM prefetch, Phase 2 adapters) don't race.
	localInfoOnce sync.Once
	localInfo     *driver.ComputerInfoData
	localInfoErr  error
	// vmPrefetch holds pre-fetched VM + detail data from a background goroutine.
	// Set by StartVMPrefetch, consumed by ListVMs.
	vmPrefetch     chan vmPrefetchResult
	vmPrefetchDone bool
	// netLocalPrefetch holds pre-fetched local network data from a background
	// goroutine. Started before Phase 1 so it overlaps with ClusterAdapter.
	netLocalPrefetch     chan netLocalPrefetchResult
	netLocalPrefetchDone bool
}

type netLocalPrefetchResult struct {
	networks []driver.Network
	err      error
}

// vmPrefetchResult holds the result of a background VM pre-fetch.
type vmPrefetchResult struct {
	results []nodeResult
	err     error
}

// Connect establishes a WinRM connection to the HyperV host using Secret credentials.
func (r *Client) Connect(provider *api.Provider) (err error) {
	if r.driver != nil {
		if alive, _ := r.driver.IsAlive(); alive {
			return nil
		}
		_ = r.driver.Close()
	}

	username, password := hvutil.HyperVCredentials(r.Secret)
	host := extractHostFromURL(provider.Spec.URL)
	port := hvutil.WinRMPort(provider.Spec.Settings)

	drv := driver.NewWinRMDriver(host, port, username, password, true, nil)
	if err = drv.Connect(); err != nil {
		return fmt.Errorf("WinRM connect failed: %w", err)
	}

	r.driver = drv
	r.provider = provider
	r.smbUrl = hvutil.SMBUrl(r.Secret)
	r.smbMountPath = hvutil.SMBMountPath

	if r.smbUrl != "" {
		if pErr := r.discoverSMBWindowsPrefix(); pErr != nil {
			if errors.Is(pErr, driver.ErrUnauthorized) {
				return fmt.Errorf("SMB discovery auth failed: %w", pErr)
			}
			r.Log.Info("SMB Windows prefix not yet discovered, will attempt on next reconnect")
		}
	}

	return nil
}

// getClusterCache returns cached cluster+node data, fetching on first call per cycle.
// Uses the combined GetClusterInfo call to save a WinRM round trip.
func (r *Client) getClusterCache() (*clusterCache, error) {
	if r.cache != nil {
		return r.cache, nil
	}
	info, err := r.driver.GetClusterInfo()
	if err != nil {
		// Fall back to separate calls if the combined script isn't supported.
		clusterData, cErr := r.driver.GetCluster()
		if cErr != nil {
			return nil, fmt.Errorf("GetCluster failed: %w", cErr)
		}
		nodesData, nErr := r.driver.GetClusterNodes()
		if nErr != nil {
			return nil, fmt.Errorf("GetClusterNodes failed: %w", nErr)
		}
		r.cache = &clusterCache{cluster: clusterData, nodes: nodesData}
		return r.cache, nil
	}
	r.cache = &clusterCache{cluster: &info.Cluster, nodes: info.Nodes}
	return r.cache, nil
}

// InvalidateCycleCache clears all per-cycle caches so the next call re-fetches.
func (r *Client) InvalidateCycleCache() {
	r.cache = nil
	r.localInfoOnce = sync.Once{}
	r.localInfo = nil
	r.localInfoErr = nil
	r.vmCached = false
	r.vmCache = nil
	r.netCached = false
	r.vmPrefetch = nil
	r.vmPrefetchDone = false
	r.netLocalPrefetch = nil
	r.netLocalPrefetchDone = false
	r.netCache = nil
}

// StartVMPrefetch kicks off the combined VM+details script on all cluster nodes
// in background goroutines. Call this after the cluster cache is populated
// (Phase 1) so it can overlap with Phase 2 adapters. The results are consumed
// by ListVMs when it runs in the DiskAdapter phase.
func (r *Client) StartVMPrefetch() {
	if r.vmPrefetchDone || r.vmPrefetch != nil {
		return
	}
	if r.provider == nil || !r.provider.IsHyperVCluster() || !r.LightMode {
		return
	}

	ch := make(chan vmPrefetchResult, 1)
	r.vmPrefetch = ch

	go func() {
		results, err := r.fetchAllNodesVMsAndDetails()
		ch <- vmPrefetchResult{results: results, err: err}
	}()
}

// PrewarmClusterCache fetches and caches the cluster identity + node list.
// Call this early so that subsequent phases can start background prefetches
// that depend on knowing the cluster topology.
func (r *Client) PrewarmClusterCache() (*clusterCache, error) {
	return r.getClusterCache()
}

// StartNetworkPrefetch kicks off ListAllNetworks (local switches) in the
// background. This doesn't need the cluster cache, so it can overlap with
// Phase 1's ClusterAdapter. The result is consumed by ListNetworks.
func (r *Client) StartNetworkPrefetch() {
	if r.netLocalPrefetchDone || r.netLocalPrefetch != nil || r.netCached {
		return
	}
	ch := make(chan netLocalPrefetchResult, 1)
	r.netLocalPrefetch = ch
	go func() {
		nets, err := r.driver.ListAllNetworks()
		ch <- netLocalPrefetchResult{networks: nets, err: err}
	}()
}

// ListCluster returns the cluster info when in cluster mode, nil for standalone.
func (r *Client) ListCluster() (*types.Cluster, error) {
	if r.provider == nil || !r.provider.IsHyperVCluster() {
		return nil, nil //nolint:nilnil
	}
	cc, err := r.getClusterCache()
	if err != nil {
		return nil, err
	}
	var nodeNames []string
	for _, n := range cc.nodes {
		nodeNames = append(nodeNames, n.Name)
	}
	return &types.Cluster{
		Name:   cc.cluster.Name,
		Domain: cc.cluster.Domain,
		Nodes:  nodeNames,
	}, nil
}

// ListHosts returns the cluster hosts when in cluster mode.
func (r *Client) ListHosts() ([]types.Host, error) {
	if r.provider == nil || !r.provider.IsHyperVCluster() {
		return nil, nil
	}
	cc, err := r.getClusterCache()
	if err != nil {
		return nil, err
	}

	type hostResult struct {
		index int
		info  *driver.ComputerInfoData
	}
	hosts := make([]types.Host, len(cc.nodes))
	ch := make(chan hostResult, len(cc.nodes))

	for i, n := range cc.nodes {
		hosts[i] = types.Host{
			ID:          n.Id,
			Name:        n.Name,
			State:       driver.ClusterNodeStateName(n.State),
			ClusterName: cc.cluster.Name,
		}
		go func(idx int, name string) {
			info, err := r.getNodeComputerInfo(name)
			if err != nil {
				r.Log.V(1).Info("Failed to get hardware info for node", "node", name, "error", err)
			}
			ch <- hostResult{index: idx, info: info}
		}(i, n.Name)
	}

	for range cc.nodes {
		res := <-ch
		if res.info != nil {
			hosts[res.index].CpuCount = res.info.NumberOfProcessors
			hosts[res.index].CpuCores = res.info.NumberOfLogicalProcessors
			hosts[res.index].MemoryMB = res.info.TotalVisibleMemoryKB / 1024
		}
	}
	return hosts, nil
}

// getLocalComputerInfo returns the entry-point host's ComputerInfo, cached per
// cycle. Safe for concurrent use — sync.Once ensures the WinRM call runs
// exactly once even when called from parallel goroutines.
func (r *Client) getLocalComputerInfo() (*driver.ComputerInfoData, error) {
	r.localInfoOnce.Do(func() {
		r.localInfo, r.localInfoErr = r.driver.GetComputerInfo()
	})
	return r.localInfo, r.localInfoErr
}

// getNodeComputerInfo fetches hardware info from a specific cluster node.
func (r *Client) getNodeComputerInfo(nodeName string) (*driver.ComputerInfoData, error) {
	stdout, err := r.driver.RunOnNode(ps.GetComputerInfo, nodeName)
	if err != nil {
		return nil, err
	}
	if stdout == "" {
		return nil, nil //nolint:nilnil
	}
	var info driver.ComputerInfoData
	if err := json.Unmarshal([]byte(stdout), &info); err != nil {
		return nil, fmt.Errorf("parse ComputerInfo: %w", err)
	}
	return &info, nil
}

// nodeResult holds the combined VM list + detail map from a single node.
type nodeResult struct {
	nodeName string
	vms      []driver.VMData
	details  map[string]*batchVMDetail
	err      error
}

// listAndEnrichClusterVMs assembles VM data from per-node results, applies
// owner-node enrichment, and maps disk/NIC details. If a pre-fetch was
// started with StartVMPrefetch, its results are consumed here. Otherwise
// the combined script is run on-demand.
func (r *Client) listAndEnrichClusterVMs(networks []types.Network) ([]types.VM, error) {
	var nodeResults []nodeResult

	if r.vmPrefetch != nil && !r.vmPrefetchDone {
		// Consume pre-fetched results (blocks until prefetch completes).
		pf := <-r.vmPrefetch
		r.vmPrefetchDone = true
		if pf.err != nil {
			return nil, pf.err
		}
		nodeResults = pf.results
	} else {
		// No pre-fetch available — run synchronously.
		results, err := r.fetchAllNodesVMsAndDetails()
		if err != nil {
			return nil, err
		}
		nodeResults = results
	}

	var allVMs []types.VM
	allDetails := make(map[string]*batchVMDetail)
	for _, res := range nodeResults {
		for j := range res.vms {
			res.vms[j].ComputerName = res.nodeName
			dom := &driver.WinRMDomain{VMDataPtr: &res.vms[j]}
			vm, err := r.getVMBaseFromDomain(dom)
			if err != nil {
				r.Log.Error(err, "Failed to process domain")
				continue
			}
			allVMs = append(allVMs, *vm)
		}
		for k, v := range res.details {
			allDetails[k] = v
		}
	}

	r.enrichVMsWithOwnerNode(allVMs)

	if len(allDetails) > 0 {
		allIdx := make([]int, len(allVMs))
		for i := range allVMs {
			allIdx[i] = i
		}
		r.applyBatchDetails(allVMs, allIdx, allDetails, networks)
	}

	return allVMs, nil
}

// fetchAllNodesVMsAndDetails runs the combined script on all cluster nodes
// concurrently and returns the per-node results. Used both by StartVMPrefetch
// (background) and listAndEnrichClusterVMs (synchronous fallback).
func (r *Client) fetchAllNodesVMsAndDetails() ([]nodeResult, error) {
	cc, err := r.getClusterCache()
	if err != nil {
		return nil, fmt.Errorf("cluster cache unavailable: %w", err)
	}

	localInfo, err := r.getLocalComputerInfo()
	if err != nil {
		return nil, fmt.Errorf("cannot determine local hostname: %w", err)
	}
	localName := localInfo.DNSHostName

	light := r.LightMode
	script := ps.ListVMsWithDetailsLight
	if !light {
		script = ps.ListAllVMs
	}

	ch := make(chan nodeResult, len(cc.nodes)+1)

	go func() {
		r.fetchNodeVMsAndDetails(ch, localName, "", script, light)
	}()

	remoteCount := 0
	for _, node := range cc.nodes {
		if strings.EqualFold(node.Name, localName) || node.State != driver.ClusterNodeStateUp {
			continue
		}
		remoteCount++
		go func(name string) {
			r.fetchNodeVMsAndDetails(ch, name, name, script, light)
		}(node.Name)
	}

	var results []nodeResult
	succeeded := 0
	for i := 0; i < 1+remoteCount; i++ {
		res := <-ch
		if res.err != nil {
			r.Log.Error(res.err, "Failed to list VMs on node, skipping", "node", res.nodeName)
			continue
		}
		succeeded++
		results = append(results, res)
	}
	if succeeded == 0 {
		return nil, fmt.Errorf("all cluster nodes failed to list VMs")
	}
	return results, nil
}

// fetchNodeVMsAndDetails runs the combined or list-only script on a node and
// sends the parsed result to the channel. If remoteName is empty, runs locally.
// The light parameter controls output parsing, captured by the caller to
// avoid reading r.LightMode from a concurrent goroutine.
func (r *Client) fetchNodeVMsAndDetails(ch chan<- nodeResult, nodeName, remoteName, script string, light bool) {
	var stdout string
	var err error
	if remoteName == "" {
		stdout, err = r.driver.ExecuteCommandWithTimeout(script, longCommandTimeout)
	} else {
		stdout, err = r.driver.RunOnNodeWithTimeout(script, remoteName, longCommandTimeout)
	}
	if err != nil {
		ch <- nodeResult{nodeName: nodeName, err: err}
		return
	}
	if stdout == "" {
		ch <- nodeResult{nodeName: nodeName}
		return
	}

	if light {
		// Combined output: {"VMs":[...],"Details":{...}}
		var combined driver.VMsWithDetailsData
		if err := json.Unmarshal([]byte(stdout), &combined); err != nil {
			ch <- nodeResult{nodeName: nodeName, err: fmt.Errorf("parse combined output: %w", err)}
			return
		}
		vms, err := driver.UnmarshalArrayOrSingle[driver.VMData](combined.VMs)
		if err != nil {
			ch <- nodeResult{nodeName: nodeName, err: fmt.Errorf("parse VMs: %w", err)}
			return
		}
		var details map[string]*batchVMDetail
		if len(combined.Details) > 0 {
			if err := json.Unmarshal(combined.Details, &details); err != nil {
				r.Log.V(1).Info("Failed to parse combined details, will enrich later", "node", nodeName, "error", err)
			}
		}
		ch <- nodeResult{nodeName: nodeName, vms: vms, details: details}
	} else {
		// List-only output: [...]
		vms, err := driver.UnmarshalArrayOrSingle[driver.VMData]([]byte(stdout))
		if err != nil {
			ch <- nodeResult{nodeName: nodeName, err: fmt.Errorf("parse VMs: %w", err)}
			return
		}
		ch <- nodeResult{nodeName: nodeName, vms: vms}
	}
}

// ListVMs collects all VMs from the HyperV host via WinRM.
// In cluster mode, VMs are collected from all nodes concurrently using a
// combined script (list + details in one WinRM call per node).
func (r *Client) ListVMs() ([]types.VM, error) {
	if r.vmCached {
		return r.vmCache, nil
	}

	networks, err := r.ListNetworks()
	if err != nil {
		return nil, err
	}

	isCluster := r.provider != nil && r.provider.IsHyperVCluster()

	var vms []types.VM
	if isCluster && r.LightMode {
		// Combined path: list + light details in one call per node
		vms, err = r.listAndEnrichClusterVMs(networks)
		if err != nil {
			return nil, err
		}
	} else if isCluster {
		// Full mode: separate list + enrich
		var domains []driver.Domain
		domains, err = r.listClusterDomainsParallel()
		if err != nil {
			return nil, err
		}
		for _, domain := range domains {
			vm, vmErr := r.getVMBaseFromDomain(domain)
			if vmErr != nil {
				r.Log.Error(vmErr, "Failed to process domain")
				_ = domain.Free()
				continue
			}
			vms = append(vms, *vm)
			_ = domain.Free()
		}
		r.enrichVMsWithOwnerNode(vms)
		r.enrichVMDetails(vms, networks)
	} else {
		var domains []driver.Domain
		domains, err = r.driver.ListAllDomains()
		if err != nil {
			return nil, err
		}
		for _, domain := range domains {
			vm, vmErr := r.getVMBaseFromDomain(domain)
			if vmErr != nil {
				r.Log.Error(vmErr, "Failed to process domain")
				_ = domain.Free()
				continue
			}
			vms = append(vms, *vm)
			_ = domain.Free()
		}
		r.enrichVMDetails(vms, networks)
	}

	r.validateDisksOnSMB(vms)

	r.vmCached = true
	r.vmCache = vms

	return vms, nil
}

// listClusterDomainsParallel runs Get-VM on every cluster node concurrently.
// Used in full (non-light) mode where details are collected separately.
func (r *Client) listClusterDomainsParallel() ([]driver.Domain, error) {
	cc, err := r.getClusterCache()
	if err != nil {
		return nil, fmt.Errorf("cluster cache unavailable for parallel VM list: %w", err)
	}

	localInfo, err := r.getLocalComputerInfo()
	if err != nil {
		return nil, fmt.Errorf("cannot determine local hostname: %w", err)
	}
	localName := localInfo.DNSHostName

	ch := make(chan nodeResult, len(cc.nodes)+1)

	go func() {
		r.fetchNodeVMsAndDetails(ch, localName, "", ps.ListAllVMs, false)
	}()

	remoteCount := 0
	for _, node := range cc.nodes {
		if strings.EqualFold(node.Name, localName) || node.State != driver.ClusterNodeStateUp {
			continue
		}
		remoteCount++
		go func(name string) {
			r.fetchNodeVMsAndDetails(ch, name, name, ps.ListAllVMs, false)
		}(node.Name)
	}

	var allDomains []driver.Domain
	succeeded := 0
	for i := 0; i < 1+remoteCount; i++ {
		res := <-ch
		if res.err != nil {
			r.Log.Error(res.err, "Failed to list VMs on node, skipping", "node", res.nodeName)
			continue
		}
		succeeded++
		for j := range res.vms {
			res.vms[j].ComputerName = res.nodeName
			allDomains = append(allDomains, &driver.WinRMDomain{VMDataPtr: &res.vms[j]})
		}
	}
	if succeeded == 0 {
		return nil, fmt.Errorf("all cluster nodes failed to list VMs")
	}
	return allDomains, nil
}

// enrichVMDetails populates VM security, checkpoints, disk capacity/RCT, guest OS,
// and guest networks using batch PowerShell (per-node in cluster mode, local otherwise).
// In cluster mode, per-node batch calls run concurrently (the WinRM client is safe
// for parallel use because each call opens its own shell over HTTP).
func (r *Client) enrichVMDetails(vms []types.VM, networks []types.Network) {
	if r.provider != nil && r.provider.IsHyperVCluster() {
		nodeVMs := make(map[string][]int)
		for i := range vms {
			node := vms[i].OwnerNode
			nodeVMs[node] = append(nodeVMs[node], i)
		}

		type nodeResult struct {
			node     string
			indices  []int
			batchMap map[string]*batchVMDetail
			err      error
		}
		results := make(chan nodeResult, len(nodeVMs))
		for node, indices := range nodeVMs {
			go func(n string, idx []int) {
				bm, err := r.collectBatchVMDetails(n)
				results <- nodeResult{node: n, indices: idx, batchMap: bm, err: err}
			}(node, indices)
		}
		for range nodeVMs {
			res := <-results
			if res.err != nil {
				r.Log.Error(res.err, "Batch detail collection failed for node, falling back to per-VM", "node", res.node)
				r.fallbackPerVMDetails(vms, res.indices, networks)
				continue
			}
			r.applyBatchDetails(vms, res.indices, res.batchMap, networks)
		}
	} else {
		allIndices := make([]int, len(vms))
		for i := range vms {
			allIndices[i] = i
		}
		batchMap, err := r.collectBatchVMDetails("")
		if err != nil {
			r.Log.Error(err, "Batch detail collection failed, falling back to per-VM")
			r.fallbackPerVMDetails(vms, allIndices, networks)
			return
		}
		r.applyBatchDetails(vms, allIndices, batchMap, networks)
	}
}

// fallbackPerVMDetails collects details individually for the given VM indices.
// Used when the batch script fails (e.g., older Windows versions).
func (r *Client) fallbackPerVMDetails(vms []types.VM, indices []int, networks []types.Network) {
	for _, idx := range indices {
		vm := &vms[idx]
		computerName := vm.OwnerNode

		// If disks/NICs weren't populated (cluster mode with getVMBaseFromDomain),
		// collect them per-VM as fallback.
		if len(vm.Disks) == 0 {
			vm.Disks = r.collectPerVMDisks(vm.Name, vm.UUID, computerName)
		}
		if len(vm.NICs) == 0 {
			vm.NICs = r.collectPerVMNICs(vm.Name, computerName, networks)
		}

		if vm.Firmware == "uefi" {
			si, err := r.collectSecurityInfo(vm.Name, computerName)
			if err != nil {
				r.Log.V(1).Info("Failed to collect security info", "vm", vm.Name, "error", err)
			} else {
				vm.TpmEnabled = si.TpmEnabled
				vm.SecureBoot = si.SecureBoot
			}
		}

		hasCheckpoint, err := r.collectHasCheckpoint(vm.Name, computerName)
		if err != nil {
			r.Log.V(1).Info("Failed to check for checkpoints", "vm", vm.Name, "error", err)
		} else {
			vm.HasCheckpoint = hasCheckpoint
		}

		for j := range vm.Disks {
			vm.Disks[j].Capacity = r.getDiskCapacity(vm.Disks[j].WindowsPath, computerName)
			vm.Disks[j].RCTEnabled = r.getDiskRCTEnabled(vm.Disks[j].WindowsPath, computerName)
		}

		if vm.PowerState == "On" {
			guestOS, err := r.collectGuestOS(vm.Name, computerName)
			if err != nil {
				r.Log.V(1).Info("Guest OS detection failed", "vm", vm.Name, "error", err)
			} else if guestOS != "" {
				vm.GuestOS = guestOS
			}

			guestNetworks, err := r.collectGuestNetworkConfig(vm.Name, vm.NICs, computerName)
			if err != nil {
				r.Log.Info("KVP data collection failed", "vm", vm.Name, "error", err)
			} else if len(guestNetworks) > 0 {
				vm.GuestNetworks = guestNetworks
			}
		}
	}
}

// enrichVMsWithOwnerNode maps cluster VM groups to VMs by name and sets OwnerNode.
func (r *Client) enrichVMsWithOwnerNode(vms []types.VM) {
	groups, err := r.driver.GetClusterVMGroups()
	if err != nil {
		r.Log.Error(err, "Failed to get cluster VM groups for OwnerNode enrichment")
		return
	}
	ownerMap := make(map[string]string, len(groups))
	for _, g := range groups {
		ownerMap[g.Name] = g.OwnerNode
	}
	for i := range vms {
		if owner, found := ownerMap[vms[i].Name]; found {
			vms[i].OwnerNode = owner
			vms[i].IsClusterRole = true
		}
	}
}

// collectBatchVMDetails runs batch PowerShell on the given node.
// In LightMode (initial staging), it uses BatchGetVMDetailsLight which skips
// the expensive Get-VHD per disk. The full script is used during refresh.
func (r *Client) collectBatchVMDetails(computerName string) (map[string]*batchVMDetail, error) {
	script := ps.BatchGetVMDetails
	if r.LightMode {
		script = ps.BatchGetVMDetailsLight
	}
	out, err := r.driver.RunOnNodeWithTimeout(script, computerName, longCommandTimeout)
	if err != nil {
		r.Log.V(1).Info("Merged batch script failed, trying split fallback", "node", computerName, "error", err)
		return r.collectBatchVMDetailsSplit(computerName)
	}
	out = strings.TrimSpace(out)
	result := make(map[string]*batchVMDetail)
	if out != "" && out != "{}" && out != "null" {
		if err := json.Unmarshal([]byte(out), &result); err != nil {
			r.Log.V(1).Info("Parse merged batch failed, trying split fallback", "node", computerName, "error", err)
			return r.collectBatchVMDetailsSplit(computerName)
		}
	}
	return result, nil
}

// collectBatchVMDetailsSplit is the legacy two-call path: hardware first, then guest.
// Used as a fallback if the merged script exceeds the host's WinRM command limit.
func (r *Client) collectBatchVMDetailsSplit(computerName string) (map[string]*batchVMDetail, error) {
	hwOut, err := r.driver.RunOnNodeWithTimeout(ps.BatchGetVMHardware, computerName, longCommandTimeout)
	if err != nil {
		return nil, fmt.Errorf("batch hardware details failed: %w", err)
	}
	hwOut = strings.TrimSpace(hwOut)
	result := make(map[string]*batchVMDetail)
	if hwOut != "" && hwOut != "{}" && hwOut != "null" {
		if err := json.Unmarshal([]byte(hwOut), &result); err != nil {
			return nil, fmt.Errorf("parse batch hardware details: %w", err)
		}
	}

	guestOut, err := r.driver.RunOnNodeWithTimeout(ps.BatchGetVMGuest, computerName, longCommandTimeout)
	if err != nil {
		r.Log.V(1).Info("Batch guest details failed, hardware details still usable", "node", computerName, "error", err)
		return result, nil
	}
	guestOut = strings.TrimSpace(guestOut)
	if guestOut == "" || guestOut == "{}" || guestOut == "null" {
		return result, nil
	}
	var guestMap map[string]*batchVMDetail
	if err := json.Unmarshal([]byte(guestOut), &guestMap); err != nil {
		r.Log.V(1).Info("Parse batch guest details failed", "node", computerName, "error", err)
		return result, nil
	}

	for vmName, guest := range guestMap {
		if hw, exists := result[vmName]; exists {
			hw.GuestOS = guest.GuestOS
			hw.GuestNetworks = guest.GuestNetworks
		} else {
			result[vmName] = guest
		}
	}
	return result, nil
}

// applyBatchDetails enriches the VMs at the given indices with details from the batch script result.
// Builds full Disk and NIC arrays from batch data when empty (normal path for both standalone and cluster).
// Falls back to enriching capacity/RCT on pre-populated disks if present.
func (r *Client) applyBatchDetails(vms []types.VM, indices []int, batchMap map[string]*batchVMDetail, networks []types.Network) {
	for _, i := range indices {
		detail, found := batchMap[vms[i].Name]
		if !found {
			continue
		}

		vms[i].TpmEnabled = detail.Security.TpmEnabled
		vms[i].SecureBoot = detail.Security.SecureBoot
		vms[i].HasCheckpoint = detail.HasCheckpoint

		if detail.GuestOS != "" {
			vms[i].GuestOS = detail.GuestOS
		}

		if len(vms[i].Disks) == 0 && len(detail.Disks) > 0 {
			// Build full disk array from batch data.
			for j, bd := range detail.Disks {
				if bd.Path == "" {
					continue
				}
				smbPath := r.mapWindowsPathToSMB(bd.Path, r.smbWindowsPrefix)
				format := "vhdx"
				if strings.HasSuffix(strings.ToLower(bd.Path), ".vhd") {
					format = "vhd"
				}
				vms[i].Disks = append(vms[i].Disks, types.Disk{
					ID:          fmt.Sprintf("%s-disk-%d", vms[i].UUID, j),
					WindowsPath: bd.Path,
					SMBPath:     smbPath,
					Capacity:    bd.Capacity,
					RCTEnabled:  bd.RCTEnabled,
					Format:      format,
				})
			}
		} else {
			// Enrich pre-populated disks with capacity/RCT (defensive fallback).
			for j := range vms[i].Disks {
				for _, bd := range detail.Disks {
					if strings.EqualFold(
						strings.ReplaceAll(vms[i].Disks[j].WindowsPath, "\\", "/"),
						strings.ReplaceAll(bd.Path, "\\", "/")) {
						vms[i].Disks[j].Capacity = bd.Capacity
						vms[i].Disks[j].RCTEnabled = bd.RCTEnabled
						break
					}
				}
			}
		}

		if len(vms[i].NICs) == 0 && len(detail.NICs) > 0 {
			// Build full NIC array from batch data.
			for j, nd := range detail.NICs {
				mac := formatMAC(nd.MACAddress)
				vms[i].NICs = append(vms[i].NICs, types.NIC{
					Name:        fmt.Sprintf("nic-%d", j),
					MAC:         mac,
					DeviceIndex: j,
					NetworkUUID: resolveNetworkUUID(nd.SwitchName, vms[i].OwnerNode, networks),
					NetworkName: nd.SwitchName,
					VlanId:      nd.VlanId,
				})
			}
		}

		if len(detail.GuestNetworks) > 0 {
			var cfgs []guestNetCfg
			for _, g := range detail.GuestNetworks {
				cfgs = append(cfgs, guestNetCfg(g))
			}
			vms[i].GuestNetworks = buildGuestNetworks(cfgs, vms[i].NICs)
		}
	}
}

type guestNetCfg struct {
	MAC     string   `json:"MAC"`
	IPs     []string `json:"IPs"`
	Subnets []string `json:"Subnets"`
	DHCP    bool     `json:"DHCP"`
	GW      []string `json:"GW"`
	DNS     []string `json:"DNS"`
}

// buildGuestNetworks converts raw PowerShell guest-network configs into typed GuestNetwork entries.
// Shared by both the batch-enrichment and per-VM fallback paths.
func buildGuestNetworks(cfgs []guestNetCfg, nics []types.NIC) []types.GuestNetwork {
	var guestNetworks []types.GuestNetwork
	for _, cfg := range cfgs {
		mac := formatMAC(cfg.MAC)
		deviceIndex := findNICDeviceIndex(mac, nics)
		origin := "Manual"
		if cfg.DHCP {
			origin = "Dhcp"
		}
		for k, ip := range cfg.IPs {
			parsedIP := net.ParseIP(ip)
			if parsedIP == nil {
				continue
			}
			isIPv4 := parsedIP.To4() != nil
			gateway := ""
			for _, gw := range cfg.GW {
				gwIP := net.ParseIP(gw)
				if gwIP == nil {
					continue
				}
				if (gwIP.To4() != nil) == isIPv4 {
					gateway = gw
					break
				}
			}
			var prefixLen int32
			if k < len(cfg.Subnets) {
				if isIPv4 {
					prefixLen = subnetToPrefixLength(cfg.Subnets[k])
				} else {
					prefixLen = parseIPv6PrefixLength(cfg.Subnets[k])
				}
			} else if isIPv4 {
				prefixLen = 24
			} else {
				prefixLen = 64
			}
			dns := filterDNSByFamily(cfg.DNS, isIPv4)
			guestNetworks = append(guestNetworks, types.GuestNetwork{
				MAC:          mac,
				IP:           ip,
				DeviceIndex:  deviceIndex,
				Origin:       origin,
				PrefixLength: prefixLen,
				DNS:          dns,
				Gateway:      gateway,
			})
		}
	}
	return guestNetworks
}

// validateDisksOnSMB calls the provider-server validation endpoint to verify
// that disk files mapped to SMB paths actually exist on the mount. Disks whose
// files are missing have their SMBPath cleared so the OPA validation policy
// (hyperv.disk.smb_path.missing) can flag them.
func (r *Client) validateDisksOnSMB(vms []types.VM) {
	if r.provider == nil || r.provider.Status.Service == nil {
		r.Log.V(1).Info("Skipping SMB disk validation: no provider service available")
		return
	}

	svc := r.provider.Status.Service
	baseURL := fmt.Sprintf("http://%s.%s.svc.cluster.local:8080", svc.Name, svc.Namespace)

	type diskRef struct {
		vmIdx   int
		diskIdx int
	}
	var allPaths []string
	pathToDiskRefs := make(map[string][]diskRef)
	for i := range vms {
		for j := range vms[i].Disks {
			p := vms[i].Disks[j].SMBPath
			if p == "" {
				continue
			}
			if _, seen := pathToDiskRefs[p]; !seen {
				allPaths = append(allPaths, p)
			}
			pathToDiskRefs[p] = append(pathToDiskRefs[p], diskRef{vmIdx: i, diskIdx: j})
		}
	}

	if len(allPaths) == 0 {
		return
	}

	body, err := json.Marshal(map[string][]string{"paths": allPaths})
	if err != nil {
		r.Log.Error(err, "Failed to marshal validate-disks request")
		return
	}

	client := &http.Client{Timeout: ValidationTimeout}
	resp, err := client.Post(baseURL+"/validate-disks", "application/json", bytes.NewReader(body))
	if err != nil {
		r.Log.Error(err, "Failed to call validate-disks endpoint", "url", baseURL)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		r.Log.Info("SMB disk validation unavailable, provider-server returned unexpected status",
			"status", resp.StatusCode, "url", baseURL+"/validate-disks")
		return
	}

	var result struct {
		Missing []string `json:"missing"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		r.Log.Error(err, "Failed to decode validate-disks response")
		return
	}

	missingSet := make(map[string]bool, len(result.Missing))
	for _, p := range result.Missing {
		missingSet[p] = true
	}

	for path, refs := range pathToDiskRefs {
		if !missingSet[path] {
			continue
		}
		for _, ref := range refs {
			r.Log.Info("Disk file not found on SMB mount, clearing SMBPath",
				"vm", vms[ref.vmIdx].Name,
				"windowsPath", vms[ref.vmIdx].Disks[ref.diskIdx].WindowsPath,
				"smbPath", path)
			vms[ref.vmIdx].Disks[ref.diskIdx].SMBPath = ""
		}
	}

	if len(result.Missing) > 0 {
		r.Log.Info("SMB disk validation found missing disks", "count", len(result.Missing))
	}
}

// ListNetworks collects all virtual switches from the Hyper-V host via WinRM.
// In cluster mode, switches are collected from all cluster nodes so that NICs
// on VMs running on remote nodes can be resolved to a known network UUID.
func (r *Client) ListNetworks() ([]types.Network, error) {
	if r.netCached {
		return r.netCache, nil
	}

	var netDomains []driver.Network
	var err error
	if r.netLocalPrefetch != nil && !r.netLocalPrefetchDone {
		pf := <-r.netLocalPrefetch
		r.netLocalPrefetchDone = true
		netDomains, err = pf.networks, pf.err
	} else {
		netDomains, err = r.driver.ListAllNetworks()
	}
	if err != nil {
		return nil, err
	}

	localName := ""
	if r.provider != nil && r.provider.IsHyperVCluster() {
		if info, err := r.getLocalComputerInfo(); err == nil {
			localName = info.DNSHostName
		}
	}

	seen := make(map[string]bool)
	var result []types.Network
	for _, n := range netDomains {
		uuid, err := n.GetUUIDString()
		if err != nil {
			r.Log.Error(err, "Failed to get network UUID")
			_ = n.Free()
			continue
		}
		name, err := n.GetName()
		if err != nil {
			r.Log.Error(err, "Failed to get network name", "uuid", uuid)
			_ = n.Free()
			continue
		}
		switchType, _ := n.GetSwitchType()

		seen[uuid] = true
		ownerNodes := []string{}
		if localName != "" {
			ownerNodes = []string{localName}
		}
		result = append(result, types.Network{
			UUID:       uuid,
			Name:       name,
			SwitchType: switchType,
			OwnerNodes: ownerNodes,
		})
		_ = n.Free()
	}

	if r.provider != nil && r.provider.IsHyperVCluster() {
		r.mergeRemoteNodeNetworks(&result, seen)
	}

	r.netCached = true
	r.netCache = result

	return result, nil
}

// mergeRemoteNodeNetworks queries Get-VMSwitch on each remote cluster node
// concurrently and merges the results. New switches are appended to result,
// switches whose UUID was already seen get the remote node appended to OwnerNodes.
func (r *Client) mergeRemoteNodeNetworks(result *[]types.Network, seen map[string]bool) {
	cc, err := r.getClusterCache()
	if err != nil {
		r.Log.Error(err, "Cannot collect remote node networks: cluster cache unavailable")
		return
	}

	localInfo, err := r.getLocalComputerInfo()
	if err != nil {
		r.Log.V(1).Info("Cannot determine local hostname for network dedup", "error", err)
		return
	}
	localName := strings.ToUpper(localInfo.DNSHostName)

	type nodeSwitches struct {
		nodeName string
		switches []driver.SwitchData
	}
	ch := make(chan nodeSwitches, len(cc.nodes))
	count := 0
	for _, node := range cc.nodes {
		if strings.EqualFold(node.Name, localName) || node.State != driver.ClusterNodeStateUp {
			continue
		}
		count++
		go func(name string) {
			stdout, err := r.driver.RunOnNode(ps.ListAllSwitches, name)
			if err != nil {
				r.Log.Info("Failed to collect switches from remote node", "node", name, "error", err)
				ch <- nodeSwitches{nodeName: name}
				return
			}
			stdout = strings.TrimSpace(stdout)
			if stdout == "" {
				ch <- nodeSwitches{nodeName: name}
				return
			}
			data, err := driver.UnmarshalArrayOrSingle[driver.SwitchData]([]byte(stdout))
			if err != nil {
				r.Log.Info("Failed to parse remote node switches", "node", name, "error", err)
				ch <- nodeSwitches{nodeName: name}
				return
			}
			ch <- nodeSwitches{nodeName: name, switches: data}
		}(node.Name)
	}

	for i := 0; i < count; i++ {
		ns := <-ch
		for _, sw := range ns.switches {
			if seen[sw.Id] {
				for j := range *result {
					if (*result)[j].UUID == sw.Id {
						(*result)[j].OwnerNodes = append((*result)[j].OwnerNodes, ns.nodeName)
						break
					}
				}
				continue
			}
			seen[sw.Id] = true
			*result = append(*result, types.Network{
				UUID:       sw.Id,
				Name:       sw.Name,
				SwitchType: mapSwitchType(sw.SwitchType),
				OwnerNodes: []string{ns.nodeName},
			})
			r.Log.Info("Discovered remote-only switch", "node", ns.nodeName, "name", sw.Name, "id", sw.Id)
		}
	}
}

// ListStorages returns the SMB storage record from the HyperV host via WinRM.
func (r *Client) ListStorages() ([]types.Storage, error) {
	if r.smbUrl == "" {
		return nil, nil
	}

	shareName := extractShareName(r.smbUrl)
	if shareName == "" {
		shareName = StorageNameDefaultSMB
	}

	storage := types.Storage{
		ID:   hvutil.StorageIDDefault,
		Name: StorageNamePrefixSMB + shareName,
		Type: StorageTypeSMB,
		Path: r.smbWindowsPrefix,
	}

	if r.smbWindowsPrefix != "" {
		capacity, free := r.getStorageCapacity(r.smbWindowsPrefix)
		storage.Capacity = capacity
		storage.Free = free
	}

	r.Log.Info("Extracted storage",
		"name", storage.Name,
		"path", r.smbWindowsPrefix,
		"capacity", humanize.IBytes(uint64(storage.Capacity)),
		"free", humanize.IBytes(uint64(storage.Free)),
		"smbUrl", r.smbUrl)

	return []types.Storage{storage}, nil
}

// vhdCapacity holds the Get-VHD output for a single disk path.
type vhdCapacity struct {
	Size       int64 `json:"S"`
	RCTEnabled bool  `json:"R"`
}

// EnrichDiskCapacity runs Get-VHD in parallel across cluster nodes and
// updates cached VMs' Capacity and RCTEnabled in place. Standalone VMs
// (OwnerNode == "") are queried on the entry-point host ("__local__").
func (r *Client) EnrichDiskCapacity() error {
	if !r.vmCached || len(r.vmCache) == 0 {
		return nil
	}

	// Build a per-node list of VMs for disk enrichment.
	nodeVMs := make(map[string][]int) // node → indices into vmCache
	for i, vm := range r.vmCache {
		node := vm.OwnerNode
		if node == "" {
			node = "__local__"
		}
		nodeVMs[node] = append(nodeVMs[node], i)
	}

	type nodeCapResult struct {
		node string
		caps map[string]vhdCapacity
	}
	ch := make(chan nodeCapResult, len(nodeVMs))
	for node := range nodeVMs {
		go func(n string) {
			var out string
			var err error
			if n == "__local__" {
				out, err = r.driver.ExecuteCommandWithTimeout(ps.BatchGetVHDCapacity, longCommandTimeout)
			} else {
				out, err = r.driver.RunOnNodeWithTimeout(ps.BatchGetVHDCapacity, n, longCommandTimeout)
			}
			if err != nil {
				r.Log.Info("VHD capacity enrichment failed, will be filled on next refresh",
					"node", n, "error", err)
				ch <- nodeCapResult{node: n}
				return
			}
			out = strings.TrimSpace(out)
			if out == "" || out == "{}" || out == "null" {
				ch <- nodeCapResult{node: n}
				return
			}
			var caps map[string]vhdCapacity
			if err := json.Unmarshal([]byte(out), &caps); err != nil {
				r.Log.Info("Parse VHD capacity failed", "node", n, "error", err)
				ch <- nodeCapResult{node: n}
				return
			}
			ch <- nodeCapResult{node: n, caps: caps}
		}(node)
	}

	for range nodeVMs {
		res := <-ch
		if res.caps == nil {
			continue
		}
		for _, idx := range nodeVMs[res.node] {
			for d := range r.vmCache[idx].Disks {
				if c, ok := res.caps[r.vmCache[idx].Disks[d].WindowsPath]; ok {
					r.vmCache[idx].Disks[d].Capacity = c.Size
					r.vmCache[idx].Disks[d].RCTEnabled = c.RCTEnabled
				}
			}
		}
	}
	return nil
}

// ListDisks returns all disks from all VMs.
func (r *Client) ListDisks() ([]types.Disk, error) {
	vms, err := r.ListVMs()
	if err != nil {
		return nil, err
	}
	var disks []types.Disk
	for _, vm := range vms {
		disks = append(disks, vm.Disks...)
	}
	return disks, nil
}

// getVMBaseFromDomain extracts base VM metadata without disk/NIC WinRM calls.
// Used in cluster mode where disks and NICs are collected in batch per node.
func (r *Client) getVMBaseFromDomain(domain driver.Domain) (*types.VM, error) {
	uuid, err := domain.GetUUIDString()
	if err != nil {
		return nil, err
	}

	name, err := domain.GetName()
	if err != nil {
		return nil, err
	}

	state, _, err := domain.GetState()
	if err != nil {
		return nil, err
	}

	info, err := domain.GetInfo()
	if err != nil {
		return nil, err
	}

	generation, err := domain.GetGeneration()
	if err != nil {
		r.Log.V(1).Info("Failed to get VM generation, defaulting to BIOS", "vm", name, "error", err)
	}
	firmware := "bios"
	if generation == VMGenerationGen2 {
		firmware = "uefi"
	}

	return &types.VM{
		UUID:       uuid,
		Name:       name,
		PowerState: mapPowerState(state),
		CpuCount:   int(info.NrVirtCpu),
		MemoryMB:   int64(info.Memory / 1024),
		Firmware:   firmware,
		OwnerNode:  domain.GetComputerName(),
	}, nil
}

// collectPerVMDisks fetches disk info for a single VM on a specific node.
// Used as fallback in cluster mode when the batch script fails.
func (r *Client) collectPerVMDisks(vmName, vmUUID, computerName string) []types.Disk {
	stdout, err := r.driver.RunOnNode(ps.BuildCommand(ps.GetVMDisks, vmName), computerName)
	if err != nil {
		r.Log.Error(err, "Failed to get disks per-VM", "vm", vmName)
		return []types.Disk{}
	}
	if stdout == "" {
		return []types.Disk{}
	}
	type diskData struct {
		Path               string `json:"Path"`
		ControllerType     int    `json:"ControllerType"`
		ControllerNumber   int    `json:"ControllerNumber"`
		ControllerLocation int    `json:"ControllerLocation"`
	}
	disksData, err := driver.UnmarshalArrayOrSingle[diskData]([]byte(stdout))
	if err != nil {
		r.Log.Error(err, "Failed to parse disks JSON", "vm", vmName)
		return []types.Disk{}
	}
	var disks []types.Disk
	for i, dd := range disksData {
		if dd.Path == "" {
			continue
		}
		smbPath := r.mapWindowsPathToSMB(dd.Path, r.smbWindowsPrefix)
		format := "vhdx"
		if strings.HasSuffix(strings.ToLower(dd.Path), ".vhd") {
			format = "vhd"
		}
		disks = append(disks, types.Disk{
			ID:          fmt.Sprintf("%s-disk-%d", vmUUID, i),
			WindowsPath: dd.Path,
			SMBPath:     smbPath,
			Format:      format,
		})
	}
	return disks
}

// collectPerVMNICs fetches NIC info for a single VM on a specific node.
// Used as fallback in cluster mode when the batch script fails.
func (r *Client) collectPerVMNICs(vmName, computerName string, networks []types.Network) []types.NIC {
	stdout, err := r.driver.RunOnNode(ps.BuildCommand(ps.GetVMNICs, vmName), computerName)
	if err != nil {
		r.Log.Error(err, "Failed to get NICs per-VM", "vm", vmName)
		return []types.NIC{}
	}
	if stdout == "" {
		return []types.NIC{}
	}
	type nicData struct {
		Name       string `json:"Name"`
		MacAddress string `json:"MacAddress"`
		SwitchName string `json:"SwitchName"`
		VlanId     int    `json:"VlanId"`
	}
	nicsData, err := driver.UnmarshalArrayOrSingle[nicData]([]byte(stdout))
	if err != nil {
		r.Log.Error(err, "Failed to parse NICs JSON", "vm", vmName)
		return []types.NIC{}
	}
	var nics []types.NIC
	for i, nd := range nicsData {
		mac := formatMAC(nd.MacAddress)
		nics = append(nics, types.NIC{
			Name:        fmt.Sprintf("nic-%d", i),
			MAC:         mac,
			DeviceIndex: i,
			NetworkUUID: resolveNetworkUUID(nd.SwitchName, computerName, networks),
			NetworkName: nd.SwitchName,
			VlanId:      nd.VlanId,
		})
	}
	return nics
}

func formatMAC(mac string) string {
	mac = strings.ReplaceAll(mac, "-", "")
	mac = strings.ReplaceAll(mac, ":", "")
	mac = strings.ToLower(mac)
	if len(mac) == 12 {
		return fmt.Sprintf("%s:%s:%s:%s:%s:%s",
			mac[0:2], mac[2:4], mac[4:6], mac[6:8], mac[8:10], mac[10:12])
	}
	return mac
}

func (r *Client) collectGuestOS(vmName, computerName string) (string, error) {
	script := ps.BuildCommand(ps.GetGuestOS, vmName)
	stdout, err := r.driver.RunOnNode(script, computerName)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(stdout), nil
}

func (r *Client) collectSecurityInfo(vmName, computerName string) (*securityInfo, error) {
	script := ps.BuildCommand(ps.GetVMSecurityInfo, vmName, vmName, vmName)
	stdout, err := r.driver.RunOnNode(script, computerName)
	if err != nil {
		return nil, err
	}

	stdout = strings.TrimSpace(stdout)
	if stdout == "" || stdout == "{}" {
		return &securityInfo{}, nil
	}

	var info securityInfo
	if err := json.Unmarshal([]byte(stdout), &info); err != nil {
		return nil, fmt.Errorf("failed to parse security info JSON: %w", err)
	}
	return &info, nil
}

func (r *Client) collectHasCheckpoint(vmName, computerName string) (bool, error) {
	script := ps.BuildCommand(ps.GetVMHasCheckpoint, vmName)
	stdout, err := r.driver.RunOnNode(script, computerName)
	if err != nil {
		return false, err
	}
	result, err := strconv.ParseBool(strings.TrimSpace(stdout))
	if err != nil {
		return false, fmt.Errorf("parse checkpoint state for VM %q: %w", vmName, err)
	}
	return result, nil
}

func (r *Client) collectGuestNetworkConfig(vmName string, nics []types.NIC, computerName string) ([]types.GuestNetwork, error) {
	script := ps.BuildCommand(ps.GetGuestNetworkConfig, vmName)
	stdout, err := r.driver.RunOnNode(script, computerName)
	if err != nil {
		return nil, err
	}

	if stdout == "" || strings.Contains(stdout, "no_vm") || strings.Contains(stdout, "no_gc") {
		return []types.GuestNetwork{}, nil
	}

	configs, err := driver.UnmarshalArrayOrSingle[guestNetCfg]([]byte(stdout))
	if err != nil {
		return nil, fmt.Errorf("failed to parse KVP JSON: %w", err)
	}

	return buildGuestNetworks(configs, nics), nil
}

func filterDNSByFamily(dns []string, ipv4 bool) []string {
	seen := make(map[string]struct{})
	var result []string
	for _, d := range dns {
		parsed := net.ParseIP(d)
		if parsed == nil {
			continue
		}
		if (parsed.To4() != nil) != ipv4 {
			continue
		}
		if _, dup := seen[d]; dup {
			continue
		}
		seen[d] = struct{}{}
		result = append(result, d)
	}
	return result
}

func findNICDeviceIndex(mac string, nics []types.NIC) int {
	normalizedMAC := strings.ToUpper(strings.ReplaceAll(mac, ":", ""))
	for _, nic := range nics {
		nicMAC := strings.ToUpper(strings.ReplaceAll(nic.MAC, ":", ""))
		if nicMAC == normalizedMAC {
			return nic.DeviceIndex
		}
	}
	return -1
}

func subnetToPrefixLength(subnet string) int32 {
	ip := net.ParseIP(subnet)
	if ip == nil {
		return 24
	}
	ip4 := ip.To4()
	if ip4 == nil {
		return 24
	}
	ones, _ := net.IPv4Mask(ip4[0], ip4[1], ip4[2], ip4[3]).Size()
	return int32(ones)
}

func parseIPv6PrefixLength(subnet string) int32 {
	var prefixLen int32
	if _, err := fmt.Sscanf(subnet, "%d", &prefixLen); err == nil {
		if prefixLen >= 0 && prefixLen <= 128 {
			return prefixLen
		}
	}

	ip := net.ParseIP(subnet)
	if ip != nil && ip.To4() == nil {
		ones := 0
		for _, b := range ip.To16() {
			for i := 7; i >= 0; i-- {
				if b&(1<<uint(i)) != 0 {
					ones++
				} else {
					return int32(ones)
				}
			}
		}
		return int32(ones)
	}

	return 64
}

func (r *Client) mapWindowsPathToSMB(windowsPath, smbWindowsPrefix string) string {
	if r.smbMountPath == "" {
		r.Log.V(1).Info("Cannot map disk path: SMB mount path not configured",
			"windowsPath", windowsPath)
		return ""
	}

	normalizedWindowsPath := strings.ReplaceAll(windowsPath, "\\", "/")

	// Handle UNC paths (e.g. //SERVER/ShareName/file.vhdx) from cluster
	// nodes that reference the SMB share by network path.
	shareName := extractShareName(r.smbUrl)
	if shareName != "" && strings.HasPrefix(normalizedWindowsPath, "//") {
		parts := strings.SplitN(strings.TrimPrefix(normalizedWindowsPath, "//"), "/", 3)
		if len(parts) >= 2 && strings.EqualFold(parts[1], shareName) {
			relativePath := ""
			if len(parts) == 3 {
				relativePath = parts[2]
			}
			return r.smbMountPath + "/" + relativePath
		}
	}

	// Handle local paths that start with the share's Windows directory.
	if smbWindowsPrefix == "" {
		r.Log.V(1).Info("Cannot map disk path: SMB Windows prefix not discovered",
			"windowsPath", windowsPath)
		return ""
	}
	normalizedPrefix := strings.ReplaceAll(smbWindowsPrefix, "\\", "/")
	if strings.HasPrefix(strings.ToLower(normalizedWindowsPath), strings.ToLower(normalizedPrefix)) {
		relativePath := normalizedWindowsPath[len(normalizedPrefix):]
		relativePath = strings.TrimPrefix(relativePath, "/")
		return r.smbMountPath + "/" + relativePath
	}

	r.Log.Info("Disk path does not match SMB Windows prefix",
		"windowsPath", windowsPath,
		"smbWindowsPrefix", smbWindowsPrefix)
	return ""
}

func (r *Client) getDiskCapacity(windowsPath, computerName string) int64 {
	command := ps.BuildCommand(ps.GetDiskCapacity, windowsPath)
	stdout, err := r.driver.RunOnNode(command, computerName)
	if err != nil {
		r.Log.Error(err, "Failed to get disk capacity", "path", windowsPath)
		return 0
	}
	var capacity int64
	if _, err := fmt.Sscanf(strings.TrimSpace(stdout), "%d", &capacity); err != nil {
		return 0
	}
	return capacity
}

func (r *Client) getDiskRCTEnabled(windowsPath, computerName string) bool {
	command := ps.BuildCommand(ps.GetDiskRCTEnabled, windowsPath)
	stdout, err := r.driver.RunOnNode(command, computerName)
	if err != nil {
		r.Log.Error(err, "Failed to get disk RCT status", "path", windowsPath)
		return false
	}
	result, _ := strconv.ParseBool(strings.TrimSpace(stdout))
	return result
}

func (r *Client) getStorageCapacity(windowsPath string) (capacity int64, free int64) {
	cmd := ps.BuildCommand(ps.GetStorageCapacity, windowsPath)
	output, err := r.driver.ExecuteCommand(cmd)
	if err != nil {
		r.Log.Error(err, "Failed to get storage capacity", "path", windowsPath)
		return 0, 0
	}

	output = strings.TrimSpace(output)
	if output == "" {
		return 0, 0
	}

	var result struct {
		Size          int64 `json:"Size"`
		SizeRemaining int64 `json:"SizeRemaining"`
	}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		r.Log.Error(err, "Failed to parse storage capacity", "output", output)
		return 0, 0
	}
	return result.Size, result.SizeRemaining
}

func (r *Client) discoverSMBWindowsPrefix() error {
	shareName := extractShareName(r.smbUrl)
	if shareName == "" {
		return fmt.Errorf("cannot extract share name from SMB URL: %s", r.smbUrl)
	}

	command := ps.BuildCommand(ps.GetSMBSharePath, shareName)
	stdout, err := r.driver.ExecuteCommand(command)
	if err != nil {
		return fmt.Errorf("Get-SmbShare failed: %w", err)
	}

	windowsPath := strings.TrimSpace(stdout)
	if windowsPath == "" {
		return fmt.Errorf("SMB share '%s' not found", shareName)
	}

	r.smbWindowsPrefix = windowsPath
	r.Log.Info("Discovered SMB Windows prefix", "shareName", shareName, "windowsPath", windowsPath)
	return nil
}

func mapPowerState(state driver.DomainState) string {
	switch state {
	case driver.DOMAIN_RUNNING:
		return "On"
	case driver.DOMAIN_PAUSED:
		return "Paused"
	case driver.DOMAIN_SHUTDOWN:
		return "ShuttingDown"
	case driver.DOMAIN_SHUTOFF:
		return "Off"
	case driver.DOMAIN_CRASHED:
		return "Crashed"
	case driver.DOMAIN_PMSUSPENDED:
		return "Suspended"
	default:
		return "Unknown"
	}
}

// resolveNetworkUUID returns the UUID of the switch named `name`.
// When vmOwnerNode is set, a switch whose OwnerNodes includes that node
// is preferred. A switch with empty OwnerNodes (no ownership data) is
// treated as unscoped and matches any VM. A scoped fallback (populated
// OwnerNodes that don't include vmOwnerNode) is only used when the VM
// owner node is unknown.
func resolveNetworkUUID(name, vmOwnerNode string, networks []types.Network) string {
	if name == "" {
		return ""
	}
	unscopedFallback := ""
	scopedFallback := ""
	for _, n := range networks {
		if !strings.EqualFold(n.Name, name) {
			continue
		}
		if len(n.OwnerNodes) == 0 {
			if unscopedFallback == "" {
				unscopedFallback = n.UUID
			}
			continue
		}
		if vmOwnerNode != "" && containsIgnoreCase(n.OwnerNodes, vmOwnerNode) {
			return n.UUID
		}
		if scopedFallback == "" {
			scopedFallback = n.UUID
		}
	}
	if unscopedFallback != "" {
		return unscopedFallback
	}
	if scopedFallback != "" && vmOwnerNode == "" {
		return scopedFallback
	}
	clientLog.Info("NIC references undiscovered virtual switch",
		"switchName", name,
		"vmOwnerNode", vmOwnerNode,
		"discoveredSwitches", len(networks))
	return ""
}

// containsIgnoreCase reports whether any element of ss case-insensitively
// matches target.
func containsIgnoreCase(ss []string, target string) bool {
	for _, s := range ss {
		if strings.EqualFold(s, target) {
			return true
		}
	}
	return false
}

// mapSwitchType converts the PowerShell SwitchType int to a human-readable string.
func mapSwitchType(switchType int) string {
	switch switchType {
	case 0:
		return "External"
	case 1:
		return "Internal"
	case 2:
		return "Private"
	default:
		return "Unknown"
	}
}

func extractHostFromURL(addr string) string {
	addr = strings.TrimSpace(addr)
	host, _, err := net.SplitHostPort(addr)
	if err == nil {
		return host
	}
	if strings.HasPrefix(addr, "[") && strings.HasSuffix(addr, "]") {
		return addr[1 : len(addr)-1]
	}
	return addr
}

func extractShareName(smbUrl string) string {
	url := strings.TrimPrefix(smbUrl, "smb://")
	url = strings.TrimPrefix(url, "//")
	url = strings.TrimPrefix(url, "\\\\")
	parts := strings.FieldsFunc(url, func(r rune) bool {
		return r == '/' || r == '\\'
	})
	if len(parts) >= 2 {
		return parts[1]
	}
	return ""
}
