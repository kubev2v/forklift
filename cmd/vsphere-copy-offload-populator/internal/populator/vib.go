package populator

import (
	"context"
	"encoding/json"
	"fmt"

	hversion "github.com/hashicorp/go-version"
	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/vmware"
	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/version"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25/mo"
	"k8s.io/klog/v2"
)

// validateVibVersion checks the installed VIB version; failures are advisory (logged, not fatal) except a malformed required-version string.
func validateVibVersion(ctx context.Context, client vmware.Client, host *object.HostSystem) (string, error) {
	log := klog.FromContext(ctx)

	r, err := client.RunEsxCommand(ctx, host, []string{"vmkfstools", "version"})
	if err != nil {
		log.V(0).Info("VIB version check unavailable — version command not supported on this host, skipping validation. "+
			"Install or update the VIB using the vib-installer tool if xcopy migrations fail",
			"host", host.Name(), "required", version.VibVersion, "err", err)
		return "", nil
	}

	response := ""
	if len(r) > 0 {
		response = r[0].Value("message")
	}

	var versionInfo struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal([]byte(response), &versionInfo); err != nil {
		log.V(0).Info("failed to parse VIB version response, skipping validation",
			"host", host.Name(), "err", err)
		return "", nil
	}

	installedVer, err := hversion.NewVersion(versionInfo.Version)
	if err != nil {
		log.V(0).Info("invalid VIB version format, skipping validation",
			"host", host.Name(), "version", versionInfo.Version, "err", err)
		return "", nil
	}

	requiredVer, err := hversion.NewVersion(version.VibVersion)
	if err != nil {
		return "", fmt.Errorf("invalid required VIB version format %q: %w", version.VibVersion, err)
	}

	if installedVer.LessThan(requiredVer) {
		log.V(0).Info("VIB vmkfstools-wrapper version is outdated — xcopy migrations may fail. "+
			"Update the VIB using the vib-installer tool",
			"host", host.Name(), "installed", versionInfo.Version, "required", version.VibVersion)
		return versionInfo.Version, nil
	}

	log.Info("VIB version validated", "host", host.Name(), "installed", versionInfo.Version, "required", version.VibVersion)
	return versionInfo.Version, nil
}

func getHostDC(esx *object.HostSystem) (*object.Datacenter, error) {
	ctx := context.Background()
	hostRef := esx.Reference()
	pc := property.DefaultCollector(esx.Client())
	var hostMo mo.HostSystem
	err := pc.RetrieveOne(context.Background(), hostRef, []string{"parent"}, &hostMo)
	if err != nil {
		klog.Fatalf("failed to retrieve host parent: %v", err)
	}

	parentRef := hostMo.Parent
	var datacenter *object.Datacenter
	currentParentRef := parentRef

	// walk the parents of the host up till the datacenter
	for {
		if currentParentRef.Type == "Datacenter" {
			finder := find.NewFinder(esx.Client(), true)
			datacenter, err = finder.Datacenter(ctx, currentParentRef.String())
			if err != nil {
				return nil, err
			}
			return datacenter, nil
		}

		var genericParentMo mo.ManagedEntity
		err = pc.RetrieveOne(context.Background(), *currentParentRef, []string{"parent"}, &genericParentMo)
		if err != nil {
			klog.Fatalf("failed to retrieve intermediate parent: %v", err)
		}

		if genericParentMo.Parent == nil {
			break
		}
		currentParentRef = genericParentMo.Parent
	}

	return nil, fmt.Errorf("could not determine datacenter for host '%s'.", esx.Name())
}
