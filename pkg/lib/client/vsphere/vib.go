package vsphere

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/kubev2v/forklift/pkg/lib/client/vsphere/vmware"
	"github.com/vmware/govmomi/cli/esx"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25/mo"
	"golang.org/x/crypto/ssh"
	"k8s.io/klog/v2"
)

const (
	vibName = "vmkfstools-wrapper"
)

// VibVersion is set by ldflags
var VibVersion = "x.x.x"

//go:generate go run go.uber.org/mock/mockgen -destination=mocks/vib_installer_mock.go -package=vsphere_mocks . VIBInstaller

// VIBInstaller handles VIB installation on ESXi hosts (disk-only; caller must restart hostd to activate).
type VIBInstaller interface {
	InstallVib(ctx context.Context, client vmware.Client, esx *object.HostSystem, localVibPath string) error
}

// DefaultVIBInstaller is the production implementation
type DefaultVIBInstaller struct{}

// InstallVib installs the VIB on disk if needed. It does NOT restart hostd — the caller must do that to load the VIB into memory.
func (d *DefaultVIBInstaller) InstallVib(ctx context.Context, client vmware.Client, esx *object.HostSystem, localVibPath string) error {
	// Try the lightweight loaded-version check first (asks hostd, no esxcli).
	// If this succeeds and matches, the VIB is already active — skip installation.
	// If this fails but the on-disk version matches below, we still return nil:
	// the VIB is installed on disk but not yet loaded into hostd memory.
	// The caller (Host controller) is responsible for restarting hostd to activate it.
	loadedVersion, err := GetLoadedVIBVersion(ctx, client, esx)
	if err == nil && loadedVersion == VibVersion {
		return nil
	}

	// Fall back to the on-disk VIB database (esxcli software vib get).
	version, err := GetVIBVersion(ctx, client, esx)
	if err != nil {
		return fmt.Errorf("failed to get the VIB version from ESXi %s: %w", esx.Name(), err)
	}

	if version == VibVersion {
		return nil
	}
	dsName, err := GetHostDatastore(ctx, esx)
	if err != nil {
		return fmt.Errorf("failed to get datastore for ESXi %s: %w", esx.Name(), err)
	}
	return InstallVibToDatastore(ctx, client, esx, localVibPath, dsName, esx.Reference().Value)
}

// InstallVibToDatastore uploads the VIB to the given datastore and installs it via esxcli.
// The caller is responsible for restarting hostd to activate the VIB.
// Use this when the datastore name is already known (e.g. from an annotation cache) to
// avoid the GetHostDatastore discovery query.
func InstallVibToDatastore(ctx context.Context, client vmware.Client, esx *object.HostSystem, localVibPath, dsName, hostIP string) error {
	dc, err := GetHostDC(ctx, esx)
	if err != nil {
		return err
	}
	vibPath, err := uploadVib(ctx, client, dc, dsName, localVibPath)
	if err != nil {
		return fmt.Errorf("failed to upload the VIB to ESXi %s: %w", hostIP, err)
	}
	klog.Infof("uploaded vib to ESXi %s", hostIP)

	if err = installVib(ctx, client, esx, vibPath); err != nil {
		return fmt.Errorf("failed to install the VIB on ESXi %s: %w", hostIP, err)
	}
	klog.Infof("installed vib on ESXi %s version %s", hostIP, VibVersion)
	return nil
}

// GetHostDC retrieves the datacenter for a given ESXi host by walking the inventory tree
func GetHostDC(ctx context.Context, esx *object.HostSystem) (*object.Datacenter, error) {
	hostRef := esx.Reference()
	pc := property.DefaultCollector(esx.Client())
	var hostMo mo.HostSystem
	err := pc.RetrieveOne(ctx, hostRef, []string{"parent"}, &hostMo)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve host parent: %w", err)
	}

	parentRef := hostMo.Parent
	currentParentRef := parentRef

	for {
		if currentParentRef.Type == "Datacenter" {
			finder := find.NewFinder(esx.Client(), true)
			dc, err := finder.Datacenter(ctx, currentParentRef.String())
			if err != nil {
				return nil, fmt.Errorf("failed to find datacenter %s: %w", currentParentRef.String(), err)
			}
			return dc, nil
		}

		var genericParentMo mo.ManagedEntity
		err = pc.RetrieveOne(ctx, *currentParentRef, []string{"parent"}, &genericParentMo)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve intermediate parent: %w", err)
		}

		if genericParentMo.Parent == nil {
			break
		}
		currentParentRef = genericParentMo.Parent
	}

	return nil, fmt.Errorf("could not determine datacenter for host '%s'", esx.Name())
}

