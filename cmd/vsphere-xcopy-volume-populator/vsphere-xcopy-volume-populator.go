package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/ontap"
	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/populator"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

var version = "unknown"

var (
	//secretRef is needed because of lack of control on the populator pod
	// deployment. When/if we gain control on the pod deployment we should mount
	// the secret as env vars. There is an attempt to push that,
	// see https://github.com/kubernetes-csi/lib-volume-populator/pull/171
	crName          string
	crNamespace     string
	pvcSize         string
	ownerUID        string
	secretName      string
	sourceVMDKFile  string
	targetPVC       string
	targetNamespace string
	storageVendor   string
	storageHostname string
	storageUsername string
	storagePassword string
	vsphereHostname string
	vsphereUsername string
	vspherePassword string

	// kube args
	httpEndpoint string
	metricsPath  string
	masterURL    string
	kubeconfig   string

	showVersion bool

	clientSet *kubernetes.Clientset
)

func main() {
	handleArgs()

	var storageApi populator.StorageApi
	switch storageVendor {
	case "ontap":
		sm, err := ontap.NewNetappClonner(storageHostname, storageUsername, storagePassword)
		if err != nil {
			klog.Fatalf("failed to initialize ontap storage mapper with %s", err)
		}
		storageApi = &sm
	default:
		klog.Fatalf("Unsupported storage vendor %s use one of [ontap,]", storageVendor)
	}

	// validations
	_, err := populator.ParseVmdkPath(sourceVMDKFile)
	if err != nil {
		klog.Fatal(err)
	}

	p, err := populator.NewWithRemoteEsxcli(storageApi, vsphereHostname, vsphereUsername, vspherePassword)
	if err != nil {
		klog.Fatalf("Failed to create a remote esxcli populator: %s", err)
	}

	volumeHandle, err := getVolumeHandle(clientSet, targetNamespace, targetNamespace, targetPVC)
	if err != nil {
		klog.Fatalf("Failed to fetch the volume handle details from the target pvc %s: %s", targetPVC, err)
	}
	err = p.Populate(sourceVMDKFile, volumeHandle)
	if err != nil {
		klog.Fatal(err)
	}

	err = annotatingPVC(clientSet, targetPVC, targetNamespace)
	if err != nil {
		klog.Fatal(err)
	}

}

func annotatingPVC(kubeClient *kubernetes.Clientset, pvcName string, pvcNamespace string) error {
	klog.Info("annotating the PVC with progress 100")
	pvc, err := kubeClient.CoreV1().PersistentVolumeClaims(pvcNamespace).Get(context.Background(), pvcName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("Failed to fetch the PVC %w", err)
	}
	// Annotate the PVC
	if pvc.Annotations == nil {
		pvc.Annotations = make(map[string]string)
	}
	pvc.Annotations["copy-offload-progress"] = "100"

	// Update the PVC with the new annotation
	_, err = kubeClient.CoreV1().PersistentVolumeClaims(pvc.Namespace).Update(
		context.Background(),
		pvc,
		metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("Failed to update PVC with xcopy progress: %w", err)
	}
	return nil
}

func newKubeClient(masterURL string, kubeconfig string) (*kubernetes.Clientset, error) {
	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("Failed to create kubernetes config: %w", err)
	}

	coreCfg := rest.CopyConfig(cfg)
	coreCfg.ContentType = runtime.ContentTypeProtobuf
	return kubernetes.NewForConfig(coreCfg)
}

// getVolumeHandle extract the volume handle from the PVC. To detect the volume of the said targetPVC we need
// to locate the created volume on the PVC. There is a  chance where the volume details are listed on the
// "prime-{ORIG_PVC_NAME}" PVC because when the controller pod is handling it, the pvc prime should be bounded
// to popoulator pod. However it is not guarnteed to be bounded at that stage and it may take time
func getVolumeHandle(kubeClient *kubernetes.Clientset, targetNamespace, namespace, targetPVC string) (string, error) {
	pvc, err := kubeClient.CoreV1().PersistentVolumeClaims(targetNamespace).Get(context.Background(), targetPVC, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to fetch the the target persistent volume claim %q %w", pvc.Name, err)
	}
	var volumeName string
	if pvc.Spec.VolumeName != "" {
		volumeName = pvc.Spec.VolumeName
	} else {
		primePVCName := "prime-" + pvc.GetUID()
		klog.Infof("the volume name is not found on the claim %q. Trying the prime pvc %q", pvc.Name, primePVCName)
		// try pvc with postfix "prime" that the populator copies. The prime volume is created in the namespace where the populator controller runs.
		primePVC, err := kubeClient.CoreV1().PersistentVolumeClaims(namespace).Get(context.Background(), string(primePVCName), metav1.GetOptions{})
		if err != nil {
			return "", fmt.Errorf("failed to fetch the the target persistent volume claim %q %w", primePVC.Name, err)
		}

		if primePVC.Spec.VolumeName == "" {
			return "", fmt.Errorf("the volume name is not found on the prime volume claim %q", primePVC.Name)
		}
		volumeName = primePVC.Spec.VolumeName
	}

	pv, err := kubeClient.CoreV1().PersistentVolumes().Get(context.Background(), volumeName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to fetch the the target volume details %w", err)
	}
	klog.Infof("target volume %s volumeHandle %s", pv.Name, pv.Spec.CSI.VolumeHandle)
	return pv.Spec.CSI.VolumeHandle, nil
}

