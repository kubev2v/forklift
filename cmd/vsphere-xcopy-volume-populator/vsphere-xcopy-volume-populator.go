package main

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/client-go/util/cert"

	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/flashsystem"
	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/infinibox"
	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/ontap"
	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/populator"
	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/powerflex"
	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/powermax"
	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/powerstore"
	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/primera3par"
	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/pure"
	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/vantara"
	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/version"

	forklift "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

var (
	crName                     string
	crNamespace                string
	pvcSize                    string
	ownerUID                   string
	ownerName                  string
	secretName                 string
	sourceVmId                 string
	sourceVMDKFile             string
	targetNamespace            string
	storageVendor              string
	storageHostname            string
	storageUsername            string
	storagePassword            string
	storageToken               string
	storageSkipSSLVerification string
	vsphereHostname            string
	vsphereUsername            string
	vspherePassword            string
	esxiCloneMethod            string
	sshTimeoutSeconds          int

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
	klog.Info(version.Get())

	var storageApi populator.StorageApi
	product := forklift.StorageVendorProduct(storageVendor)
	switch product {
	case forklift.StorageVendorProductVantara:
		sm, err := vantara.NewVantaraClonner(storageHostname, storageUsername, storagePassword)
		if err != nil {
			klog.Fatalf("failed to initialize Vantara storage mapper with %s", err)
		}
		storageApi = &sm
	case forklift.StorageVendorProductOntap:
		sm, err := ontap.NewNetappClonner(storageHostname, storageUsername, storagePassword)
		if err != nil {
			klog.Fatalf("failed to initialize Ontap storage mapper with %s", err)
		}
		storageApi = &sm
	case forklift.StorageVendorProductFlashSystem:
		sm, err := flashsystem.NewFlashSystemClonner(storageHostname, storageUsername, storagePassword, storageSkipSSLVerification == "true")
		if err != nil {
			klog.Fatalf("failed to initialize flashsystem storage mapper with %s", err)
		}
		storageApi = &sm
	case forklift.StorageVendorProductPrimera3Par:
		sm, err := primera3par.NewPrimera3ParClonner(
			storageHostname, storageUsername, storagePassword, storageSkipSSLVerification == "true")
		if err != nil {
			klog.Fatalf("failed to initialize primera3par clonner with %s", err)
		}
		storageApi = &sm
	case forklift.StorageVendorProductPureFlashArray:
		sm, err := pure.NewFlashArrayClonner(
			storageHostname, storageUsername, storagePassword, storageToken, storageSkipSSLVerification == "true", os.Getenv(pure.ClusterPrefixEnv))
		if err != nil {
			klog.Fatalf("failed to initialize Pure FlashArray clonner with %s", err)
		}
		storageApi = &sm
	case forklift.StorageVendorProductPowerFlex:
		systemId := os.Getenv(powerflex.SYSTEM_ID_ENV_KEY)
		sm, err := powerflex.NewPowerflexClonner(
			storageHostname, storageUsername, storagePassword, storageSkipSSLVerification == "true", systemId)
		if err != nil {
			klog.Fatalf("failed to initialize PowerFlex clonner with %s", err)
		}
		storageApi = &sm
	case forklift.StorageVendorProductPowerMax:
		sm, err := powermax.NewPowermaxClonner(
			storageHostname, storageUsername, storagePassword, storageSkipSSLVerification == "true")
		if err != nil {
			klog.Fatalf("failed to initialize PowerMax clonner with %s", err)
		}
		storageApi = &sm
	case forklift.StorageVendorProductPowerStore:
		sm, err := powerstore.NewPowerstoreClonner(
			storageHostname, storageUsername, storagePassword, storageSkipSSLVerification == "true")
		if err != nil {
			klog.Fatalf("failed to initialize PowerStore clonner with %s", err)
		}
		storageApi = &sm
	case forklift.StorageVendorProductInfinibox:
		sm, err := infinibox.NewInfiniboxClonner(
			storageHostname, storageUsername, storagePassword, storageSkipSSLVerification == "true")
		if err != nil {
			klog.Fatalf("failed to initialize Infinibox clonner with %s", err)
		}
		storageApi = &sm
	default:
		klog.Fatalf("Unsupported storage vendor %s use one of %v",
			storageVendor, forklift.StorageVendorProducts())
	}

	// validations
	_, err := populator.ParseVmdkPath(sourceVMDKFile)
	if err != nil {
		klog.Fatal(err)
	}

	// Prepare SSH config if needed
	var sshConfig *populator.SSHConfig
	methodStr := strings.ToLower(strings.TrimSpace(esxiCloneMethod))
	if methodStr == string(populator.CloneMethodSSH) {
		sshPrivateKey, sshPublicKey, err := getSSHKeysFromEnvironment()
		if err != nil {
			klog.Fatalf("Failed to get SSH keys from environment: %s", err)
		}
		klog.Infof("SSH keys retrieved, private key length: %d, public key length: %d", len(sshPrivateKey), len(sshPublicKey))
		sshConfig = &populator.SSHConfig{
			UseSSH:         true,
			PrivateKey:     sshPrivateKey,
			PublicKey:      sshPublicKey,
			TimeoutSeconds: sshTimeoutSeconds,
		}
	}

	// Select the appropriate populator based on disk type
	p, err := populator.NewPopulator(
		storageApi,
		vsphereHostname,
		vsphereUsername,
		vspherePassword,
		sourceVmId,
		sourceVMDKFile,
		sshConfig,
	)
	if err != nil {
		klog.Fatalf("Failed to initialize populator: %s", err)
	}

	pv, err := getPv(clientSet, targetNamespace, ownerName)
	if err != nil {
		klog.Fatalf("Failed to fetch the volume handle details from the target pvc %s: %s", ownerName, err)
	}

	progressCounter, xcopyUsedGauge, err := setupTracingMetrics()
	if err != nil {
		klog.Fatal(err)
	}

	progressCh := make(chan uint64)
	xCopyUsedCh := make(chan int)
	quitCh := make(chan error)

	log := klog.Background().WithName("copy-offload").WithValues("pvc", ownerName, "source_vmdk", sourceVMDKFile)
	cloneLog := log.WithName("xcopy").WithName("clone")
	log.Info("copy-offload started")

	hll := populator.NewHostLeaseLocker(clientSet)
	go p.Populate(sourceVmId, sourceVMDKFile, pv, hll, progressCh, xCopyUsedCh, quitCh)

	for {
		select {
		case p := <-progressCh:
			cloneLog.Info("clone progress", "progress", p)
			metric := dto.Metric{}
			if err := progressCounter.WithLabelValues(ownerUID).Write(&metric); err != nil {
				klog.Error(err)
			} else if float64(p) > metric.Counter.GetValue() {
				progressCounter.WithLabelValues(ownerUID).Add(float64(p) - metric.Counter.GetValue())
			}
		case c := <-xCopyUsedCh:
			cloneLog.Info("xcopy", "xcopyUsed", c)
			metric := dto.Metric{}
			if err := xcopyUsedGauge.WithLabelValues(ownerUID).Write(&metric); err != nil {
				log.Error(err, "failed to write xcopy used gauge")
			} else {
				xcopyUsedGauge.WithLabelValues(ownerUID).Set(float64(c))
			}
		case q := <-quitCh:
			if q != nil {
				log.Error(q, "copy-offload failed")
				klog.Fatal(q)
			}
			log.Info("copy-offload finished")
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

// getPv extract the volume handle from the PVC. To detect the volume of the said targetPVC we need
// to locate the created volume on the PVC. There is a  chance where the volume details are listed on the
// "prime-{ORIG_PVC_NAME}" PVC because when the controller pod is handling it, the pvc prime should be bounded
// to popoulator pod. However it is not guarnteed to be bounded at that stage and it may take time
func getPv(kubeClient *kubernetes.Clientset, targetNamespace, targetPVC string) (populator.PersistentVolume, error) {
	pvc, err := kubeClient.CoreV1().PersistentVolumeClaims(targetNamespace).Get(context.Background(), targetPVC, metav1.GetOptions{})
	if err != nil {
		return populator.PersistentVolume{}, fmt.Errorf("failed to fetch the the target persistent volume claim %q %w", pvc.Name, err)
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
			return populator.PersistentVolume{}, fmt.Errorf("failed to fetch the the target persistent volume claim %q %w", primePVC.Name, err)
		}

		if primePVC.Spec.VolumeName == "" {
			return populator.PersistentVolume{}, fmt.Errorf("the volume name is not found on the prime volume claim %q", primePVC.Name)
		}
		volumeName = primePVC.Spec.VolumeName
	}

	pv, err := kubeClient.CoreV1().PersistentVolumes().Get(context.Background(), volumeName, metav1.GetOptions{})
	if err != nil {
		return populator.PersistentVolume{}, fmt.Errorf("failed to fetch the target volume details %w", err)
	}
	return populator.PersistentVolume{
		Name:             pv.Name,
		VolumeHandle:     pv.Spec.CSI.VolumeHandle,
		VolumeAttributes: pv.Spec.CSI.VolumeAttributes}, nil
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
	flag.StringVar(&sourceVmId, "source-vm-id", "", "VM object id in vsphere")
	flag.StringVar(&sourceVMDKFile, "source-vmdk", "", "File name to populate")
	flag.StringVar(&storageVendor, "storage-vendor-product", os.Getenv("STORAGE_VENDOR"), "The storage vendor to work with. Current values: [flashsystem, infinibox, ontap, powerflex, powermax, powerstore, primera3par, pureFlashArray, vantara]")
	flag.StringVar(&targetNamespace, "target-namespace", "", "Contents to populate file with")
	flag.StringVar(&storageHostname, "storage-hostname", os.Getenv("STORAGE_HOSTNAME"), "The storage vendor api hostname")
	flag.StringVar(&storageUsername, "storage-username", os.Getenv("STORAGE_USERNAME"), "The storage vendor api username")
	flag.StringVar(&storagePassword, "storage-password", os.Getenv("STORAGE_PASSWORD"), "The storage vendor api password")
	flag.StringVar(&storageToken, "storage-token", os.Getenv("STORAGE_TOKEN"), "The storage vendor api token (alternative to username/password)")
	flag.StringVar(&storageSkipSSLVerification, "storage-skip-ssl-verification", os.Getenv("STORAGE_SKIP_SSL_VERIFICATION"), "skip the storage ssl verification")
	flag.StringVar(&vsphereHostname, "vsphere-hostname", os.Getenv("GOVMOMI_HOSTNAME"), "vSphere's API hostname")
	flag.StringVar(&vsphereUsername, "vsphere-username", os.Getenv("GOVMOMI_USERNAME"), "vSphere's API username")
	flag.StringVar(&vspherePassword, "vsphere-password", os.Getenv("GOVMOMI_PASSWORD"), "vSphere's API password")
	flag.StringVar(&esxiCloneMethod, "esxi-clone-method", os.Getenv("ESXI_CLONE_METHOD"), "ESXi clone method: 'vib' (default) or 'ssh'")
	flag.IntVar(&sshTimeoutSeconds, "ssh-timeout-seconds", 30, "SSH timeout in seconds for ESXi operations (default: 30)")
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	// Metrics args
	flag.StringVar(&httpEndpoint, "http-endpoint", "", "The TCP network address where the HTTP server for diagnostics, including metrics and leader election health check, will listen (example: `:8080`). The default is empty string, which means the server is disabled.")
	flag.StringVar(&metricsPath, "metrics-path", "/metrics", "The HTTP path where prometheus metrics will be exposed. Default is `/metrics`.")
	// Other args
	flag.BoolVar(&showVersion, "version", false, "display the version string")
	flag.Parse()

	if showVersion {
		klog.Info(version.Get())
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
		case "source-vm-id", "source-vmdk", "target-pvc", "storage-vendor":
			if f.Value.String() == "" {
				missingFlags = true
				klog.Errorf("missing value for mandatory flag --%s", f.Name)
			}
		case "storage-hostname", "vsphere-hostname", "vsphere-username", "vsphere-password":
			if f.Value.String() == "" {
				missingFlags = true
				klog.Errorf("missing value for flag --%s", f.Name)
			}
		}
	})

	// Validate storage authentication: either token or username/password
	if err := validateStorageAuthentication(storageToken, storageUsername, storagePassword); err != nil {
		missingFlags = true
		klog.Error(err)
	}

	if missingFlags {
		os.Exit(2)
	}
}