// GetHostDatastore returns the name of the first VMFS datastore that is actually mounted
// on the given ESXi host. It queries the host directly via esxcli (storage filesystem list)
// to get the real-time mounted volumes, bypassing vCenter's potentially-stale cache.
// The host object already carries the vim25 client, so no separate vmware.Client is needed.
func GetHostDatastore(ctx context.Context, esxi *object.HostSystem) (string, error) {
	executor, err := esx.NewExecutor(ctx, esxi.Client(), esxi.Reference())
	if err != nil {
		return "", fmt.Errorf("failed to create esxcli executor for host %s: %w", esxi.Name(), err)
	}
	res, err := executor.Run(ctx, []string{"storage", "filesystem", "list"})
	if err != nil {
		return "", fmt.Errorf("failed to list filesystems on host %s: %w", esxi.Name(), err)
	}
	rows := res.Values

	// Build a set of VMFS volume names that are actually mounted on this host.
	mountedVMFS := map[string]bool{}
	for _, row := range rows {
		fsType := row.Value("Type")
		mounted := row.Value("Mounted")
		name := row.Value("VolumeName")
		if strings.HasPrefix(fsType, "VMFS") && mounted == "true" && name != "" {
			mountedVMFS[name] = true
		}
	}

	// Pick the first datastore from vCenter's list that is confirmed mounted.
	pc := property.DefaultCollector(esxi.Client())
	var hostMo mo.HostSystem
	if err = pc.RetrieveOne(ctx, esxi.Reference(), []string{"datastore"}, &hostMo); err != nil {
		return "", fmt.Errorf("failed to retrieve host datastores for %s: %w", esxi.Name(), err)
	}
	if len(hostMo.Datastore) == 0 {
		return "", fmt.Errorf("no datastores found on host %s", esxi.Name())
	}

	var datastores []mo.Datastore
	if err = pc.Retrieve(ctx, hostMo.Datastore, []string{"name", "summary.type", "summary.accessible"}, &datastores); err != nil {
		return "", fmt.Errorf("failed to retrieve datastore properties for %s: %w", esxi.Name(), err)
	}

	for _, ds := range datastores {
		if ds.Summary.Type == "VMFS" && ds.Summary.Accessible && mountedVMFS[ds.Name] {
			return ds.Name, nil
		}
	}

	return "", fmt.Errorf("no mounted VMFS datastore found on host %s", esxi.Name())
}

// GetVIBVersion returns the currently installed version of the vmkfstools-wrapper VIB on the given ESXi host.
// Returns empty string if the VIB is not installed.
func GetVIBVersion(ctx context.Context, client vmware.Client, esxi *object.HostSystem) (string, error) {
	r, err := client.RunEsxCommand(ctx, esxi, []string{"software", "vib", "get", "-n", vibName})
	if err != nil {
		vFault, conversionErr := vmware.ErrToFault(err)
		if conversionErr != nil {
			return "", err
		}
		if vFault != nil {
			for _, m := range vFault.ErrMsgs {
				if strings.Contains(m, "[NoMatchError]") {
					return "", nil
				}
			}
		}
		return "", err
	}

	if len(r) == 0 {
		return "", nil
	}
	return r[0].Value("Version"), err
}

