package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/kubev2v/forklift/cmd/hyperv-provider-server/driver"
	ps "github.com/kubev2v/forklift/cmd/hyperv-provider-server/powershell"
	"github.com/kubev2v/forklift/cmd/provider-common/settings"
	hvutil "github.com/kubev2v/forklift/pkg/controller/hyperv"
	"github.com/kubev2v/forklift/pkg/lib/logging"
)

var log = logging.WithName("hyperv|collector")

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

// VM represents a HyperV virtual machine.
type VM struct {
	UUID          string         `json:"uuid"`
	Name          string         `json:"name"`
	PowerState    string         `json:"powerState"`
	CpuCount      int            `json:"cpuCount"`
	MemoryMB      int64          `json:"memoryMB"`
	Firmware      string         `json:"firmware"`
	GuestOS       string         `json:"guestOS,omitempty"`
	TpmEnabled    bool           `json:"tpmEnabled"`
	SecureBoot    bool           `json:"secureBoot"`
	HasCheckpoint bool           `json:"hasCheckpoint"`
	Disks         []Disk         `json:"disks"`
	NICs          []NIC          `json:"nics"`
	GuestNetworks []GuestNetwork `json:"guestNetworks,omitempty"`
	Concerns      []Concern      `json:"concerns,omitempty"`
}

type Disk struct {
	ID          string `json:"id"`
	WindowsPath string `json:"windowsPath"`
	SMBPath     string `json:"smbPath"`
	Capacity    int64  `json:"capacity"`
	Format      string `json:"format"`
	RCTEnabled  bool   `json:"rctEnabled"` // Resilient Change Tracking for warm migration
}

type NIC struct {
	Name        string `json:"name"`
	MAC         string `json:"mac"`
	DeviceIndex int    `json:"deviceIndex"`
	NetworkUUID string `json:"networkUUID"`
	NetworkName string `json:"networkName"`
}

type GuestNetwork struct {
	MAC          string   `json:"mac"`
	IP           string   `json:"ip"`
	DeviceIndex  int      `json:"deviceIndex"`
	Origin       string   `json:"origin"` // "Manual" or "Dhcp"
	PrefixLength int32    `json:"prefix"`
	DNS          []string `json:"dns"`
	Gateway      string   `json:"gateway"`
}

// Network represents a HyperV virtual network/switch.
type Network struct {
	UUID       string `json:"uuid"`
	Name       string `json:"name"`
	SwitchType string `json:"switchType"`
}

// Storage represents a HyperV storage location (SMB share path).
type Storage struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Path     string `json:"path"`
	Capacity int64  `json:"capacity,omitempty"`
	Free     int64  `json:"free,omitempty"`
}

type Concern struct {
	Category string `json:"category"`
	Label    string `json:"label"`
	Message  string `json:"message"`
}

type securityInfo struct {
	TpmEnabled bool `json:"TpmEnabled"`
	SecureBoot bool `json:"SecureBoot"`
}

type Collector struct {
	settings *settings.ProviderSettings
	driver   driver.HyperVDriver
	ctx      context.Context
	cancel   context.CancelFunc

	// In-memory cache
	mu               sync.RWMutex
	vms              []VM
	networks         []Network
	storages         []Storage
	parity           bool
	smbWindowsPrefix string // Auto-discovered via WinRM Get-SmbShare
}

func NewCollector(s *settings.ProviderSettings) *Collector {
	ctx, cancel := context.WithCancel(context.Background())

	host := extractHostFromURL(s.HyperV.URL)

	// Read CA certificate if path is configured
	var caCert []byte
	if s.HyperV.CACertPath != "" {
		var err error
		caCert, err = os.ReadFile(s.HyperV.CACertPath)
		if err != nil {
			log.Error(err, "Failed to read CA certificate", "path", s.HyperV.CACertPath)
		}
	}

	// Always uses HTTPS (port 5986)
	drv := driver.NewWinRMDriver(host, 0, s.HyperV.Username, s.HyperV.Password, s.HyperV.InsecureSkipVerify, caCert)

	return &Collector{
		settings: s,
		driver:   drv,
		ctx:      ctx,
		cancel:   cancel,
		vms:      []VM{},
		networks: []Network{},
	}
}

