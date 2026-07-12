package driver

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf16"

	ps "github.com/kubev2v/forklift/pkg/lib/hyperv/powershell"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	"github.com/masterzen/winrm"
)

var log = logging.WithName("hyperv|driver")

const defaultCommandTimeout = 60 * time.Second

const WinRMPortHTTPS = 5986

const (
	HyperVStateRunning   = 2
	HyperVStateOff       = 3
	HyperVStateSuspended = 6
	HyperVStatePaused    = 9
)

type VMData struct {
	Id             string `json:"Id"`
	Name           string `json:"Name"`
	State          int    `json:"State"`
	ProcessorCount int    `json:"ProcessorCount"`
	MemoryStartup  int64  `json:"MemoryStartup"`
	Generation     int    `json:"Generation"`
	ComputerName   string `json:"ComputerName,omitempty"`
}

type SwitchData struct {
	Id         string `json:"Id"`
	Name       string `json:"Name"`
	SwitchType int    `json:"SwitchType"`
}

type ClusterData struct {
	Name   string `json:"Name"`
	Domain string `json:"Domain"`
}

type ClusterNodeData struct {
	Name  string `json:"Name"`
	State int    `json:"State"`
	Id    string `json:"Id"`
}

// ClusterNode State values from the FailoverClusters module.
const (
	ClusterNodeStateUp      = 0
	ClusterNodeStateDown    = 1
	ClusterNodeStatePaused  = 2
	ClusterNodeStateJoining = 3
)

func ClusterNodeStateName(state int) string {
	switch state {
	case ClusterNodeStateUp:
		return "Up"
	case ClusterNodeStateDown:
		return "Down"
	case ClusterNodeStatePaused:
		return "Paused"
	case ClusterNodeStateJoining:
		return "Joining"
	default:
		return "Unknown"
	}
}

type ClusterGroupData struct {
	Name      string `json:"Name"`
	OwnerNode string `json:"OwnerNode"`
	State     int    `json:"State"`
	Id        string `json:"Id"`
}

type ComputerInfoData struct {
	DNSHostName               string `json:"CsDNSHostName"`
	Domain                    string `json:"CsDomain"`
	NumberOfProcessors        int    `json:"CsNumberOfProcessors"`
	NumberOfLogicalProcessors int    `json:"CsNumberOfLogicalProcessors"`
	TotalVisibleMemoryKB      int64  `json:"OsTotalVisibleMemorySize"`
}

type WinRMDriver struct {
	mu                 sync.Mutex
	host               string
	port               int
	username           string
	password           string
	insecureSkipVerify bool
	caCert             []byte
	client             *winrm.Client
}

func NewWinRMDriver(host string, port int, username, password string, insecureSkipVerify bool, caCert []byte) *WinRMDriver {
	if port == 0 {
		port = WinRMPortHTTPS
	}
	return &WinRMDriver{
		host:               host,
		port:               port,
		username:           username,
		password:           password,
		insecureSkipVerify: insecureSkipVerify,
		caCert:             caCert,
	}
}

func (d *WinRMDriver) Connect() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	useHTTPS := true
	endpoint := winrm.NewEndpoint(d.host, d.port, useHTTPS, d.insecureSkipVerify, d.caCert, nil, nil, 0)
	client, err := winrm.NewClient(endpoint, d.username, d.password)
	if err != nil {
		return fmt.Errorf("failed to create WinRM client: %w", err)
	}
	d.client = client
	log.Info("WinRM client initialized.", "host", d.host, "port", d.port, "insecureSkipVerify", d.insecureSkipVerify)
	return nil
}

func (d *WinRMDriver) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.client = nil
	return nil
}

func (d *WinRMDriver) IsAlive() (bool, error) {
	_, err := d.ExecuteCommand(ps.TestConnection)
	return err == nil, err
}

func (d *WinRMDriver) ExecuteCommand(command string) (string, error) {
	return d.ExecuteCommandWithTimeout(command, defaultCommandTimeout)
}