// GetLoadedVIBVersion returns the version of the VIB plugin currently loaded in hostd's memory.
// This only changes when hostd is restarted, unlike GetVIBVersion which reads the on-disk VIB database.
// Returns empty string if the version command is not available (old VIB or no VIB loaded).
func GetLoadedVIBVersion(ctx context.Context, client vmware.Client, esxi *object.HostSystem) (string, error) {
	r, err := client.RunEsxCommand(ctx, esxi, []string{"vmkfstools", "version"})
	if err != nil {
		return "", err
	}
	if len(r) == 0 {
		return "", fmt.Errorf("vmkfstools version command returned empty response")
	}
	msg := r[0].Value("message")
	var versionInfo struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal([]byte(msg), &versionInfo); err != nil {
		return "", fmt.Errorf("failed to parse VIB version response %q: %w", msg, err)
	}
	return versionInfo.Version, nil
}

func uploadVib(ctx context.Context, client vmware.Client, dc *object.Datacenter, dsName, localVibPath string) (string, error) {
	ds, err := client.GetDatastore(ctx, dc, dsName)
	if err != nil {
		return "", fmt.Errorf("failed to get datastore for VIB upload: %w", err)
	}
	destFilename := vibName + ".vib"
	if err = ds.UploadFile(ctx, localVibPath, destFilename, nil); err != nil {
		return "", fmt.Errorf("failed to upload %s: %w", localVibPath, err)
	}
	return fmt.Sprintf("/vmfs/volumes/%s/%s", dsName, destFilename), nil
}

func installVib(ctx context.Context, client vmware.Client, esx *object.HostSystem, vibPath string) error {
	r, err := client.RunEsxCommand(ctx, esx, []string{"software", "vib", "install", "-f", "1", "-v", vibPath})
	if err != nil {
		return err
	}

	if len(r) > 0 {
		if vibsSkipped, ok := r[0]["VIBsSkipped"]; ok && len(vibsSkipped) > 0 {
			klog.Infof("VIB already installed on ESXi host, skipping: %s", vibsSkipped[0])
			return nil
		}

		if vibsInstalled, ok := r[0]["VIBsInstalled"]; ok && len(vibsInstalled) > 0 {
			return nil
		}
	}

	return fmt.Errorf("unexpected VIB install response format, cannot confirm success: %v", r)
}

// ShouldSkipVIBCheck returns true if the last check was within the cache duration
func ShouldSkipVIBCheck(lastTransitionTime time.Time, cacheDuration time.Duration) bool {
	if lastTransitionTime.IsZero() {
		return false
	}
	return time.Since(lastTransitionTime) < cacheDuration
}

// RestartHostd connects to an ESXi host via SSH and restarts the hostd service
// to load a newly installed VIB into memory.
func RestartHostd(ctx context.Context, hostIP string, sshConfig *ssh.ClientConfig) error {
	addr := fmt.Sprintf("%s:22", hostIP)

	dialer := &net.Dialer{}
	netConn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("SSH connection failed: %w", err)
	}
	defer func() {
		if cerr := netConn.Close(); cerr != nil {
			klog.V(2).Infof("SSH netConn close failed for %s: %v", hostIP, cerr)
		}
	}()

	cc, chans, reqs, err := ssh.NewClientConn(netConn, addr, sshConfig)
	if err != nil {
		return fmt.Errorf("SSH handshake failed: %w", err)
	}
	sshClient := ssh.NewClient(cc, chans, reqs)
	defer func() {
		if cerr := sshClient.Close(); cerr != nil {
			klog.Warningf("SSH client close failed for %s: %v", hostIP, cerr)
		}
	}()

	session, err := sshClient.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer func() {
		if sessErr := session.Close(); sessErr != nil {
			klog.V(2).Infof("SSH session close warning for %s: %v", hostIP, sessErr)
		}
	}()

	cmdCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmdDone := make(chan error, 1)
	go func() {
		_, err := session.CombinedOutput("/etc/init.d/hostd restart")
		cmdDone <- err
	}()

	select {
	case err := <-cmdDone:
		if err != nil {
			return fmt.Errorf("hostd restart command failed: %w", err)
		}
		return nil
	case <-cmdCtx.Done():
		_ = session.Close()
		return fmt.Errorf("hostd restart timed out after 30s")
	}
}
