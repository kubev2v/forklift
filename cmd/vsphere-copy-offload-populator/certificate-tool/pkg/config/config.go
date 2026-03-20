package config

import (
	"bufio"
	"certificate-tool/internal/utils"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/term"
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

const passwordsDir = ".passwords"

func readPasswordFromFile(filePath string) (string, error) {
	if filePath == "" {
		return "", nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	password := strings.TrimRight(string(data), "\r\n")
	return password, nil
}

// promptForPassword prompts the user to enter a password securely
func promptForPassword(prompt string) (string, error) {
	fmt.Print(prompt)

	password, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		return "", fmt.Errorf("failed to read password: %w", err)
	}

	return string(password), nil
}

// savePasswordToFile saves a password to a file in the .passwords directory
func savePasswordToFile(password, filename string) error {
	if err := os.MkdirAll(passwordsDir, 0700); err != nil {
		return fmt.Errorf("failed to create passwords directory: %w", err)
	}

	filePath := filepath.Join(passwordsDir, filename)

	if err := os.WriteFile(filePath, []byte(password), 0600); err != nil {
		return fmt.Errorf("failed to write password file: %w", err)
	}

	return nil
}

// askToSavePassword prompts the user if they want to save the password
func askToSavePassword(passwordType string) bool {
	fmt.Printf("Would you like to save the %s password to %s for future use? (y/N): ", passwordType, passwordsDir)

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes"
}

// getOrPromptPassword gets password from file or prompts user if file path is empty
func getOrPromptPassword(passwordFile, passwordType, defaultFilename string) (string, error) {
	if passwordFile != "" {
		return readPasswordFromFile(passwordFile)
	}

	savedPasswordPath := filepath.Join(passwordsDir, defaultFilename)
	if _, err := os.Stat(savedPasswordPath); err == nil {
		fmt.Printf("Found saved %s password in %s\n", passwordType, savedPasswordPath)
		return readPasswordFromFile(savedPasswordPath)
	}

	password, err := promptForPassword(fmt.Sprintf("Enter %s password: ", passwordType))
	if err != nil {
		return "", err
	}
	if askToSavePassword(passwordType) {
		if saveErr := savePasswordToFile(password, defaultFilename); saveErr != nil {
			fmt.Printf("Warning: Failed to save password: %v\n", saveErr)
		} else {
			fmt.Printf("Password saved to %s\n", filepath.Join(passwordsDir, defaultFilename))
		}
	}

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

	cfg.StoragePassword, err = getOrPromptPassword(
		cfg.StoragePasswordFile,
		"storage",
		"storage-password.txt",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get storage password: %w", err)
	}

	// Get vSphere password
	cfg.VspherePassword, err = getOrPromptPassword(
		cfg.VspherePasswordFile,
		"vSphere",
		"vsphere-password.txt",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get vSphere password: %w", err)
	}

	return &cfg, nil
}

func DefaultConfigPath() string {
	return filepath.Join("assets", "config", "static_values.yaml")
}
