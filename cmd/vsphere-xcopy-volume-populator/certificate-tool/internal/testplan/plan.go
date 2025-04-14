package testplan

import (
	"certificate-tool/internal/utils"
	"certificate-tool/pkg/config"
	"certificate-tool/pkg/storage"
	"context"
	"fmt"
	"k8s.io/klog/v2"
	"time"

	"gopkg.in/yaml.v3"

	"k8s.io/client-go/kubernetes"
)

// TestPlan aggregates multiple test cases under a VM image.
type TestPlan struct {
	StorageVendorProduct string                `yaml:"storageVendorProduct"`
	TestCases            []TestCase            `yaml:"tests"`
	Namespace            string                `yaml:"-"`
	StorageClass         string                `yaml:"-"`
	ClientSet            *kubernetes.Clientset `yaml:"-"`
	VSphereURL           string                `yaml:"-"`
	VSphereUser          string                `yaml:"-"`
	VSpherePassword      string                `yaml:"-"`
	Datacenter           string                `yaml:"-"`
	Datastore            string                `yaml:"-"`
	ResourcePool         string                `yaml:"-"`
	HostName             string                `yaml:"hostName"`
	// New fields for VMDK download URL, local VMDK path, and ISO path
	VmdkDownloadURL string
	LocalVmdkPath   string
	IsoPath         string
	AppConfig       *config.Config
}

// Parse unmarshals YAML data into a TestPlan.
func Parse(yamlData []byte) (*TestPlan, error) {
	var tp TestPlan
	if err := yaml.Unmarshal(yamlData, &tp); err != nil {
		return nil, err
	}
	return &tp, nil
}

// Start runs all test cases sequentially, creating PVCs and pods, recording results.
func (tp *TestPlan) Start(ctx context.Context, podImage, pvcYamlPath string) error {
	for i := range tp.TestCases {
		tc := &tp.TestCases[i]
		tc.ClientSet = tp.ClientSet
		tc.Namespace = tp.Namespace
		tc.VSphereURL = tp.VSphereURL
		tc.VSphereUser = tp.VSphereUser
		tc.VSpherePassword = tp.VSpherePassword
		tc.Datacenter = tp.Datacenter
		tc.Datastore = tp.Datastore
		tc.ResourcePool = tp.ResourcePool
		tc.HostName = tp.HostName
		tc.VmdkDownloadURL = tp.VmdkDownloadURL
		tc.IsoPath = tp.IsoPath
		tc.StorageClass = tp.StorageClass
		if tc.LocalVmdkPath == "" {
			tc.LocalVmdkPath = tp.LocalVmdkPath
		}
		start := time.Now()
		if err := tc.Run(ctx, podImage, pvcYamlPath, tp.StorageVendorProduct); err != nil {
			tc.ResultSummary = utils.TestResult{
				Success:       false,
				ElapsedTime:   int64(time.Since(start).Seconds()),
				FailureReason: err.Error(),
			}
			return fmt.Errorf("test %s failed: %w", tc.Name, err)
		}
		tc.ResultSummary.ElapsedTime = int64(time.Since(start).Seconds())
	}
	return nil
}

// FormatOutput returns the marshaled YAML of metadata, image, and test results.
func (tp *TestPlan) FormatOutput() ([]byte, error) {
	output := struct {
		Metadata struct {
			Storage struct {
				storage.Storage
				StorageVendorProduct string `yaml:"storageVendorProduct"`
				ConnectionType       string `yaml:"connectionType"`
			} `yaml:"storage"`
		} `yaml:"metadata"`
		Image string     `yaml:"image"`
		Tests []TestCase `yaml:"tests"`
	}{
		Tests: tp.TestCases,
	}

	c := storage.StorageCredentials{
		Hostname:      tp.AppConfig.StorageURL,
		Username:      tp.AppConfig.StorageUser,
		Password:      tp.AppConfig.StoragePassword,
		SSLSkipVerify: tp.AppConfig.StorageSkipSSLVerification == "true",
		VendorProduct: tp.StorageVendorProduct,
	}
	storageInfo, err := storage.StorageInfo(c)
	if err != nil {
		klog.Errorf("failed to get storage info: %v", err)
	}
	output.Metadata.Storage.Storage = storageInfo
	output.Metadata.Storage.StorageVendorProduct = tp.StorageVendorProduct
	output.Metadata.Storage.ConnectionType = "TODOTODOOOOTODOTODDO"
	return yaml.Marshal(output)
}
