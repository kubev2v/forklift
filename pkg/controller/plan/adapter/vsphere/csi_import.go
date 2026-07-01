package vsphere

import (
	"fmt"
	"strings"

	forklift "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/storage/resolver"
	"github.com/kubev2v/forklift/pkg/storage/resolver/hpe"
	"github.com/kubev2v/forklift/pkg/storage/resolver/ontap"
	"github.com/kubev2v/forklift/pkg/storage/resolver/pure"
)

// newCsiImportPlugin instantiates the CsiImportPlugin for the given storage vendor.
// This is the equivalent of xcopy's main() vendor switch — the only place that imports
// vendor sub-packages. Adding a new vendor = new sub-package + one case here.
func newCsiImportPlugin(product forklift.StorageVendorProduct, host, user, pass string, skipSSL bool, secretData map[string][]byte) (resolver.CsiImportPlugin, error) {
	switch product {
	case forklift.StorageVendorProductPrimera3Par:
		return hpe.NewHpeImporter(host, user, pass, skipSSL)
	case forklift.StorageVendorProductPureFlashArray:
		return pure.NewPureImporter(host, user, pass, skipSSL)
	case forklift.StorageVendorProductOntap:
		svm := string(secretData["ONTAP_SVM"])
		backendUUID := string(secretData["TRIDENT_BACKEND_UUID"])
		var missing []string
		if svm == "" {
			missing = append(missing, "ONTAP_SVM")
		}
		if backendUUID == "" {
			missing = append(missing, "TRIDENT_BACKEND_UUID")
		}
		if len(missing) > 0 {
			return nil, fmt.Errorf("ONTAP storage secret missing required keys: %s", strings.Join(missing, ", "))
		}
		driverType := string(secretData["TRIDENT_DRIVER"])
		return ontap.NewOntapImporter(host, user, pass, svm, backendUUID, driverType, skipSSL)
	default:
		return nil, fmt.Errorf("CSI import not supported for vendor %q", product)
	}
}
