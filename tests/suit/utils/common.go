package utils

import (
	"context"
	"fmt"
	"github.com/onsi/ginkgo"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"net/http"
)

// cdi-file-host pod/service relative values
const (
	//RegistryHostName provides a deploymnet and service name for registry
	RegistryHostName = "cdi-docker-registry-host"
	// FileHostName provides a deployment and service name for tests
	FileHostName = "cdi-file-host"
	// FileHostS3Bucket provides an S3 bucket name for tests (e.g. http://<serviceIP:port>/FileHostS3Bucket/image)
	FileHostS3Bucket = "images"
	// AccessKeyValue provides a username to use for http and S3 (see hack/build/docker/cdi-func-test-file-host-http/htpasswd)
	AccessKeyValue = "admin"
	// SecretKeyValue provides a password to use for http and S3 (see hack/build/docker/cdi-func-test-file-host-http/htpasswd)
	SecretKeyValue = "password"
	// HttpAuthPort provides a cdi-file-host service auth port for tests
	HTTPAuthPort = 81
	// HttpNoAuthPort provides a cdi-file-host service no-auth port for tests, requires AccessKeyValue and SecretKeyValue
	HTTPNoAuthPort = 80
	// HTTPRateLimitPort provides a cdi-file-host service rate limit port for tests, speed is limited to 25k/s to allow for testing slow connection behavior. No auth.
	HTTPRateLimitPort = 82
	// S3Port provides a cdi-file-host service S3 port, requires AccessKey and SecretKeyValue
	S3Port = 9000
	// HTTPSPort is the https port of cdi-file-host
	HTTPSNoAuthPort = 443
	// RegistryCertConfigMap is the ConfigMap where the cert for the docker registry is stored
	RegistryCertConfigMap = "cdi-docker-registry-host-certs"
	// FileHostCertConfigMap is the ConfigMap where the cert fir the file host is stored
	FileHostCertConfigMap = "cdi-file-host-certs"
	// ImageIOCertConfigMap is the ConfigMap where the cert fir the file host is stored
	ImageIOCertConfigMap = "imageio-certs"
)

var (
	// DefaultStorageClass the default storage class used in tests
	DefaultStorageClass *storagev1.StorageClass
	// DefaultStorageClassCsiDriver the default storage class CSI driver if it exists.
	DefaultStorageClassCsiDriver *storagev1.CSIDriver

	// NfsService is the service in the cdi namespace that will be created if KUBEVIRT_STORAGE=nfs
	NfsService *corev1.Service
	nfsChecked bool
	// DefaultStorageCSIRespectsFsGroup is true if the default storage class is CSI and respects fsGroup, false other wise.
	DefaultStorageCSIRespectsFsGroup bool
)

// ClientsIface is the clients interface
type ClientsIface interface {
	K8s() *kubernetes.Clientset
}

// GetDefaultStorageClass return the storage class which is marked as default in the cluster
func GetDefaultStorageClass(client *kubernetes.Clientset) *storagev1.StorageClass {
	storageclasses, err := client.StorageV1().StorageClasses().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		ginkgo.Fail("Unable to list storage classes")
		return nil
	}
	for _, storageClass := range storageclasses.Items {
		if storageClass.Annotations["storageclass.kubernetes.io/is-default-class"] == "true" {
			return &storageClass
		}
	}
	ginkgo.Fail("Unable to find default storage classes")
	return nil
}

func getDefaultStorageClassCsiDriver(client *kubernetes.Clientset) *storagev1.CSIDriver {
	if DefaultStorageClass != nil {
		csidrivers, err := client.StorageV1().CSIDrivers().List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			ginkgo.Fail(fmt.Sprintf("Unable to get csi driver: %v", err))
		}
		for _, driver := range csidrivers.Items {
			if driver.Name == DefaultStorageClass.Provisioner {
				return &driver
			}
		}
	}
	return nil
}

func isDefaultStorageClassCSIRespectsFsGroup() bool {
	return DefaultStorageClassCsiDriver != nil && DefaultStorageClassCsiDriver.Spec.FSGroupPolicy != nil && *DefaultStorageClassCsiDriver.Spec.FSGroupPolicy != storagev1.NoneFSGroupPolicy
}

// CacheTestsData fetch and cache data required for tests
func CacheTestsData(client *kubernetes.Clientset, cdiNs string) {
	if DefaultStorageClass == nil {
		DefaultStorageClass = GetDefaultStorageClass(client)
	}
	DefaultStorageClassCsiDriver = getDefaultStorageClassCsiDriver(client)
	DefaultStorageCSIRespectsFsGroup = isDefaultStorageClassCSIRespectsFsGroup()

	if !nfsChecked {
		NfsService = getNfsService(client, cdiNs)
		nfsChecked = true
	}
}

func getNfsService(client *kubernetes.Clientset, cdiNs string) *corev1.Service {
	for _, ns := range []string{cdiNs, "nfs-csi"} {
		service, err := client.CoreV1().Services(ns).Get(context.TODO(), "nfs-service", metav1.GetOptions{})
		if err == nil {
			return service
		}
	}
	return nil
}

//IsOpenshift checks if we are on OpenShift platform
func IsOpenshift(client kubernetes.Interface) bool {
	//OpenShift 3.X check
	result := client.Discovery().RESTClient().Get().AbsPath("/oapi/v1").Do(context.TODO())
	var statusCode int
	result.StatusCode(&statusCode)

	if result.Error() == nil {
		// It is OpenShift
		if statusCode == http.StatusOK {
			return true
		}
	} else {
		// Got 404 so this is not Openshift 3.X, let's check OpenShift 4
		result = client.Discovery().RESTClient().Get().AbsPath("/apis/route.openshift.io").Do(context.TODO())
		var statusCode int
		result.StatusCode(&statusCode)

		if result.Error() == nil {
			// It is OpenShift
			if statusCode == http.StatusOK {
				return true
			}
		}
	}

	return false
}