func (c *Collector) Start() {
	log.Info("Starting HyperV collector",
		"url", c.settings.HyperV.URL,
		"smbUrl", c.settings.HyperV.SMBUrl,
		"refreshInterval", c.settings.HyperV.RefreshInterval)

	if err := c.driver.Connect(); err != nil {
		log.Error(err, "Failed to connect to Hyper-V host")
	}
	if err := c.discoverSMBWindowsPrefix(); err != nil {
		log.Error(err, "Failed to discover SMB Windows prefix, will retry")
	}

	c.refresh()

	// Periodic refresh
	ticker := time.NewTicker(c.settings.HyperV.RefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			c.driver.Close()
			return
		case <-ticker.C:
			c.refresh()
		}
	}
}

func (c *Collector) Stop() {
	c.cancel()
}

func (c *Collector) GetVMs() []VM {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.vms
}

func (c *Collector) GetNetworks() []Network {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.networks
}

func (c *Collector) GetStorages() []Storage {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.storages
}

func (c *Collector) GetDisks() []Disk {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var disks []Disk
	for _, vm := range c.vms {
		disks = append(disks, vm.Disks...)
	}
	return disks
}

// HasParity returns whether the collector has completed initial sync.
func (c *Collector) HasParity() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.parity
}

// TestConnection performs a live WinRM connectivity check.
func (c *Collector) TestConnection() bool {
	alive, err := c.driver.IsAlive()
	if err != nil {
		log.V(1).Info("Connection test failed", "error", err)
	}
	return alive
}

func (c *Collector) refresh() {
	log.V(1).Info("Refreshing inventory")

	if alive, _ := c.driver.IsAlive(); !alive {
		if err := c.driver.Connect(); err != nil {
			log.Error(err, "Failed to reconnect to Hyper-V host")
			return
		}
	}

	// Load networks first (needed for NIC resolution)
	networks, err := c.loadNetworks()
	if err != nil {
		log.Error(err, "Failed to load networks")
		return
	}

	c.mu.RLock()
	smbWindowsPrefix := c.smbWindowsPrefix
	c.mu.RUnlock()

	vms, err := c.loadVMs(networks, smbWindowsPrefix)
	if err != nil {
		log.Error(err, "Failed to load VMs")
		return
	}

	storages := c.extractStorages()

	c.mu.Lock()
	// KVP data is empty when the VM is off or mid-migration; keep previous values.
	if len(c.vms) > 0 {
		oldVMs := make(map[string]*VM, len(c.vms))
		for i := range c.vms {
			oldVMs[c.vms[i].UUID] = &c.vms[i]
		}
		for i := range vms {
			if old, found := oldVMs[vms[i].UUID]; found {
				if len(vms[i].GuestNetworks) == 0 && len(old.GuestNetworks) > 0 {
					vms[i].GuestNetworks = old.GuestNetworks
				}
				if vms[i].GuestOS == "" && old.GuestOS != "" {
					vms[i].GuestOS = old.GuestOS
				}
			}
		}
	}
	c.networks = networks
	c.vms = vms
	c.storages = storages
	c.parity = true
	c.mu.Unlock()

	log.Info("Inventory refresh complete",
		"vms", len(vms),
		"networks", len(networks),
		"storages", len(storages))
}

func (c *Collector) loadNetworks() ([]Network, error) {
	log.Info("Loading networks from HyperV host")

	netDomains, err := c.driver.ListAllNetworks()
	if err != nil {
		return nil, err
	}

	var result []Network
	for _, net := range netDomains {
		uuid, err := net.GetUUIDString()
		if err != nil {
			log.Error(err, "Failed to get network UUID")
			_ = net.Free()
			continue
		}
		name, err := net.GetName()
		if err != nil {
			log.Error(err, "Failed to get network name", "uuid", uuid)
			_ = net.Free()
			continue
		}
		switchType, _ := net.GetSwitchType()

		result = append(result, Network{
			UUID:       uuid,
			Name:       name,
			SwitchType: switchType,
		})
		_ = net.Free()
	}

	log.Info("Loaded networks", "count", len(result))
	return result, nil
}

