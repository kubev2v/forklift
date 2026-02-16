package hyperv

import (
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"net/url"
	"strings"
	"unicode/utf16"

	ps "github.com/kubev2v/forklift/cmd/hyperv-provider-server/powershell"
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	base "github.com/kubev2v/forklift/pkg/controller/base"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/controller/plan/util"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/hyperv"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/hyperv"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	"github.com/masterzen/winrm"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

var log = logging.WithName("hyperv|client")

// HyperV VM Client
type Client struct {
	*plancontext.Context
	winrmClient *winrm.Client
}

func (r *Client) connect() error {
	secret := r.Source.Secret
	if secret == nil {
		return fmt.Errorf("source secret not available")
	}

	// Extract host from provider URL
	host := extractHost(r.Source.Provider.Spec.URL)
	username := string(secret.Data["username"])
	password := string(secret.Data["password"])

	port := base.WinRMPortHTTPS

	// Read TLS settings from secret
	insecureSkipVerify := base.GetInsecureSkipVerifyFlag(secret)
	var caCert []byte
	if cacert, ok := secret.Data["cacert"]; ok {
		caCert = cacert
	}

	endpoint := winrm.NewEndpoint(host, port, true, insecureSkipVerify, caCert, nil, nil, 0)
	client, err := winrm.NewClient(endpoint, username, password)
	if err != nil {
		return fmt.Errorf("failed to create WinRM client: %w", err)
	}

	r.winrmClient = client
	log.Info("Connected to Hyper-V via WinRM/HTTPS", "host", host, "port", port, "insecureSkipVerify", insecureSkipVerify)
	return nil
}

func (r *Client) Close() {
	r.winrmClient = nil
}

func (r *Client) Finalize(_ []*planapi.VMStatus, _ string) {
}

func (r *Client) DetachDisks(_ ref.Ref) error {
	return nil
}

func (r *Client) PowerState(vmRef ref.Ref) (planapi.VMPowerState, error) {
	vm := &hyperv.VM{}
	err := r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		return planapi.VMPowerStateUnknown, err
	}

	switch vm.PowerState {
	case model.PowerStateOn:
		return planapi.VMPowerStateOn, nil
	case model.PowerStateOff:
		return planapi.VMPowerStateOff, nil
	default:
		return planapi.VMPowerStateUnknown, nil
	}
}

func (r *Client) PowerOn(_ ref.Ref) error {
	// Not needed for migration
	return nil
}

func (r *Client) PowerOff(vmRef ref.Ref) error {
	vm := &hyperv.VM{}
	err := r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		return err
	}

	if vm.PowerState == model.PowerStateOff {
		log.Info("VM already powered off", "vm", vm.Name)
		return nil
	}

	cmd := ps.BuildCommand(ps.StopVM, vm.Name)
	_, err = r.executeCommand(cmd)
	if err != nil {
		return fmt.Errorf("failed to power off VM %s: %w", vm.Name, err)
	}

	log.Info("Powered off VM", "vm", vm.Name)
	return nil
}

func (r *Client) PoweredOff(vmRef ref.Ref) (bool, error) {
	state, err := r.PowerState(vmRef)
	if err != nil {
		return false, err
	}
	return state == planapi.VMPowerStateOff, nil
}

func (r *Client) CreateSnapshot(_ ref.Ref, _ util.HostsFunc) (string, string, error) {
	return "", "", nil
}

func (r *Client) RemoveSnapshot(_ ref.Ref, _ string, _ util.HostsFunc) (string, error) {
	return "", nil
}

func (r *Client) CheckSnapshotReady(_ ref.Ref, _ planapi.Precopy, _ util.HostsFunc) (bool, string, error) {
	return true, "", nil
}

func (r *Client) CheckSnapshotRemove(_ ref.Ref, _ planapi.Precopy, _ util.HostsFunc) (bool, error) {
	return true, nil
}

func (r *Client) SetCheckpoints(_ ref.Ref, _ []planapi.Precopy, _ []cdi.DataVolume, _ bool, _ util.HostsFunc) error {
	return nil
}

func (r *Client) PreTransferActions(_ ref.Ref) (bool, error) {
	return true, nil
}

func (r *Client) GetSnapshotDeltas(_ ref.Ref, _ string, _ util.HostsFunc) (s map[string]string, err error) {
	return
}

func extractHost(addr string) string {
	addr = strings.TrimSpace(addr)

	// Handle full URLs with scheme (e.g., https://host:5986/wsman)
	if strings.Contains(addr, "://") {
		if u, err := url.Parse(addr); err == nil {
			addr = u.Host
		}
	}

	// Handle host:port - use net.SplitHostPort for IPv6 safety
	if host, _, err := net.SplitHostPort(addr); err == nil {
		return host
	}

	// Handle bare IPv6 in brackets: [::1]
	if strings.HasPrefix(addr, "[") && strings.HasSuffix(addr, "]") {
		return addr[1 : len(addr)-1]
	}

	return addr
}

// executeCommand runs a PowerShell command via WinRM
func (r *Client) executeCommand(command string) (string, error) {
	if r.winrmClient == nil {
		// Lazy initialization / reconnection
		if err := r.connect(); err != nil {
			return "", fmt.Errorf("WinRM client not connected: %w", err)
		}
	}

	originalCmd := command
	// Wrap in powershell if not already
	if !strings.HasPrefix(strings.ToLower(command), "powershell") {
		// For complex scripts (multiline or containing quotes), use encoded command
		if strings.Contains(command, "\n") || strings.Contains(command, "'") || strings.Contains(command, "\"") {
			encoded := utf16LEEncode(command)
			command = fmt.Sprintf(`powershell -EncodedCommand %s`, encoded)
		} else {
			command = fmt.Sprintf(`powershell -Command "%s"`, command)
		}
	}

	// Log at debug level to avoid exposing sensitive command content
	log.V(1).Info("Executing WinRM command", "commandLength", len(originalCmd))

	stdout, stderr, exitCode, err := r.winrmClient.RunWithContextWithString(context.Background(), command, "")
	if err != nil {
		// Avoid logging full stdout/stderr which may contain sensitive data
		log.Error(err, "WinRM command failed", "exitCode", exitCode, "stderrLength", len(stderr))
		return "", fmt.Errorf("WinRM command failed: %w", err)
	}

	if exitCode != 0 {
		return "", fmt.Errorf("command exited with code %d: %s", exitCode, stderr)
	}

	return strings.TrimSpace(stdout), nil
}

// utf16LEEncode encodes a string to UTF-16LE then base64 for PowerShell -EncodedCommand
func utf16LEEncode(s string) string {
	u16 := utf16.Encode([]rune(s))
	bytes := make([]byte, len(u16)*2)
	for i, v := range u16 {
		bytes[i*2] = byte(v)
		bytes[i*2+1] = byte(v >> 8)
	}
	return base64.StdEncoding.EncodeToString(bytes)
}