func (d *WinRMDriver) ExecuteCommandWithTimeout(command string, timeout time.Duration) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.client == nil {
		return "", fmt.Errorf("WinRM client not connected")
	}

	// Wrap in powershell if not already
	if !strings.HasPrefix(strings.ToLower(command), "powershell") {
		// For complex scripts (multiline or containing quotes), use encoded command
		if strings.Contains(command, "\n") || strings.Contains(command, "'") || strings.Contains(command, "\"") {
			encoded := base64.StdEncoding.EncodeToString(utf16LEEncode(command))
			command = fmt.Sprintf(`powershell -EncodedCommand %s`, encoded)
		} else {
			command = fmt.Sprintf(`powershell -Command "%s"`, command)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	stdout, stderr, exitCode, err := d.client.RunWithContextWithString(ctx, command, "")
	if err != nil {
		return "", WrapCommandError(fmt.Errorf("WinRM command failed: %w", err))
	}

	if exitCode != 0 {
		return "", fmt.Errorf("command exited with code %d: %s", exitCode, stderr)
	}

	return strings.TrimSpace(stdout), nil
}

func (d *WinRMDriver) RunOnNode(command, computerName string) (string, error) {
	cmd := ps.RunOnNode(command, computerName, d.password, d.username)
	return d.ExecuteCommand(cmd)
}

func utf16LEEncode(s string) []byte {
	u16 := utf16.Encode([]rune(s))
	encoded := make([]byte, len(u16)*2)
	for i, v := range u16 {
		encoded[i*2] = byte(v)
		encoded[i*2+1] = byte(v >> 8)
	}
	return encoded
}

func (d *WinRMDriver) ListAllDomains() ([]Domain, error) {
	stdout, err := d.ExecuteCommand(ps.ListAllVMs)
	if err != nil {
		return nil, err
	}

	if stdout == "" {
		return []Domain{}, nil
	}

	var vmsData []VMData
	// Try array first
	if err := json.Unmarshal([]byte(stdout), &vmsData); err != nil {
		// Try single object
		var single VMData
		if err := json.Unmarshal([]byte(stdout), &single); err != nil {
			return nil, fmt.Errorf("failed to parse VMs JSON: %w", err)
		}
		vmsData = append(vmsData, single)
	}

	var domains []Domain
	for i := range vmsData {
		domains = append(domains, &WinRMDomain{
			driver: d,
			vmData: &vmsData[i],
		})
	}
	return domains, nil
}

func (d *WinRMDriver) ListAllClusterDomains() ([]Domain, error) {
	cmd := ps.BuildCommand(ps.ListClusterVMs, d.password, d.username)
	stdout, err := d.ExecuteCommand(cmd)
	if err != nil {
		return nil, err
	}

	if stdout == "" {
		return []Domain{}, nil
	}

	var vmsData []VMData
	if err := json.Unmarshal([]byte(stdout), &vmsData); err != nil {
		var single VMData
		if err := json.Unmarshal([]byte(stdout), &single); err != nil {
			return nil, fmt.Errorf("failed to parse cluster VMs JSON: %w", err)
		}
		vmsData = append(vmsData, single)
	}

	var domains []Domain
	for i := range vmsData {
		domains = append(domains, &WinRMDomain{
			driver: d,
			vmData: &vmsData[i],
		})
	}
	return domains, nil
}

func (d *WinRMDriver) LookupDomainByName(name string) (Domain, error) {
	cmd := ps.BuildCommand(ps.GetVMByName, name)
	stdout, err := d.ExecuteCommand(cmd)
	if err != nil {
		return nil, err
	}

	if stdout == "" {
		return nil, fmt.Errorf("VM not found: %s", name)
	}

	var vmData VMData
	if err := json.Unmarshal([]byte(stdout), &vmData); err != nil {
		return nil, fmt.Errorf("failed to parse VM JSON: %w", err)
	}
	return &WinRMDomain{driver: d, vmData: &vmData}, nil
}

func (d *WinRMDriver) LookupDomainByUUIDString(uuid string) (Domain, error) {
	cmd := ps.BuildCommand(ps.GetVMByID, uuid)
	stdout, err := d.ExecuteCommand(cmd)
	if err != nil {
		return nil, err
	}

	if stdout == "" {
		return nil, fmt.Errorf("VM not found: %s", uuid)
	}

	var vmData VMData
	if err := json.Unmarshal([]byte(stdout), &vmData); err != nil {
		return nil, fmt.Errorf("failed to parse VM JSON: %w", err)
	}
	return &WinRMDomain{driver: d, vmData: &vmData}, nil
}

func (d *WinRMDriver) ListAllNetworks() ([]Network, error) {
	stdout, err := d.ExecuteCommand(ps.ListAllSwitches)
	if err != nil {
		return nil, err
	}

	if stdout == "" {
		return []Network{}, nil
	}

	var switchesData []SwitchData
	if err := json.Unmarshal([]byte(stdout), &switchesData); err != nil {
		var single SwitchData
		if err := json.Unmarshal([]byte(stdout), &single); err != nil {
			return nil, fmt.Errorf("failed to parse switches JSON: %w", err)
		}
		switchesData = append(switchesData, single)
	}

	var networks []Network
	for i := range switchesData {
		networks = append(networks, &WinRMNetwork{
			switchData: &switchesData[i],
		})
	}
	return networks, nil
}

func (d *WinRMDriver) LookupNetworkByUUIDString(uuid string) (Network, error) {
	networks, err := d.ListAllNetworks()
	if err != nil {
		return nil, err
	}

	for _, net := range networks {
		netUUID, err := net.GetUUIDString()
		if err != nil {
			log.V(1).Info("Failed to get network UUID, skipping", "error", err)
			continue
		}
		if strings.EqualFold(netUUID, uuid) {
			return net, nil
		}
	}
	return nil, fmt.Errorf("network not found: %s", uuid)
}

// runSingle executes a PowerShell script and unmarshals the JSON output into a
// single object of type T. Returns an error when stdout is empty.
func runSingle[T any](d *WinRMDriver, script, label string) (*T, error) {
	stdout, err := d.ExecuteCommand(script)
	if err != nil {
		return nil, err
	}
	if stdout == "" {
		return nil, fmt.Errorf("no %s returned", label)
	}
	var result T
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		return nil, fmt.Errorf("failed to parse %s JSON: %w", label, err)
	}
	return &result, nil
}