// validateStorageAuthentication validates that either token or username/password credentials are provided
// Returns nil if valid, error if invalid
func validateStorageAuthentication(token, username, password string) error {
	if token != "" {
		// Token-based authentication: only token is required
		klog.Infof("Using token-based authentication for storage")
		return nil
	}

	// Username/password authentication: both username and password are required
	if username == "" || password == "" {
		return fmt.Errorf("either STORAGE_TOKEN or both STORAGE_USERNAME and STORAGE_PASSWORD must be provided")
	}

	klog.Infof("Using username/password authentication for storage")
	return nil
}

func startMetricsServer(certFile, keyFile string) {
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		cfg := tls.Config{MinVersion: tls.VersionTLS12}
		server := http.Server{
			Addr:      ":8443",
			TLSConfig: &cfg,
		}
		klog.Info("Starting metrics server")
		if err := server.ListenAndServeTLS(certFile, keyFile); err != nil {
			klog.Fatal("Error starting Prometheus endpoint: ", err)
		}
	}()
}

func setupTracingMetrics() (*prometheus.CounterVec, *prometheus.GaugeVec, error) {
	certsDirectory, err := os.MkdirTemp("", "certsdir")
	if err != nil {
		return nil, nil, err
	}

	certBytes, keyBytes, err := cert.GenerateSelfSignedCertKey("", nil, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("Error generating cert for prometheus: %w", err)
	}

	certFile := path.Join(certsDirectory, "tls.crt")
	if err = os.WriteFile(certFile, certBytes, 0600); err != nil {
		return nil, nil, fmt.Errorf("Error writing cert file: %w", err)
	}

	keyFile := path.Join(certsDirectory, "tls.key")
	if err = os.WriteFile(keyFile, keyBytes, 0600); err != nil {
		return nil, nil, fmt.Errorf("Error writing key file: %w", err)
	}

	// Register metrics
	progressCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "vsphere_xcopy_volume_populator_progress",
			Help: "Progress of vsphere XCOPY volume population",
		},
		[]string{"ownerUID"},
	)

	xcopyUsedGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "vsphere_xcopy_volume_populator_xcopy_used",
			Help: "Indicates whether XCOPY was used for cloning (0=no, 1=yes)",
		},
		[]string{"ownerUID"},
	)

	// Register both to the default registry
	if err := prometheus.Register(progressCounter); err != nil {
		return nil, nil, fmt.Errorf("Progress counter not registered: %w", err)
	}
	if err := prometheus.Register(xcopyUsedGauge); err != nil {
		return nil, nil, fmt.Errorf("XCOPY used gauge not registered: %w", err)
	}

	startMetricsServer(certFile, keyFile)

	return progressCounter, xcopyUsedGauge, nil
}

// getSSHKeysFromEnvironment retrieves SSH keys from environment variables set by the provider controller
func getSSHKeysFromEnvironment() ([]byte, []byte, error) {
	sshPrivateKeyEnv := os.Getenv("SSH_PRIVATE_KEY")
	sshPublicKeyEnv := os.Getenv("SSH_PUBLIC_KEY")

	if sshPrivateKeyEnv == "" || sshPublicKeyEnv == "" {
		return nil, nil, fmt.Errorf("SSH keys not found in environment variables - ensure provider controller has set SSH_PRIVATE_KEY and SSH_PUBLIC_KEY")
	}

	privateKeyBytes, err := base64.StdEncoding.DecodeString(sshPrivateKeyEnv)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode SSH private key from environment: %w", err)
	}

	publicKeyBytes, err := base64.StdEncoding.DecodeString(sshPublicKeyEnv)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode SSH public key from environment: %w", err)
	}

	klog.Infof("Successfully retrieved SSH keys from environment variables")
	return privateKeyBytes, publicKeyBytes, nil
}
