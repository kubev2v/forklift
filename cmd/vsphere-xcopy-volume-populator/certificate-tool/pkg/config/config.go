package config

import (
	"certificate-tool/internal/utils"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	TestNamespace string `yaml:"test-namespace"`
	Kubeconfig    string `yaml:"kubeconfig"`
	SecretName    string `yaml:"secret-name"`
	PvcYamlPath   string `yaml:"pvc-yaml-path"`
	TestLabels    string `yaml:"test-labels"`

	TestImageLabel     string `yaml:"test-image-label"`
	TestPopulatorImage string `yaml:"test-populator-image"`

	StoragePassword  string `yaml:"storage-password"`
	StorageUser      string `yaml:"storage-user"`
	StorageURL       string `yaml:"storage-url"`
	StorageClassName string `yaml:"storage-class-name"`

	VspherePassword string      `yaml:"vsphere-password"`
	VsphereUser     string      `yaml:"vsphere-user"`
	VsphereURL      string      `yaml:"vsphere-url"`
	VMs             []*utils.VM `yaml:"vms"`
	Name            string      `yaml:"name"`

	IsoPath                    string `yaml:"iso-path"`
	DataStore                  string `yaml:"vsphere-datastore"`
	DataCenter                 string `yaml:"data-center"`
	WaitTimeout                string `yaml:"wait-timeout"` // Will be parsed to time.Duration
	Pool                       string `yaml:"vsphere-resource-pool"`
	DownloadVmdkURL            string `yaml:"download-vmdk-url"`
	LocalVmdkPath              string `yaml:"local-vmdk-path"`
	StorageSkipSSLVerification string `yaml:"storage-skip-ssl-verification"`
}

func LoadConfig(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var cfg Config
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}

func DefaultConfigPath() string {
	return filepath.Join("assets", "config", "static_values.yaml")
}
