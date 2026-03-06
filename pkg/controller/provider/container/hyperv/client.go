package hyperv

import (
	"bytes"
	"encoding/json"
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

// Client talks directly to HyperV host via WinRM.
type Client struct {
	driver           driver.HyperVDriver
	Secret           *core.Secret
	Log              logging.LevelLogger
	provider         *api.Provider
	smbUrl           string
	smbMountPath     string
	smbWindowsPrefix string
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

	drv := driver.NewWinRMDriver(host, driver.WinRMPortHTTPS, username, password, true, nil)
	if err = drv.Connect(); err != nil {
		return fmt.Errorf("WinRM connect failed: %w", err)
	}

	r.driver = drv
	r.provider = provider
	r.smbUrl = hvutil.SMBUrl(r.Secret)
	r.smbMountPath = hvutil.SMBMountPath

	if r.smbUrl != "" {
		if pErr := r.discoverSMBWindowsPrefix(); pErr != nil {
			r.Log.Error(pErr, "Failed to discover SMB Windows prefix, will retry")
		}
	}

	return nil
}

// ListVMs collects all VMs from the HyperV host via WinRM.
func (r *Client) ListVMs() ([]types.VM, error) {
	networks, err := r.ListNetworks()
	if err != nil {
		return nil, err
	}

	domains, err := r.driver.ListAllDomains()
	if err != nil {
		return nil, err
	}

	var vms []types.VM
	for _, domain := range domains {
		vm, err := r.getVMFromDomain(domain, networks, r.smbWindowsPrefix)
		if err != nil {
			r.Log.Error(err, "Failed to process domain")
			_ = domain.Free()
			continue
		}
		vms = append(vms, *vm)
		_ = domain.Free()
	}

	r.validateDisksOnSMB(vms)

	return vms, nil
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

	vm := &types.VM{
		UUID:       uuid,
		Name:       name,
		PowerState: mapPowerState(state),
		CpuCount:   int(info.NrVirtCpu),
		MemoryMB:   int64(info.Memory / 1024), // KB to MB
		Firmware:   firmware,
	}

	if generation == VMGenerationGen2 {
		si, err := r.collectSecurityInfo(name)
		if err != nil {
			r.Log.V(1).Info("Failed to collect security info", "vm", name, "error", err)
		} else {
			vm.TpmEnabled = si.TpmEnabled
			vm.SecureBoot = si.SecureBoot
		}
	}

	hasCheckpoint, err := r.collectHasCheckpoint(name)
	if err != nil {
		r.Log.V(1).Info("Failed to check for checkpoints", "vm", name, "error", err)
	} else {
		vm.HasCheckpoint = hasCheckpoint
	}

	vm.Disks = r.extractDisks(domain, smbWindowsPrefix, uuid)
	vm.NICs = r.extractNICs(domain, networks)

	if vm.PowerState == "On" {
		guestOS, err := r.collectGuestOS(name)
		if err != nil {
			r.Log.V(1).Info("Guest OS detection failed", "vm", name, "error", err)
		} else if guestOS != "" {
			vm.GuestOS = guestOS
		}

		guestNetworks, err := r.collectGuestNetworkConfig(name, vm.NICs)
		if err != nil {
			r.Log.Info("KVP data collection failed", "vm", name, "error", err)
		} else if len(guestNetworks) > 0 {
			vm.GuestNetworks = guestNetworks
		}
	}

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
		capacity := r.getDiskCapacity(di.Path)
		rctEnabled := r.getDiskRCTEnabled(di.Path)

		format := "vhdx"
		if strings.HasSuffix(strings.ToLower(di.Path), ".vhd") {
			format = "vhd"
		}

		disks = append(disks, types.Disk{
			ID:          fmt.Sprintf("%s-disk-%d", vmUUID, i),
			WindowsPath: di.Path,
			SMBPath:     smbPath,
			Capacity:    capacity,
			Format:      format,
			RCTEnabled:  rctEnabled,
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

func (r *Client) collectGuestOS(vmName string) (string, error) {
	script := ps.BuildCommand(ps.GetGuestOS, vmName)
	stdout, err := r.driver.ExecuteCommand(script)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(stdout), nil
}

func (r *Client) collectSecurityInfo(vmName string) (*securityInfo, error) {
	script := ps.BuildCommand(ps.GetVMSecurityInfo, vmName, vmName, vmName)
	stdout, err := r.driver.ExecuteCommand(script)
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

func (r *Client) collectHasCheckpoint(vmName string) (bool, error) {
	script := ps.BuildCommand(ps.GetVMHasCheckpoint, vmName)
	stdout, err := r.driver.ExecuteCommand(script)
	if err != nil {
		return false, err
	}
	result, _ := strconv.ParseBool(strings.TrimSpace(stdout))
	return result, nil
}

func (r *Client) collectGuestNetworkConfig(vmName string, nics []types.NIC) ([]types.GuestNetwork, error) {
	script := ps.BuildCommand(ps.GetGuestNetworkConfig, vmName)
	stdout, err := r.driver.ExecuteCommand(script)
	if err != nil {
		return nil, err
	}

	if stdout == "" || strings.Contains(stdout, "no_vm") || strings.Contains(stdout, "no_gc") {
		return []types.GuestNetwork{}, nil
	}

	type guestNetConfig struct {
		MAC     string   `json:"MAC"`
		IPs     []string `json:"IPs"`
		Subnets []string `json:"Subnets"`
		DHCP    bool     `json:"DHCP"`
		GW      []string `json:"GW"`
		DNS     []string `json:"DNS"`
	}

	var configs []guestNetConfig
	if err := json.Unmarshal([]byte(stdout), &configs); err != nil {
		var single guestNetConfig
		if err := json.Unmarshal([]byte(stdout), &single); err != nil {
			return nil, fmt.Errorf("failed to parse KVP JSON: %w", err)
		}
		configs = append(configs, single)
	}

	var guestNetworks []types.GuestNetwork
	for _, cfg := range configs {
		mac := cfg.MAC
		if len(mac) == 12 && !strings.Contains(mac, ":") {
			mac = fmt.Sprintf("%s:%s:%s:%s:%s:%s", mac[0:2], mac[2:4], mac[4:6], mac[6:8], mac[8:10], mac[10:12])
		}

		deviceIndex := findNICDeviceIndex(mac, nics)
		origin := "Manual"
		if cfg.DHCP {
			origin = "Dhcp"
		}

		for i, ip := range cfg.IPs {
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
			if i < len(cfg.Subnets) {
				if isIPv4 {
					prefixLen = subnetToPrefixLength(cfg.Subnets[i])
				} else {
					prefixLen = parseIPv6PrefixLength(cfg.Subnets[i])
				}
			} else {
				if isIPv4 {
					prefixLen = 24
				} else {
					prefixLen = 64
				}
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
	return guestNetworks, nil
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
	if smbWindowsPrefix == "" || r.smbMountPath == "" {
		r.Log.V(1).Info("Cannot map disk path: SMB Windows prefix not discovered",
			"windowsPath", windowsPath)
		return ""
	}

	normalizedWindowsPath := strings.ReplaceAll(windowsPath, "\\", "/")
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

func (r *Client) getDiskCapacity(windowsPath string) int64 {
	command := ps.BuildCommand(ps.GetDiskCapacity, windowsPath)
	stdout, err := r.driver.ExecuteCommand(command)
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

func (r *Client) getDiskRCTEnabled(windowsPath string) bool {
	command := ps.BuildCommand(ps.GetDiskRCTEnabled, windowsPath)
	stdout, err := r.driver.ExecuteCommand(command)
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
