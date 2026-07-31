package vsphere

import (
	"fmt"
	"strconv"
	"strings"

	forklift "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/storage/resolver"
	"github.com/kubev2v/forklift/pkg/storage/resolver/hpe"
	"github.com/kubev2v/forklift/pkg/storage/resolver/ontap"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// newCsiImportPlugin instantiates the CsiImportPlugin for the given storage vendor.
// This is the equivalent of xcopy's main() vendor switch — the only place that imports
// vendor sub-packages. Adding a new vendor = new sub-package + one case here.
func newCsiImportPlugin(product forklift.StorageVendorProduct, secretData map[string][]byte, k8sClient client.Client, storageClass string) (resolver.CsiImportPlugin, error) {
	host := string(secretData["STORAGE_HOSTNAME"])
	user := string(secretData["STORAGE_USERNAME"])
	pass := string(secretData["STORAGE_PASSWORD"])
	skipSSL, err := strconv.ParseBool(string(secretData["STORAGE_SKIP_SSL_VERIFICATION"]))
	if err != nil {
		klog.V(2).InfoS("CSI import: invalid or missing STORAGE_SKIP_SSL_VERIFICATION, defaulting to false")
		skipSSL = false
	}
	var missing []string
	if host == "" {
		missing = append(missing, "STORAGE_HOSTNAME")
	}
	if user == "" {
		missing = append(missing, "STORAGE_USERNAME")
	}
	if pass == "" {
		missing = append(missing, "STORAGE_PASSWORD")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("storage secret missing required keys: %s", strings.Join(missing, ", "))
	}

	switch product {
	case forklift.StorageVendorProductPrimera3Par:
		return hpe.NewHpeImporter(host, user, pass, skipSSL)
	case forklift.StorageVendorProductOntap:
		svm := string(secretData["ONTAP_SVM"])
		backendUUID := string(secretData["TRIDENT_BACKEND_UUID"])
		driverType := string(secretData["TRIDENT_DRIVER"])
		return ontap.NewOntapImporter(host, user, pass, svm, backendUUID, driverType, skipSSL, k8sClient, storageClass)
	default:
		return nil, fmt.Errorf("CSI import not supported for vendor %q", product)
	}
}
