package vsphere

import (
	"fmt"

	forklift "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/storage/resolver"
	"github.com/kubev2v/forklift/pkg/storage/resolver/hpe"
	"github.com/kubev2v/forklift/pkg/storage/resolver/pure"
)

// newCsiImportPlugin instantiates the CsiImportPlugin for the given storage vendor.
// This is the equivalent of xcopy's main() vendor switch — the only place that imports
// vendor sub-packages. Adding a new vendor = new sub-package + one case here.
func newCsiImportPlugin(product forklift.StorageVendorProduct, host, user, pass string, skipSSL bool) (resolver.CsiImportPlugin, error) {
	switch product {
	case forklift.StorageVendorProductPrimera3Par:
		return hpe.NewHpeImporter(host, user, pass, skipSSL)
	case forklift.StorageVendorProductPureFlashArray:
		return pure.NewPureImporter(host, user, pass, skipSSL)
	default:
		return nil, fmt.Errorf("CSI import not supported for vendor %q", product)
	}
}