func (c *Collector) extractStorages() []Storage {
	smbUrl := c.settings.HyperV.SMBUrl
	if smbUrl == "" {
		log.Info("No SMB URL configured, skipping storage extraction")
		return nil
	}

	c.mu.RLock()
	windowsPath := c.smbWindowsPrefix
	c.mu.RUnlock()

	shareName := extractShareName(smbUrl)
	if shareName == "" {
		shareName = StorageNameDefaultSMB
	}

	storage := Storage{
		ID:   hvutil.StorageIDDefault,
		Name: StorageNamePrefixSMB + shareName,
		Type: StorageTypeSMB,
		Path: windowsPath,
	}

	// Query storage capacity if we have a valid path
	if windowsPath != "" {
		capacity, free := c.getStorageCapacity(windowsPath)
		storage.Capacity = capacity
		storage.Free = free
	}

	log.Info("Extracted storage", "name", storage.Name, "path", windowsPath, "capacity", humanize.IBytes(uint64(storage.Capacity)), "free", humanize.IBytes(uint64(storage.Free)), "smbUrl", smbUrl)
	return []Storage{storage}
}

func (c *Collector) getStorageCapacity(windowsPath string) (capacity int64, free int64) {
	cmd := ps.BuildCommand(ps.GetStorageCapacity, windowsPath)
	output, err := c.driver.ExecuteCommand(cmd)
	if err != nil {
		log.Error(err, "Failed to get storage capacity", "path", windowsPath)
		return 0, 0
	}

	output = strings.TrimSpace(output)
	if output == "" {
		log.Info("No storage capacity info returned", "path", windowsPath)
		return 0, 0
	}

	var result struct {
		Size          int64 `json:"Size"`
		SizeRemaining int64 `json:"SizeRemaining"`
	}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		log.Error(err, "Failed to parse storage capacity", "output", output)
		return 0, 0
	}

	return result.Size, result.SizeRemaining
}

func (c *Collector) loadVMs(networks []Network, smbWindowsPrefix string) ([]VM, error) {
	log.Info("Loading VMs from HyperV host")

	domains, err := c.driver.ListAllDomains()
	if err != nil {
		return nil, err
	}

	log.Info("Found domains", "count", len(domains))

	var vms []VM
	for _, domain := range domains {
		vm, err := c.getVMFromDomain(domain, networks, smbWindowsPrefix)
		if err != nil {
			log.Error(err, "Failed to process domain")
			_ = domain.Free()
			continue
		}
		vms = append(vms, *vm)
		_ = domain.Free()
	}

	log.Info("Loaded VMs", "count", len(vms))
	return vms, nil
}

func (c *Collector) getVMFromDomain(domain driver.Domain, networks []Network, smbWindowsPrefix string) (*VM, error) {
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
		log.V(1).Info("Failed to get VM generation, defaulting to BIOS", "vm", name, "error", err)
	}
	firmware := "bios"
	if generation == VMGenerationGen2 {
		firmware = "uefi"
	}

	vm := &VM{
		UUID:       uuid,
		Name:       name,
		PowerState: mapPowerState(state),
		CpuCount:   int(info.NrVirtCpu),
		MemoryMB:   int64(info.Memory / 1024), // KB to MB
		Firmware:   firmware,
	}

	// Only Gen2 VMs support TPM and Secure Boot
	if generation == VMGenerationGen2 {
		securityInfo, err := c.collectSecurityInfo(name)
		if err != nil {
			log.V(1).Info("Failed to collect security info", "vm", name, "error", err)
		} else {
			vm.TpmEnabled = securityInfo.TpmEnabled
			vm.SecureBoot = securityInfo.SecureBoot
		}
	}

	hasCheckpoint, err := c.collectHasCheckpoint(name)
	if err != nil {
		log.V(1).Info("Failed to check for checkpoints", "vm", name, "error", err)
	} else {
		vm.HasCheckpoint = hasCheckpoint
	}

	vm.Disks = c.extractDisks(domain, smbWindowsPrefix)

	vm.NICs = c.extractNICs(domain, networks)

	if vm.PowerState == "On" {
		guestOS, err := c.collectGuestOS(name)
		if err != nil {
			log.V(1).Info("Guest OS detection failed", "vm", name, "error", err)
		} else if guestOS != "" {
			vm.GuestOS = guestOS
			log.Info("Detected guest OS", "vm", name, "os", guestOS)
		}

		guestNetworks, err := c.collectGuestNetworkConfig(name, vm.NICs)
		if err != nil {
			log.Info("KVP data collection failed", "vm", name, "error", err)
		} else if len(guestNetworks) == 0 {
			log.Info("No guest network info from KVP - check Integration Services Data Exchange is enabled in guest", "vm", name)
		} else {
			vm.GuestNetworks = guestNetworks
			log.Info("Collected guest network info via KVP", "vm", name, "networks", len(guestNetworks))
		}
	}

	vm.Concerns = c.validateDisksExistOnSMB(vm.Disks)

	log.Info("Processed VM", "name", vm.Name, "disks", len(vm.Disks), "nics", len(vm.NICs), "guestNetworks", len(vm.GuestNetworks))
	return vm, nil
}

