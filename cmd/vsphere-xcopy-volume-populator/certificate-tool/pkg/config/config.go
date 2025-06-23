package config

import (
	"certificate-tool/internal/utils"
	"os"
	"path/filepath"
	"strings"

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

	StoragePasswordFile string `yaml:"storage-password-file"`
	StorageUser         string `yaml:"storage-user"`
	StorageURL          string `yaml:"storage-url"`
	StorageClassName    string `yaml:"storage-class-name"`

	VspherePasswordFile string      `yaml:"vsphere-password-file"`
	VsphereUser         string      `yaml:"vsphere-user"`
	VsphereURL          string      `yaml:"vsphere-url"`
	VMs                 []*utils.VM `yaml:"vms"`
	Name                string      `yaml:"name"`

	IsoPath                    string `yaml:"iso-path"`
	DataStore                  string `yaml:"vsphere-datastore"`
	DataCenter                 string `yaml:"data-center"`
	WaitTimeout                string `yaml:"wait-timeout"` // Will be parsed to time.Duration
	Pool                       string `yaml:"vsphere-resource-pool"`
	DownloadVmdkURL            string `yaml:"download-vmdk-url"`
	LocalVmdkPath              string `yaml:"local-vmdk-path"`
	StorageSkipSSLVerification string `yaml:"storage-skip-ssl-verification"`

	StoragePassword string `yaml:"-"`
	VspherePassword string `yaml:"-"`
}

func readPasswordFromFile(filePath string) (string, error) {
	if filePath == "" {
		return "", nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	// Only trim newlines and carriage returns, preserve spaces and tabs
	password := strings.TrimRight(string(data), "\r\n")
	return password, nil
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

	cfg.StoragePassword, err = readPasswordFromFile(cfg.StoragePasswordFile)
	if err != nil {
		return nil, err
	}

	cfg.VspherePassword, err = readPasswordFromFile(cfg.VspherePasswordFile)
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}

func DefaultConfigPath() string {
	return filepath.Join("assets", "config", "static_values.yaml")
}