func handleArgs() {
	klog.InitFlags(nil)

	// Populator args
	flag.StringVar(&crName, "cr-name", "", "The Custom Resouce Name")
	flag.StringVar(&crNamespace, "cr-namespace", "", "The Custom Resouce Namespace")
	flag.StringVar(&pvcSize, "pvc-size", "", "The size of the PVC, passed by the populator - unused")
	flag.StringVar(&ownerUID, "owner-uid", "", "Owner UID, passed by the populator - unused")
	flag.StringVar(&sourceVMDKFile, "source-vmdk", "", "File name to populate")
	flag.StringVar(&targetPVC, "target-pvc", "", "Target PVC for population")
	flag.StringVar(&storageVendor, "storage-vendor", "ontap", "The storage vendor to work with. Current values: [ontap,]")
	flag.StringVar(&secretName, "secret-name", "", "The secret holding the credentials for vSphere API and the storage vendor API")
	flag.StringVar(&targetNamespace, "target-namespace", "", "Contents to populate file with")
	flag.StringVar(&storageHostname, "storage-hostname", os.Getenv("STORAGE_HOSTNAME"), "The storage vendor api hostname")
	flag.StringVar(&storageUsername, "storage-username", os.Getenv("STORAGE_USERNAME"), "The storage vendor api username")
	flag.StringVar(&storagePassword, "storage-password", os.Getenv("STORAGE_PASSWORD"), "The storage vendor api password")
	flag.StringVar(&vsphereHostname, "vsphere-hostname", os.Getenv("GOVMOMI_HOSTNAME"), "vSphere's API hostname")
	flag.StringVar(&vsphereUsername, "vsphere-username", os.Getenv("GOVMOMI_USERNAME"), "vSphere's API username")
	flag.StringVar(&vspherePassword, "vsphere-password", os.Getenv("GOVMOMI_PASSWORD"), "vSphere's API password")

	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	// Metrics args
	flag.StringVar(&httpEndpoint, "http-endpoint", "", "The TCP network address where the HTTP server for diagnostics, including metrics and leader election health check, will listen (example: `:8080`). The default is empty string, which means the server is disabled.")
	flag.StringVar(&metricsPath, "metrics-path", "/metrics", "The HTTP path where prometheus metrics will be exposed. Default is `/metrics`.")
	// Other args
	flag.BoolVar(&showVersion, "version", false, "display the version string")
	flag.Parse()

	if showVersion {
		fmt.Println(os.Args[0], version)
		os.Exit(0)
	}

	cs, err := newKubeClient(masterURL, kubeconfig)
	if err != nil {
		klog.Fatalf("Failed to create kubernetes client: %v", err)
	}
	clientSet = cs

	missingFlags := false
	flag.VisitAll(func(f *flag.Flag) {
		switch f.Name {
		case "source-vmdk", "target-pvc", "storage-vendor":
			if f.Value.String() == "" {
				missingFlags = true
				klog.Errorf("missing value for mandatory flag --%s", f.Name)
			}
		case "storage-hostname", "storage-username", "storage-password",
			"vsphere-hostname", "vsphere-username", "vsphere-password":
			if secretName == "" && f.Value.String() == "" {
				missingFlags = true
				klog.Errorf("secret-ref is not set, missing value for flag --%s", f.Name)
			}
		}
	})
	if missingFlags {
		os.Exit(2)
	}

	klog.Infof("Current namespace %s ", targetNamespace)
	if secretName != "" {
		secret, err := clientSet.CoreV1().Secrets(targetNamespace).Get(context.Background(), secretName, metav1.GetOptions{})
		if err != nil {
			klog.Fatalf("fail to fetch the secret %s: %s", secretName, err)
		}

		flag.VisitAll(func(f *flag.Flag) {
			if f.Value.String() != "" {
				return
			}
			name := strings.ReplaceAll(strings.ToUpper(f.Name), "-", "_")
			klog.V(2).Infof("Looking for key %q in the populator secret", name)
			if v, exists := secret.Data[name]; exists {
				f.Value.Set(string(v))
			}
		})
	}
}