func (c *Collector) extractDisks(domain driver.Domain, smbWindowsPrefix string) []Disk {
	diskInfos, err := domain.GetDisks()
	if err != nil {
		log.Error(err, "Failed to get disks")
		return []Disk{}
	}

	var disks []Disk
	for i, di := range diskInfos {
		if di.Path == "" {
			continue
		}

		smbPath := c.mapWindowsPathToSMB(di.Path, smbWindowsPrefix)
		capacity := c.getDiskCapacity(di.Path)
		rctEnabled := c.getDiskRCTEnabled(di.Path)

		format := "vhdx"
		if strings.HasSuffix(strings.ToLower(di.Path), ".vhd") {
			format = "vhd"
		}

		disks = append(disks, Disk{
			ID:          fmt.Sprintf("disk-%d", i),
			WindowsPath: di.Path,
			SMBPath:     smbPath,
			Capacity:    capacity,
			Format:      format,
			RCTEnabled:  rctEnabled,
		})
	}

	return disks
}

func (c *Collector) extractNICs(domain driver.Domain, networks []Network) []NIC {
	nicInfos, err := domain.GetNICs()
	if err != nil {
		log.Error(err, "Failed to get NICs")
		return []NIC{}
	}

	var nics []NIC
	for i, ni := range nicInfos {
		networkUUID := resolveNetworkUUID(ni.SwitchName, networks)
		mac := formatMAC(ni.MACAddress)

		nics = append(nics, NIC{
			Name:        fmt.Sprintf("nic-%d", i),
			MAC:         mac,
			DeviceIndex: i,
			NetworkUUID: networkUUID,
			NetworkName: ni.SwitchName,
		})
	}

	return nics
}

// formatMAC normalizes MAC address to colon-separated uppercase format (XX:XX:XX:XX:XX:XX)
func formatMAC(mac string) string {
	mac = strings.ReplaceAll(mac, "-", "")
	mac = strings.ReplaceAll(mac, ":", "")
	mac = strings.ToUpper(mac)

	// If 12 hex chars, format with colons
	if len(mac) == 12 {
		return fmt.Sprintf("%s:%s:%s:%s:%s:%s",
			mac[0:2], mac[2:4], mac[4:6], mac[6:8], mac[8:10], mac[10:12])
	}
	return mac
}

func (c *Collector) collectGuestOS(vmName string) (string, error) {
	script := ps.BuildCommand(ps.GetGuestOS, vmName)
	stdout, err := c.driver.ExecuteCommand(script)
	if err != nil {
		return "", err
	}

	guestOS := strings.TrimSpace(stdout)
	return guestOS, nil
}