// runList executes a PowerShell script and unmarshals the JSON output into a
// slice of T. Handles PowerShell's behavior of returning a bare object instead
// of a one-element array.
func runList[T any](d *WinRMDriver, script, label string) ([]T, error) {
	stdout, err := d.ExecuteCommand(script)
	if err != nil {
		return nil, err
	}
	if stdout == "" {
		return []T{}, nil
	}
	var results []T
	if err := json.Unmarshal([]byte(stdout), &results); err != nil {
		var single T
		if err := json.Unmarshal([]byte(stdout), &single); err != nil {
			return nil, fmt.Errorf("failed to parse %s JSON: %w", label, err)
		}
		results = append(results, single)
	}
	return results, nil
}

func (d *WinRMDriver) GetCluster() (*ClusterData, error) {
	return runSingle[ClusterData](d, ps.GetCluster, "cluster")
}

func (d *WinRMDriver) GetClusterNodes() ([]ClusterNodeData, error) {
	return runList[ClusterNodeData](d, ps.GetClusterNodes, "cluster nodes")
}

func (d *WinRMDriver) GetClusterVMGroups() ([]ClusterGroupData, error) {
	return runList[ClusterGroupData](d, ps.GetClusterVMGroups, "cluster VM groups")
}

func (d *WinRMDriver) GetComputerInfo() (*ComputerInfoData, error) {
	return runSingle[ComputerInfoData](d, ps.GetComputerInfo, "computer info")
}

// WinRMDomain implements Domain interface
type WinRMDomain struct {
	driver *WinRMDriver
	vmData *VMData
}

func (d *WinRMDomain) GetName() (string, error) {
	return d.vmData.Name, nil
}

func (d *WinRMDomain) GetUUIDString() (string, error) {
	return d.vmData.Id, nil
}

func (d *WinRMDomain) GetState() (DomainState, int, error) {
	switch d.vmData.State {
	case HyperVStateRunning:
		return DOMAIN_RUNNING, 0, nil
	case HyperVStateOff:
		return DOMAIN_SHUTOFF, 0, nil
	case HyperVStateSuspended:
		return DOMAIN_PMSUSPENDED, 0, nil
	case HyperVStatePaused:
		return DOMAIN_PAUSED, 0, nil
	default:
		return DOMAIN_NOSTATE, 0, nil
	}
}

