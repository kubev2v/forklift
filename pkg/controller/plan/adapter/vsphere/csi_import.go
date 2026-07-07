package vsphere

import (
	"context"
	"fmt"

	forklift "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/storage/resolver"
	"github.com/kubev2v/forklift/pkg/storage/resolver/hpe"
)

type diskBackingResolver interface {
	getDiskBacking(ctx context.Context, vmId, diskFile string) (*resolver.DiskBacking, error)
}

type vsphereDiskBackingResolver struct {
	planCtx *plancontext.Context
}

func (r *vsphereDiskBackingResolver) getDiskBacking(ctx context.Context, vmId, diskFile string) (*resolver.DiskBacking, error) {
	c := &Client{Context: r.planCtx}
	if err := c.connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to vSphere for disk backing detection: %w", err)
	}
	defer c.Close()
	return c.getDiskBacking(ctx, vmId, diskFile)
}

func newCsiImportPlugin(product forklift.StorageVendorProduct, host, user, pass string, skipSSL bool) (resolver.CsiImportPlugin, error) {
	switch product {
	case forklift.StorageVendorProductPrimera3Par:
		return hpe.NewHpeImporter(host, user, pass, skipSSL)
	default:
		return nil, fmt.Errorf("CSI import not supported for vendor %q", product)
	}
}