func (c *Collector) collectSecurityInfo(vmName string) (*securityInfo, error) {
	script := ps.BuildCommand(ps.GetVMSecurityInfo, vmName, vmName, vmName)
	stdout, err := c.driver.ExecuteCommand(script)
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

func (c *Collector) collectHasCheckpoint(vmName string) (bool, error) {
	script := ps.BuildCommand(ps.GetVMHasCheckpoint, vmName)
	stdout, err := c.driver.ExecuteCommand(script)
	if err != nil {
		return false, err
	}
	result, _ := strconv.ParseBool(strings.TrimSpace(stdout))
	return result, nil
}

// collectGuestNetworkConfig gets network config via KVP Exchange.
func (c *Collector) collectGuestNetworkConfig(vmName string, nics []NIC) ([]GuestNetwork, error) {
	script := ps.BuildCommand(ps.GetGuestNetworkConfig, vmName)
	stdout, err := c.driver.ExecuteCommand(script)
	if err != nil {
		return nil, err
	}

	if stdout == "" {
		log.Info("KVP query returned empty - ensure vmickvpexchange service is running in guest", "vm", vmName)
		return []GuestNetwork{}, nil
	}

	if strings.Contains(stdout, "no_vm") || strings.Contains(stdout, "no_gc") {
		return []GuestNetwork{}, nil
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

	// Convert to GuestNetwork format - collect ALL IPs (IPv4 and IPv6)
	var networks []GuestNetwork
	for _, cfg := range configs {
		// Format MAC with colons (Hyper-V returns "XXXXXXXXXXXX", convert to "XX:XX:XX:XX:XX:XX")
		mac := cfg.MAC
		if len(mac) == 12 && !strings.Contains(mac, ":") {
			mac = fmt.Sprintf("%s:%s:%s:%s:%s:%s", mac[0:2], mac[2:4], mac[4:6], mac[6:8], mac[8:10], mac[10:12])
		}

		// Find matching NIC by MAC address
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

			// Find matching gateway (same IP family)
			gateway := ""
			for _, gw := range cfg.GW {
				gwIP := net.ParseIP(gw)
				if gwIP == nil {
					continue
				}
				gwIsIPv4 := gwIP.To4() != nil
				if isIPv4 == gwIsIPv4 {
					gateway = gw
					break
				}
			}

			// Get prefix length from subnet mask (IPs and Subnets arrays are parallel)
			var prefixLen int32
			if i < len(cfg.Subnets) {
				if isIPv4 {
					prefixLen = subnetToPrefixLength(cfg.Subnets[i])
				} else {
					prefixLen = parseIPv6PrefixLength(cfg.Subnets[i])
				}
			} else {
				// Default prefix lengths
				if isIPv4 {
					prefixLen = 24
				} else {
					prefixLen = 64
				}
			}

			dns := filterDNSByFamily(cfg.DNS, isIPv4)

			networks = append(networks, GuestNetwork{
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

	log.Info("Collected guest network info via KVP", "vm", vmName, "networks", len(networks))
	return networks, nil
}

// filterDNSByFamily returns deduplicated DNS servers matching the given IP family.
// Hyper-V KVP reports all DNS servers (IPv4+IPv6) in a single list per adapter,
// so we split by family to avoid attaching IPv6 DNS to IPv4 entries and vice versa.
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

func findNICDeviceIndex(mac string, nics []NIC) int {
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

// parseIPv6PrefixLength parses IPv6 prefix length from subnet string.
func parseIPv6PrefixLength(subnet string) int32 {
	// Try parsing as a numeric prefix length (e.g., "64")
	var prefixLen int32
	if _, err := fmt.Sscanf(subnet, "%d", &prefixLen); err == nil {
		if prefixLen >= 0 && prefixLen <= 128 {
			return prefixLen
		}
	}

	// Try parsing as an IPv6 address (mask format)
	ip := net.ParseIP(subnet)
	if ip != nil && ip.To4() == nil {
		// Count leading 1 bits
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

	return 64 // Default IPv6 prefix
}

func (c *Collector) mapWindowsPathToSMB(windowsPath, smbWindowsPrefix string) string {
	smbMountPath := c.settings.CatalogPath

	if smbWindowsPrefix == "" || smbMountPath == "" {
		// Cannot map path without knowing the Windows prefix that corresponds to SMB mount
		log.V(1).Info("Cannot map disk path: SMB Windows prefix not discovered",
			"windowsPath", windowsPath)
		return ""
	}

	normalizedWindowsPath := strings.ReplaceAll(windowsPath, "\\", "/")
	normalizedPrefix := strings.ReplaceAll(smbWindowsPrefix, "\\", "/")

	if strings.HasPrefix(strings.ToLower(normalizedWindowsPath), strings.ToLower(normalizedPrefix)) {
		relativePath := normalizedWindowsPath[len(normalizedPrefix):]
		relativePath = strings.TrimPrefix(relativePath, "/")
		return smbMountPath + "/" + relativePath
	}

	// Path doesn't match SMB prefix - return empty to trigger validation warning
	log.Info("Disk path does not match SMB Windows prefix",
		"windowsPath", windowsPath,
		"smbWindowsPrefix", smbWindowsPrefix)
	return ""
}

func (c *Collector) getDiskCapacity(windowsPath string) int64 {
	command := ps.BuildCommand(ps.GetDiskCapacity, windowsPath)
	stdout, err := c.driver.ExecuteCommand(command)
	if err != nil {
		log.Error(err, "Failed to get disk capacity", "path", windowsPath)
		return 0
	}

	var capacity int64
	if _, err := fmt.Sscanf(strings.TrimSpace(stdout), "%d", &capacity); err != nil {
		return 0
	}
	return capacity
}

func (c *Collector) getDiskRCTEnabled(windowsPath string) bool {
	command := ps.BuildCommand(ps.GetDiskRCTEnabled, windowsPath)
	stdout, err := c.driver.ExecuteCommand(command)
	if err != nil {
		log.Error(err, "Failed to get disk RCT status", "path", windowsPath)
		return false
	}
	result, _ := strconv.ParseBool(strings.TrimSpace(stdout))
	return result
}

func (c *Collector) discoverSMBWindowsPrefix() error {
	shareName := extractShareName(c.settings.HyperV.SMBUrl)
	if shareName == "" {
		return fmt.Errorf("cannot extract share name from SMB URL: %s", c.settings.HyperV.SMBUrl)
	}

	command := ps.BuildCommand(ps.GetSMBSharePath, shareName)
	stdout, err := c.driver.ExecuteCommand(command)
	if err != nil {
		return fmt.Errorf("Get-SmbShare failed: %w", err)
	}

	windowsPath := strings.TrimSpace(stdout)
	if windowsPath == "" {
		return fmt.Errorf("SMB share '%s' not found", shareName)
	}

	c.mu.Lock()
	c.smbWindowsPrefix = windowsPath
	c.mu.Unlock()

	log.Info("Discovered SMB Windows prefix", "shareName", shareName, "windowsPath", windowsPath)
	return nil
}

func resolveNetworkUUID(name string, networks []Network) string {
	if name == "" {
		return ""
	}

	for _, net := range networks {
		if strings.EqualFold(net.Name, name) {
			return net.UUID
		}
	}
	return ""
}

// validateDisksExistOnSMB checks if disk files actually exist on the SMB mount.
func (c *Collector) validateDisksExistOnSMB(disks []Disk) []Concern {
	var concerns []Concern

	for _, disk := range disks {
		// Skip if no SMB path (already flagged by Rego policy)
		if disk.SMBPath == "" {
			continue
		}

		if _, err := os.Stat(disk.SMBPath); os.IsNotExist(err) {
			concerns = append(concerns, Concern{
				Category: "Warning",
				Label:    "DiskNotFoundOnSMB",
				Message:  fmt.Sprintf("Disk not found on SMB mount: %s", disk.SMBPath),
			})
		}
	}

	return concerns
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

// extractHostFromURL extracts the host/IP from a HyperV URL.
// Handles IPv4, IPv6 (with brackets), and optional port.
func extractHostFromURL(addr string) string {
	addr = strings.TrimSpace(addr)

	// Try to split host:port (handles IPv6 [host]:port correctly)
	host, _, err := net.SplitHostPort(addr)
	if err == nil {
		return host
	}

	// No port specified - check for IPv6 brackets
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