func (d *WinRMDomain) GetInfo() (*DomainInfo, error) {
	state, _, err := d.GetState()
	if err != nil {
		return nil, fmt.Errorf("failed to get domain state: %w", err)
	}
	return &DomainInfo{
		State:     state,
		MaxMem:    uint64(d.vmData.MemoryStartup / 1024), // bytes to KB
		Memory:    uint64(d.vmData.MemoryStartup / 1024),
		NrVirtCpu: uint16(d.vmData.ProcessorCount),
	}, nil
}

func (d *WinRMDomain) GetGeneration() (int, error) {
	return d.vmData.Generation, nil
}

func (d *WinRMDomain) GetComputerName() string {
	return d.vmData.ComputerName
}

// nodeCommand wraps cmd to run on the VM's owner node via Invoke-Command
// when ComputerName is set (cluster mode). In standalone mode it returns cmd unchanged.
func (d *WinRMDomain) nodeCommand(cmd string) string {
	return ps.RunOnNode(cmd, d.vmData.ComputerName, d.driver.password, d.driver.username)
}

func (d *WinRMDomain) GetDisks() ([]DiskInfo, error) {
	cmd := d.nodeCommand(ps.BuildCommand(ps.GetVMDisks, d.vmData.Name))
	stdout, err := d.driver.ExecuteCommand(cmd)
	if err != nil {
		return nil, err
	}

	if stdout == "" {
		return []DiskInfo{}, nil
	}

	type diskData struct {
		Path               string `json:"Path"`
		ControllerType     int    `json:"ControllerType"` // 0=IDE, 1=SCSI
		ControllerNumber   int    `json:"ControllerNumber"`
		ControllerLocation int    `json:"ControllerLocation"`
	}

	var disksData []diskData
	if err := json.Unmarshal([]byte(stdout), &disksData); err != nil {
		var single diskData
		if err := json.Unmarshal([]byte(stdout), &single); err != nil {
			return nil, fmt.Errorf("failed to parse disks JSON: %w", err)
		}
		disksData = append(disksData, single)
	}

	var disks []DiskInfo
	for _, dd := range disksData {
		controllerType := "IDE"
		if dd.ControllerType == 1 {
			controllerType = "SCSI"
		}
		disks = append(disks, DiskInfo{
			Path:           dd.Path,
			ControllerType: controllerType,
			ControllerNum:  dd.ControllerNumber,
			ControllerLoc:  dd.ControllerLocation,
		})
	}
	return disks, nil
}

func (d *WinRMDomain) GetNICs() ([]NICInfo, error) {
	cmd := d.nodeCommand(ps.BuildCommand(ps.GetVMNICs, d.vmData.Name))
	stdout, err := d.driver.ExecuteCommand(cmd)
	if err != nil {
		return nil, err
	}

	if stdout == "" {
		return []NICInfo{}, nil
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
			return nil, fmt.Errorf("failed to parse NICs JSON: %w", err)
		}
		nicsData = append(nicsData, single)
	}

	var nics []NICInfo
	for _, nd := range nicsData {
		nics = append(nics, NICInfo{
			Name:       nd.Name,
			MACAddress: nd.MacAddress,
			SwitchName: nd.SwitchName,
			VlanId:     nd.VlanId,
		})
	}
	return nics, nil
}

func (d *WinRMDomain) Shutdown(_ context.Context) error {
	cmd := ps.BuildCommand(ps.StopVM, d.vmData.Name)
	_, err := d.driver.ExecuteCommand(cmd)
	return err
}

func (d *WinRMDomain) Free() error {
	return nil // No resources to free for WinRM
}

// WinRMNetwork implements Network interface
type WinRMNetwork struct {
	switchData *SwitchData
}

func (n *WinRMNetwork) GetName() (string, error) {
	return n.switchData.Name, nil
}

func (n *WinRMNetwork) GetUUIDString() (string, error) {
	return n.switchData.Id, nil
}

func (n *WinRMNetwork) GetSwitchType() (string, error) {
	switch n.switchData.SwitchType {
	case 0:
		return "External", nil
	case 1:
		return "Internal", nil
	case 2:
		return "Private", nil
	default:
		return "Unknown", nil
	}
}

func (n *WinRMNetwork) Free() error {
	return nil
}
