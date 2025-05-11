package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/client-go/util/cert"

	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/ontap"
	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/populator"
	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/primera3par"
	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/vantara"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

var version = "unknown"

var (
	crName                     string
	crNamespace                string
	pvcSize                    string
	ownerUID                   string
	ownerName                  string
	secretName                 string
	sourceVMDKFile             string
	targetNamespace            string
	storageVendor              string
	storageHostname            string
	storageUsername            string
	storagePassword            string
	storageSkipSSLVerification string
	vsphereHostname            string
	vsphereUsername            string
	vspherePassword            string

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
	case "vantara":
		sm, err := vantara.NewVantaraClonner(storageHostname, storageUsername, storagePassword)
		if err != nil {
			klog.Fatalf("failed to initialize vantara storage mapper with %s", err)
		}
		storageApi = &sm
	case "ontap":
		sm, err := ontap.NewNetappClonner(storageHostname, storageUsername, storagePassword)
		if err != nil {
			klog.Fatalf("failed to initialize ontap storage mapper with %s", err)
		}
		storageApi = &sm
	case "primera3par":
		sm, err := primera3par.NewPrimera3ParClonner(
			storageHostname, storageUsername, storagePassword, storageSkipSSLVerification == "true")
		if err != nil {
			klog.Fatalf("failed to initialize primera3par clonner with %s", err)
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

	volumeHandle, err := getVolumeHandle(clientSet, targetNamespace, ownerName)
	if err != nil {
		klog.Fatalf("Failed to fetch the volume handle details from the target pvc %s: %s", ownerName, err)
	}

	progressCounter, err := setupTracing()
	if err != nil {
		klog.Fatal(err)
	}

	progressCh := make(chan uint)
	quitCh := make(chan error)

	go p.Populate(sourceVMDKFile, volumeHandle, progressCh, quitCh)

	for {
		select {
		case p := <-progressCh:
			klog.Infof(" progress reported %d%%", p)
			metric := dto.Metric{}
			if err := progressCounter.WithLabelValues(ownerUID).Write(&metric); err != nil {
				klog.Error(err)
			} else if float64(p) > metric.Counter.GetValue() {
				progressCounter.WithLabelValues(ownerUID).Add(float64(p) - metric.Counter.GetValue())
			}
		case q := <-quitCh:
			klog.Infof("channel quit %s", q)
			if q != nil {
				klog.Fatal(q)
			}
			return
		}
	}

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
func getVolumeHandle(kubeClient *kubernetes.Clientset, targetNamespace, targetPVC string) (string, error) {
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
		primePVC, err := kubeClient.CoreV1().PersistentVolumeClaims(targetNamespace).Get(context.Background(), string(primePVCName), metav1.GetOptions{})
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
	flag.StringVar(&ownerUID, "owner-uid", "", "Owner UID, passed by the populator - the PVC ID")
	flag.StringVar(&ownerName, "owner-name", "", "Owner Name, passed by the populator - the PVC Name")
	flag.StringVar(&secretName, "secret-name", "", "Secret name the populator controller uses it to mount env vars from it. Not for use internally")
	flag.StringVar(&sourceVMDKFile, "source-vmdk", "", "File name to populate")
	flag.StringVar(&storageVendor, "storage-vendor-product", "", "The storage vendor to work with. Current values: [vantara, ontap, primera3par]")
	flag.StringVar(&targetNamespace, "target-namespace", "", "Contents to populate file with")
	flag.StringVar(&storageHostname, "storage-hostname", os.Getenv("STORAGE_HOSTNAME"), "The storage vendor api hostname")
	flag.StringVar(&storageUsername, "storage-username", os.Getenv("STORAGE_USERNAME"), "The storage vendor api username")
	flag.StringVar(&storagePassword, "storage-password", os.Getenv("STORAGE_PASSWORD"), "The storage vendor api password")
	flag.StringVar(&storageSkipSSLVerification, "storage-skip-ssl-verification", os.Getenv("STORAGE_SKIP_SSL_VERIFICATION"), "skip the storage ssl verification")
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
			if f.Value.String() == "" {
				missingFlags = true
				klog.Errorf("missing value for flag --%s", f.Name)
			}
		}
	})
	if missingFlags {
		os.Exit(2)
	}
}

func setupTracing() (*prometheus.CounterVec, error) {
	certsDirectory, err := os.MkdirTemp("", "certsdir")
	if err != nil {
		return nil, err
	}

	certBytes, keyBytes, err := cert.GenerateSelfSignedCertKey("", nil, nil)
	if err != nil {
		return nil, fmt.Errorf("Error generating cert for prometheus")
	}

	certFile := path.Join(certsDirectory, "tls.crt")
	if err = os.WriteFile(certFile, certBytes, 0600); err != nil {
		return nil, fmt.Errorf("Error writing cert file: %w", err)
	}

	keyFile := path.Join(certsDirectory, "tls.key")
	if err = os.WriteFile(keyFile, keyBytes, 0600); err != nil {
		return nil, fmt.Errorf("Error writing key file: %w", err)
	}

	go func() {
		http.Handle("/metrics", promhttp.Handler())
		// use minimum TLS 1.2
		cfg := tls.Config{MinVersion: tls.VersionTLS12}
		server := http.Server{
			Addr:      ":8443",
			TLSConfig: &cfg}
		klog.Info("Staring metrics server")
		if err := server.ListenAndServeTLS(certFile, keyFile); err != nil {
			klog.Fatal("Error starting prometheus endpoint: ", err)
		}
	}()

	progressCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "vsphere_xcopy_volume_populator_progress",
			Help: "Progress of vsphere XCOPY volume population",
		},
		[]string{"ownerUID"},
	)
	if err := prometheus.Register(progressCounter); err != nil {
		return nil, fmt.Errorf("Prometheus progress gauge not registered: %w", err)
	}

	return progressCounter, nil

}
