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

	"github.com/dustin/go-humanize"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	hvutil "github.com/kubev2v/forklift/pkg/controller/hyperv"
	types "github.com/kubev2v/forklift/pkg/controller/provider/model/hyperv/types"
	"github.com/kubev2v/forklift/pkg/lib/hyperv/driver"
	ps "github.com/kubev2v/forklift/pkg/lib/hyperv/powershell"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	core "k8s.io/api/core/v1"
)

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
func (r *Client) getClusterCache() (*clusterCache, error) {
	if r.cache != nil {
		return r.cache, nil
	}
	clusterData, err := r.driver.GetCluster()
	if err != nil {
		return nil, fmt.Errorf("GetCluster failed: %w", err)
	}
	nodesData, err := r.driver.GetClusterNodes()
	if err != nil {
		return nil, fmt.Errorf("GetClusterNodes failed: %w", err)
	}
	r.cache = &clusterCache{cluster: clusterData, nodes: nodesData}
	return r.cache, nil
}

// InvalidateClusterCache clears cached cluster data so the next call re-fetches.
func (r *Client) InvalidateClusterCache() {
	r.cache = nil
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
	var hosts []types.Host
	for _, n := range cc.nodes {
		host := types.Host{
			ID:          n.Id,
			Name:        n.Name,
			State:       driver.ClusterNodeStateName(n.State),
			ClusterName: cc.cluster.Name,
		}
		info, err := r.getNodeComputerInfo(n.Name)
		if err != nil {
			r.Log.V(1).Info("Failed to get hardware info for node", "node", n.Name, "error", err)
		} else if info != nil {
			host.CpuCount = info.NumberOfProcessors
			host.CpuCores = info.NumberOfLogicalProcessors
			host.MemoryMB = info.TotalVisibleMemoryKB / 1024
		}
		hosts = append(hosts, host)
	}
	return hosts, nil
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

// ListVMs collects all VMs from the HyperV host via WinRM.
// In cluster mode, VMs are enriched with OwnerNode from cluster group data.
// Uses batch PowerShell to minimize WinRM round trips.
func (r *Client) ListVMs() ([]types.VM, error) {
	networks, err := r.ListNetworks()
	if err != nil {
		return nil, err
	}

	isCluster := r.provider != nil && r.provider.IsHyperVCluster()

	var domains []driver.Domain
	if isCluster {
		domains, err = r.driver.ListAllClusterDomains()
	} else {
		domains, err = r.driver.ListAllDomains()
	}
	if err != nil {
		return nil, err
	}

	var vms []types.VM
	for _, domain := range domains {
		var vm *types.VM
		if isCluster {
			vm, err = r.getVMBaseFromDomain(domain)
		} else {
			vm, err = r.getVMFromDomain(domain, networks, r.smbWindowsPrefix)
		}
		if err != nil {
			r.Log.Error(err, "Failed to process domain")
			_ = domain.Free()
			continue
		}
		vms = append(vms, *vm)
		_ = domain.Free()
	}

	if isCluster {
		r.enrichVMsWithOwnerNode(vms)
	}

	r.enrichVMDetails(vms, networks)

	r.validateDisksOnSMB(vms)

	return vms, nil
}

// enrichVMDetails populates VM security, checkpoints, disk capacity/RCT, guest OS,
// and guest networks using batch PowerShell (per-node in cluster mode, local otherwise).
func (r *Client) enrichVMDetails(vms []types.VM, networks []types.Network) {
	if r.provider != nil && r.provider.IsHyperVCluster() {
		// Group VMs by OwnerNode for per-node batch calls.
		nodeVMs := make(map[string][]int)
		for i := range vms {
			node := vms[i].OwnerNode
			nodeVMs[node] = append(nodeVMs[node], i)
		}
		for node, indices := range nodeVMs {
			batchMap, err := r.collectBatchVMDetails(node)
			if err != nil {
				r.Log.Error(err, "Batch detail collection failed for node, falling back to per-VM", "node", node)
				r.fallbackPerVMDetails(vms, indices, networks)
				continue
			}
			r.applyBatchDetails(vms, indices, batchMap, networks)
		}
	} else {
		// Standalone: single batch call for all VMs on this host.
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

// collectBatchVMDetails runs the two batch PowerShell scripts (hardware + guest)
// on the given node and returns a merged map of VM name -> details.
func (r *Client) collectBatchVMDetails(computerName string) (map[string]*batchVMDetail, error) {
	// Part 1: Security, checkpoints, disk capacity/RCT
	hwOut, err := r.driver.RunOnNode(ps.BatchGetVMHardware, computerName)
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

	// Part 2: Guest OS and guest networks (only running VMs)
	guestOut, err := r.driver.RunOnNode(ps.BatchGetVMGuest, computerName)
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

	// Merge guest info into hardware results
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
// In cluster mode (disks/NICs empty), it builds full Disk and NIC arrays from batch data.
// In standalone mode (disks/NICs pre-populated), it only enriches capacity/RCT on existing disks.
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
			// Cluster mode: build full disk array from batch data.
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
			// Standalone mode: enrich existing disks with capacity/RCT.
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
			// Cluster mode: build full NIC array from batch data.
			for j, nd := range detail.NICs {
				mac := formatMAC(nd.MACAddress)
				vms[i].NICs = append(vms[i].NICs, types.NIC{
					Name:        fmt.Sprintf("nic-%d", j),
					MAC:         mac,
					DeviceIndex: j,
					NetworkUUID: resolveNetworkUUID(nd.SwitchName, networks),
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
// that disk files mapped to SMB paths actually exist on the mount. Disks that
// are missing get a DiskNotFoundOnSMB concern attached to their parent VM.
func (r *Client) validateDisksOnSMB(vms []types.VM) {
	if r.provider == nil || r.provider.Status.Service == nil {
		r.Log.V(1).Info("Skipping SMB disk validation: no provider service available")
		return
	}

	svc := r.provider.Status.Service
	baseURL := fmt.Sprintf("http://%s.%s.svc.cluster.local:8080", svc.Name, svc.Namespace)

	// Collect all SMB paths, tracking which VM(s) own each path.
	type pathOwner struct {
		vmIndex  int
		diskPath string
	}
	var allPaths []string
	pathOwners := make(map[string][]pathOwner)
	for i, vm := range vms {
		for _, disk := range vm.Disks {
			if disk.SMBPath == "" {
				continue
			}
			if _, seen := pathOwners[disk.SMBPath]; !seen {
				allPaths = append(allPaths, disk.SMBPath)
			}
			pathOwners[disk.SMBPath] = append(pathOwners[disk.SMBPath], pathOwner{vmIndex: i, diskPath: disk.SMBPath})
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

	for path, owners := range pathOwners {
		if !missingSet[path] {
			continue
		}
		for _, o := range owners {
			vms[o.vmIndex].Concerns = append(vms[o.vmIndex].Concerns, types.Concern{
				Category: "Warning",
				Label:    "DiskNotFoundOnSMB",
				Message:  fmt.Sprintf("Disk file not found on SMB mount: %s", o.diskPath),
			})
		}
	}

	if len(result.Missing) > 0 {
		r.Log.Info("SMB disk validation found missing disks", "count", len(result.Missing))
	}
}

// ListNetworks collects all networks from the HyperV host via WinRM.
func (r *Client) ListNetworks() ([]types.Network, error) {
	netDomains, err := r.driver.ListAllNetworks()
	if err != nil {
		return nil, err
	}

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

		result = append(result, types.Network{
			UUID:       uuid,
			Name:       name,
			SwitchType: switchType,
		})
		_ = n.Free()
	}
	return result, nil
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

func (r *Client) getVMFromDomain(domain driver.Domain, networks []types.Network, smbWindowsPrefix string) (*types.VM, error) {
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

	computerName := domain.GetComputerName()

	vm := &types.VM{
		UUID:       uuid,
		Name:       name,
		PowerState: mapPowerState(state),
		CpuCount:   int(info.NrVirtCpu),
		MemoryMB:   int64(info.Memory / 1024), // KB to MB
		Firmware:   firmware,
		OwnerNode:  computerName,
	}

	vm.Disks = r.extractDisks(domain, smbWindowsPrefix, uuid)
	vm.NICs = r.extractNICs(domain, networks)

	return vm, nil
}

func (r *Client) extractDisks(domain driver.Domain, smbWindowsPrefix string, vmUUID string) []types.Disk {
	diskInfos, err := domain.GetDisks()
	if err != nil {
		r.Log.Error(err, "Failed to get disks")
		return []types.Disk{}
	}

	var disks []types.Disk
	for i, di := range diskInfos {
		if di.Path == "" {
			continue
		}

		smbPath := r.mapWindowsPathToSMB(di.Path, smbWindowsPrefix)

		format := "vhdx"
		if strings.HasSuffix(strings.ToLower(di.Path), ".vhd") {
			format = "vhd"
		}

		disks = append(disks, types.Disk{
			ID:          fmt.Sprintf("%s-disk-%d", vmUUID, i),
			WindowsPath: di.Path,
			SMBPath:     smbPath,
			Format:      format,
		})
	}
	return disks
}

func (r *Client) extractNICs(domain driver.Domain, networks []types.Network) []types.NIC {
	nicInfos, err := domain.GetNICs()
	if err != nil {
		r.Log.Error(err, "Failed to get NICs")
		return []types.NIC{}
	}

	var nics []types.NIC
	for i, ni := range nicInfos {
		networkUUID := resolveNetworkUUID(ni.SwitchName, networks)
		mac := formatMAC(ni.MACAddress)

		nics = append(nics, types.NIC{
			Name:        fmt.Sprintf("nic-%d", i),
			MAC:         mac,
			DeviceIndex: i,
			NetworkUUID: networkUUID,
			NetworkName: ni.SwitchName,
			VlanId:      ni.VlanId,
		})
	}
	return nics
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
	var disksData []diskData
	if err := json.Unmarshal([]byte(stdout), &disksData); err != nil {
		var single diskData
		if err := json.Unmarshal([]byte(stdout), &single); err != nil {
			r.Log.Error(err, "Failed to parse disks JSON", "vm", vmName)
			return []types.Disk{}
		}
		disksData = append(disksData, single)
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
	var nicsData []nicData
	if err := json.Unmarshal([]byte(stdout), &nicsData); err != nil {
		var single nicData
		if err := json.Unmarshal([]byte(stdout), &single); err != nil {
			r.Log.Error(err, "Failed to parse NICs JSON", "vm", vmName)
			return []types.NIC{}
		}
		nicsData = append(nicsData, single)
	}
	var nics []types.NIC
	for i, nd := range nicsData {
		mac := formatMAC(nd.MacAddress)
		nics = append(nics, types.NIC{
			Name:        fmt.Sprintf("nic-%d", i),
			MAC:         mac,
			DeviceIndex: i,
			NetworkUUID: resolveNetworkUUID(nd.SwitchName, networks),
			NetworkName: nd.SwitchName,
			VlanId:      nd.VlanId,
		})
	}
	return nics
}

func formatMAC(mac string) string {
	mac = strings.ReplaceAll(mac, "-", "")
	mac = strings.ReplaceAll(mac, ":", "")
	mac = strings.ToUpper(mac)
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

	var configs []guestNetCfg
	if err := json.Unmarshal([]byte(stdout), &configs); err != nil {
		var single guestNetCfg
		if err := json.Unmarshal([]byte(stdout), &single); err != nil {
			return nil, fmt.Errorf("failed to parse KVP JSON: %w", err)
		}
		configs = append(configs, single)
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

func resolveNetworkUUID(name string, networks []types.Network) string {
	if name == "" {
		return ""
	}
	for _, n := range networks {
		if strings.EqualFold(n.Name, name) {
			return n.UUID
		}
	}
	return ""
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
