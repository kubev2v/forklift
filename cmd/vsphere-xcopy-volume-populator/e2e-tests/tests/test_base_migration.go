package tests

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/e2e-tests/helpers"
)

// TestStep represents a single step in a test case
type TestStep struct {
	Name     string
	Status   string // "Passed", "Failed", "Skipped"
	Duration time.Duration
	Message  string
}

// TestResult holds the results of a single test case
type TestResult struct {
	Name      string
	Status    string
	Duration  time.Duration
	Steps     []*TestStep
	Test      *testing.T
	startTime time.Time
}

func NewTestResult(t *testing.T) *TestResult {
	return &TestResult{
		Name:      t.Name(),
		Status:    "Running",
		Test:      t,
		startTime: time.Now(),
	}
}

func (tr *TestResult) Step(name string) *TestStep {
	step := &TestStep{Name: name, Status: "Running"}
	tr.Steps = append(tr.Steps, step)
	return step
}

func (tr *TestResult) End() {
	tr.Duration = time.Since(tr.startTime)
	if tr.Test.Failed() {
		tr.Status = "Failed"
	} else {
		tr.Status = "Passed"
	}
}

// TestConfig holds configuration for the e2e test
type TestConfig struct {
	VMNamePrefix         string
	VMOSType             string
	VMISOPath            string
	VMTemplateName       string
	VMDiskSizeGB         string
	VMDiskType           string
	VMMemoryMB           string
	VMCPUCount           string
	VsphereHost          string
	VsphereUsername      string
	VspherePassword      string
	VsphereDatacenter    string
	VsphereDatastore     string
	VsphereNetwork       string
	OCPAPIUrl            string
	OCPUsername          string
	OCPPassword          string
	OCPNamespace         string
	OCPStorageClass      string
	StorageVendorProduct string
	StorageHostname      string
	StorageUsername      string
	StoragePassword      string
	ONTAPSVM             string
	TargetVMName         string
	MigrationTimeoutMin  int
}

// TestFramework implements the main test logic
type TestFramework struct {
	Config          *TestConfig
	T               *testing.T
	vmName          string
	startTime       time.Time
	logger          *helpers.Logger
	openshiftClient *helpers.OpenShiftClient
	results         []*TestResult
	mu              sync.Mutex
}

func NewTestFramework(t *testing.T, diskType string) *TestFramework {
	framework := &TestFramework{
		T:         t,
		startTime: time.Now(),
	}

	// Initialize logger
	logger, err := helpers.NewLogger("/forklift/cmd/vsphere-xcopy-volume-populator/e2e-tests/logs", t.Name(), false)
	if err != nil {
		t.Fatalf("Failed to initialize logger: %v", err)
	}
	framework.logger = logger
	// Defer closing the log file to ensure it's handled properly on exit.
	t.Cleanup(func() {
		logger.Close()
	})

	// Initialize clients
	framework.openshiftClient = helpers.NewOpenShiftClient(logger)

	framework.loadConfiguration()
	framework.Config.VMDiskType = diskType

	return framework
}

func (test *TestFramework) Run() {
	test.logger.LogInfo("Starting test case: %s", test.T.Name())
	// Defer summary report at the very end
	defer test.generateSummaryReport()

	// Load configuration & prerequisites
	test.validatePrerequisites()

	// Defer cleanup at the top level to ensure it runs
	defer test.cleanup()

	test.runTest("SetupTestVM", test.setupTestVM)
	if test.T.Failed() {
		return
	}
	test.runTest("SetupOpenShiftEnvironment", test.setupOpenShiftEnvironment)
	if test.T.Failed() {
		return
	}
	test.runTest("CreateCopyOffloadConfiguration", test.createCopyOffloadConfiguration)
	if test.T.Failed() {
		return
	}
	test.runTest("ExecuteMigration", test.executeMigration)
	if test.T.Failed() {
		return
	}
	test.runTest("VerifyXCOPYUsage", test.verifyXCOPYUsage)
	if test.T.Failed() {
		return
	}
	test.runTest("VerifyVMInOpenShift", test.verifyVMInOpenShift)
	if test.T.Failed() {
		return
	}

	// Report success
	duration := time.Since(test.startTime)
	if !test.T.Failed() {
		test.T.Logf("✅ Copy-offload disk migration test completed successfully in %v", duration)
	}
}

func (test *TestFramework) runTest(name string, f func(t *testing.T)) {
	if test.T.Failed() {
		test.T.Skip("Skipping due to previous failure")
		return
	}
	test.T.Run(name, func(t *testing.T) {
		result := NewTestResult(t)
		test.mu.Lock()
		test.results = append(test.results, result)
		test.mu.Unlock()

		defer func() {
			result.End()
			if t.Failed() {
				test.T.Fail()
			}
		}()

		f(t)
	})
}

func (test *TestFramework) setupTestVM(t *testing.T) {
	if test.Config.TargetVMName != "" {
		test.vmName = test.Config.TargetVMName
		t.Logf("Using target VM for migration: %s", test.vmName)
	} else {
		test.generateVMName(t)
		test.createTestVM(t)
	}
}

func (test *TestFramework) loadConfiguration() {
	test.T.Helper()

	// Load configuration from environment
	configFile := filepath.Join(test.getProjectRoot(), "config", "test-config.env")
	if _, err := os.Stat(configFile); err == nil {
		test.T.Logf("Loading configuration from %s", configFile)
		test.sourceConfigFile(configFile)
	}

	vsphereHost := getEnvOrFail(test.T, "VSPHERE_HOST")
	vsphereHost = strings.TrimPrefix(vsphereHost, "https://")
	vsphereHost = strings.TrimSuffix(vsphereHost, "/")

	// Parse migration timeout from environment variable
	migrationTimeout := 60 // default 60 minutes
	if timeoutStr := getEnvOrDefault("MIGRATION_TIMEOUT_MIN", "60"); timeoutStr != "" {
		if parsed, err := strconv.Atoi(timeoutStr); err == nil && parsed > 0 {
			migrationTimeout = parsed
		} else {
			test.T.Logf("Warning: Invalid MIGRATION_TIMEOUT_MIN value '%s', using default %d minutes", timeoutStr, migrationTimeout)
		}
	}

	test.Config = &TestConfig{
		VMNamePrefix:         getEnvOrDefault("VM_NAME_PREFIX", "xcopy-test"),
		VMOSType:             getEnvOrDefault("VM_OS_TYPE", "linux-rhel8"),
		VMISOPath:            getEnvOrDefault("VM_ISO_PATH", ""),
		VMTemplateName:       getEnvOrDefault("VM_TEMPLATE_NAME", ""),
		VMDiskSizeGB:         getEnvOrDefault("VM_DISK_SIZE_GB", "20"),
		VMMemoryMB:           getEnvOrDefault("VM_MEMORY_MB", "2048"),
		VMCPUCount:           getEnvOrDefault("VM_CPU_COUNT", "2"),
		VsphereHost:          vsphereHost,
		VsphereUsername:      getEnvOrFail(test.T, "VSPHERE_USERNAME"),
		VspherePassword:      getEnvOrFail(test.T, "VSPHERE_PASSWORD"),
		VsphereDatacenter:    getEnvOrFail(test.T, "VSPHERE_DATACENTER"),
		VsphereDatastore:     getEnvOrFail(test.T, "VSPHERE_DATASTORE"),
		VsphereNetwork:       getEnvOrFail(test.T, "VSPHERE_NETWORK"),
		OCPAPIUrl:            getEnvOrFail(test.T, "OCP_API_URL"),
		OCPUsername:          getEnvOrFail(test.T, "OCP_USERNAME"),
		OCPPassword:          getEnvOrFail(test.T, "OCP_PASSWORD"),
		OCPNamespace:         getEnvOrDefault("OCP_NAMESPACE", "openshift-mtv"),
		OCPStorageClass:      getEnvOrFail(test.T, "OCP_STORAGE_CLASS"),
		StorageVendorProduct: getEnvOrFail(test.T, "STORAGE_VENDOR_PRODUCT"),
		StorageHostname:      getEnvOrFail(test.T, "STORAGE_HOSTNAME"),
		StorageUsername:      getEnvOrFail(test.T, "STORAGE_USERNAME"),
		StoragePassword:      getEnvOrFail(test.T, "STORAGE_PASSWORD"),
		ONTAPSVM:             getEnvOrDefault("ONTAP_SVM", ""),
		TargetVMName:         getEnvOrDefault("TARGET_VM_NAME", ""),
		MigrationTimeoutMin:  migrationTimeout,
	}

	test.T.Logf("Configuration loaded successfully")
}

func (test *TestFramework) validatePrerequisites() {
	test.T.Helper()

	test.T.Log("Validating prerequisites...")

	// Check required tools
	if err := helpers.CheckRequiredTools("ansible-playbook", "oc"); err != nil {
		test.T.Fatalf("Required tools check failed: %v", err)
	}

	// Validate project structure
	projectRoot := test.getProjectRoot()
	ansiblePath := filepath.Join(projectRoot, "ansible")
	if _, err := os.Stat(ansiblePath); os.IsNotExist(err) {
		test.T.Fatalf("Ansible directory not found: %s", ansiblePath)
	}

	helpersPath := filepath.Join(projectRoot, "helpers")
	if _, err := os.Stat(helpersPath); os.IsNotExist(err) {
		test.T.Fatalf("Helpers directory not found: %s", helpersPath)
	}

	test.T.Log("✅ Prerequisites validated")
}

func (test *TestFramework) generateVMName(t *testing.T) {
	t.Helper()

	timestamp := time.Now().Format("150405") // HHMMSS
	suffix, err := helpers.GenerateRandomString(6)
	if err != nil {
		t.Fatalf("Failed to generate random string: %v", err)
	}
	test.vmName = fmt.Sprintf("%s-%s-%s", test.Config.VMNamePrefix, timestamp, suffix)

	t.Logf("Generated VM name: %s", test.vmName)
}

func (test *TestFramework) createTestVM(t *testing.T) {
	t.Log("🔄 Step 1: Creating test VM in VMware")

	// Set environment variables for Ansible
	env := test.getAnsibleEnv()
	env = append(env, fmt.Sprintf("VM_NAME=%s", test.vmName))
	env = append(env, fmt.Sprintf("VM_DISK_TYPE=%s", test.Config.VMDiskType))

	// Run Ansible playbook to create VM
	ansiblePath := filepath.Join(test.getProjectRoot(), "ansible")
	cmd := exec.Command("ansible-playbook", "-i", "localhost,", "setup-vm.yml", "-vvv")
	cmd.Dir = ansiblePath
	cmd.Env = env

	if err := test.runAndStreamCommand(cmd); err != nil {
		t.Errorf("Failed to create test VM: %v", err)
	}

	t.Logf("✅ VM created successfully: %s", test.vmName)
}

func (test *TestFramework) setupOpenShiftEnvironment(t *testing.T) {
	t.Log("🔄 Step 2: Setting up OpenShift and Forklift environment")

	if err := test.openshiftClient.InitOpenShift(); err != nil {
		t.Fatalf("Failed to initialize OpenShift environment: %v", err)
	}
	if test.Config.TargetVMName != "" {
		storageSecretName := os.Getenv("STORAGE_SECRET_NAME")
		if storageSecretName == "" {
			t.Fatalf("STORAGE_SECRET_NAME must be set when using a target VM")
		}
	} else {
		storageSecretName := test.vmName + "-storage-secret"
		os.Setenv("STORAGE_SECRET_NAME", storageSecretName)

		if err := test.openshiftClient.CreateStorageSecret(); err != nil {
			t.Fatalf("Failed to create storage secret: %v", err)
		}
	}

	t.Log("✅ OpenShift environment ready for migration")
}

func (test *TestFramework) createCopyOffloadConfiguration(t *testing.T) {
	t.Log("🔄 Step 3: Creating copy-offload configuration")

	if test.Config.TargetVMName == "" {
		networkMapName := test.vmName + "-network-map"
		if err := test.openshiftClient.CreateNetworkMap(networkMapName); err != nil {
			t.Fatalf("Failed to create network map: %v", err)
		}

		storageMapName := test.vmName + "-storage-map"
		if err := test.openshiftClient.CreateStorageMap(storageMapName, test.Config.VsphereDatastore); err != nil {
			t.Fatalf("Failed to create storage map: %v", err)
		}
	}

	t.Log("✅ Copy-offload configuration created")
}

func (test *TestFramework) executeMigration(t *testing.T) {
	t.Log("🔄 Step 4: Creating and executing migration")

	planName := test.vmName + "-plan"
	migrationName := test.vmName + "-migration"
	storageMapName := test.vmName + "-storage-map"
	networkMapName := test.vmName + "-network-map"

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(test.Config.MigrationTimeoutMin)*time.Minute)
	defer cancel()

	// Create migration plan
	if err := test.openshiftClient.CreateMigrationPlan(ctx, planName, test.vmName, storageMapName, networkMapName); err != nil {
		t.Fatalf("Failed to create migration plan: %v", err)
	}

	// Start migration
	if err := test.openshiftClient.StartMigration(planName, migrationName); err != nil {
		t.Fatalf("Failed to start migration: %v", err)
	}

	// Wait for migration to complete
	if err := test.openshiftClient.WaitForMigrationCompletion(ctx, migrationName); err != nil {
		// Before failing, describe the migration resource for detailed status
		cmd := test.openshiftClient.ExecOcCommand("describe", "migration", migrationName, "-n", os.Getenv("FORKLIFT_NAMESPACE"))
		output, _ := cmd.CombinedOutput()
		description := string(output)
		t.Logf("Description of failed migration '%s':\n%s", migrationName, description)

		// If the failure is due to a populator pod, get its logs
		if strings.Contains(description, "populator pod failed for PVC") {
			re := regexp.MustCompile(`populator pod failed for PVC ([a-z0-9-]+)`)
			matches := re.FindStringSubmatch(description)
			if len(matches) > 1 {
				pvcName := matches[1]
				t.Logf("Attempting to get logs for populator pod associated with PVC '%s'...", pvcName)
				logs, logErr := test.openshiftClient.GetPopulatorPodLogs(pvcName)
				if logErr != nil {
					t.Logf("Could not retrieve populator pod logs: %v", logErr)
				} else {
					t.Logf("Logs from failed populator pod:\n%s", logs)
				}
			}
		}

		t.Fatalf("Migration failed to complete: %v", err)
	}

	t.Log("✅ Migration executed successfully")
}

func (test *TestFramework) verifyXCOPYUsage(t *testing.T) {
	t.Helper()
	t.Log("🔄 Step 5: Verifying XCOPY usage")

	migrationName := test.vmName + "-migration"
	if err := test.openshiftClient.VerifyXCopyUsage(migrationName); err != nil {
		t.Errorf("XCOPY usage verification failed: %v", err)
		return
	}

	t.Log("✅ XCOPY usage verified")
}

func (test *TestFramework) verifyVMInOpenShift(t *testing.T) {
	t.Helper()
	t.Log("🔄 Step 6: Verifying VM in OpenShift")

	running, err := test.openshiftClient.CheckVMStatusInOpenShift(test.vmName)
	if err != nil {
		t.Fatalf("Failed to check VM status in OpenShift: %v", err)
	}
	if !running {
		// Try to start the VM if it's not running
		t.Logf("VM '%s' is not running, attempting to start it...", test.vmName)
		if err := test.openshiftClient.StartVMInOpenShift(test.vmName); err != nil {
			t.Fatalf("Failed to start VM in OpenShift: %v", err)
		}
	}

	t.Log("✅ VM is running in OpenShift")
}

func (test *TestFramework) cleanup() {
	if test == nil || test.T == nil {
		return
	}
	test.T.Helper()

	if test.vmName == "" {
		test.T.Log("Skipping cleanup, no VM was created.")
		return
	}

	test.T.Log("Cleaning up resources...")

	planName := test.vmName + "-plan"
	migrationName := test.vmName + "-migration"
	storageMapName := ""
	networkMapName := ""
	secretName := os.Getenv("STORAGE_SECRET_NAME")

	if test.Config.TargetVMName == "" {
		storageMapName = test.vmName + "-storage-map"
		networkMapName = test.vmName + "-network-map"
		if secretName == "" {
			secretName = test.vmName + "-storage-secret"
		}
	}

	// Cleanup OpenShift resources
	if test.openshiftClient != nil {
		if err := test.openshiftClient.CleanupOpenShiftResources(planName, migrationName, storageMapName, networkMapName, test.vmName, secretName); err != nil {
			test.T.Logf("Failed to cleanup OpenShift resources: %v", err)
		}
	}

	// Cleanup vSphere VM if not specified as target
	if test.Config != nil && test.Config.TargetVMName == "" {
		env := test.getAnsibleEnv()
		env = append(env, fmt.Sprintf("VM_NAME=%s", test.vmName))
		env = append(env, "FORCE_DELETE=true")

		ansiblePath := filepath.Join(test.getProjectRoot(), "ansible")
		cmd := exec.Command("ansible-playbook", "-i", "localhost,", "teardown-vm.yml")
		cmd.Dir = ansiblePath
		cmd.Env = env

		if err := test.runAndStreamCommand(cmd); err != nil {
			test.T.Logf("Warning: Failed to cleanup VMware VM: %v", err)
		} else {
			test.T.Logf("VMware VM cleanup successful")
		}
	}

	test.T.Log("Cleanup complete")
}

// Helper methods

func (test *TestFramework) getProjectRoot() string {
	test.T.Helper()
	// The project root for the e2e tests is considered the 'e2e-tests' directory itself.
	wd, err := os.Getwd()
	if err != nil {
		test.T.Fatalf("Failed to get current directory: %v", err)
	}
	// The test runs from the 'tests' subdirectory, so we go up one level.
	return filepath.Dir(wd)
}

func (test *TestFramework) getAnsibleEnv() []string {
	test.T.Helper()

	// Note: This method includes sensitive credentials in environment variables
	// for Ansible execution. Ensure the execution environment is secure.
	env := os.Environ()
	env = append(env, fmt.Sprintf("VSPHERE_HOST=%s", test.Config.VsphereHost))
	env = append(env, fmt.Sprintf("VSPHERE_USERNAME=%s", test.Config.VsphereUsername))
	env = append(env, fmt.Sprintf("VSPHERE_PASSWORD=%s", test.Config.VspherePassword))
	env = append(env, fmt.Sprintf("VSPHERE_DATACENTER=%s", test.Config.VsphereDatacenter))
	env = append(env, fmt.Sprintf("VSPHERE_DATASTORE=%s", test.Config.VsphereDatastore))
	env = append(env, fmt.Sprintf("VSPHERE_NETWORK=%s", test.Config.VsphereNetwork))
	env = append(env, fmt.Sprintf("VM_NAME_PREFIX=%s", test.Config.VMNamePrefix))
	env = append(env, fmt.Sprintf("VM_OS_TYPE=%s", test.Config.VMOSType))
	env = append(env, fmt.Sprintf("VM_ISO_PATH=%s", test.Config.VMISOPath))
	env = append(env, fmt.Sprintf("VM_TEMPLATE_NAME=%s", test.Config.VMTemplateName))
	env = append(env, fmt.Sprintf("VM_DISK_SIZE_GB=%s", test.Config.VMDiskSizeGB))
	env = append(env, fmt.Sprintf("VM_MEMORY_MB=%s", test.Config.VMMemoryMB))
	env = append(env, fmt.Sprintf("VM_CPU_COUNT=%s", test.Config.VMCPUCount))
	env = append(env, fmt.Sprintf("OCP_API_URL=%s", test.Config.OCPAPIUrl))
	env = append(env, fmt.Sprintf("OCP_USERNAME=%s", test.Config.OCPUsername))
	env = append(env, fmt.Sprintf("OCP_PASSWORD=%s", test.Config.OCPPassword))
	env = append(env, fmt.Sprintf("OCP_NAMESPACE=%s", test.Config.OCPNamespace))
	env = append(env, fmt.Sprintf("OCP_STORAGE_CLASS=%s", test.Config.OCPStorageClass))
	env = append(env, fmt.Sprintf("STORAGE_VENDOR_PRODUCT=%s", test.Config.StorageVendorProduct))
	env = append(env, fmt.Sprintf("STORAGE_HOSTNAME=%s", test.Config.StorageHostname))
	env = append(env, fmt.Sprintf("STORAGE_USERNAME=%s", test.Config.StorageUsername))
	env = append(env, fmt.Sprintf("STORAGE_PASSWORD=%s", test.Config.StoragePassword))
	env = append(env, fmt.Sprintf("ONTAP_SVM=%s", test.Config.ONTAPSVM))
	env = append(env, fmt.Sprintf("FORKLIFT_NAMESPACE=%s", test.Config.OCPNamespace))
	if os.Getenv("STORAGE_SECRET_NAME") == "" {
		env = append(env, fmt.Sprintf("STORAGE_SECRET_NAME=%s", test.vmName+"-storage-secret"))
	}
	env = append(env, fmt.Sprintf("HOST_PROVIDER_NAME=%s", "host-provider"))
	env = append(env, fmt.Sprintf("VSPHERE_PROVIDER_NAME=%s", "vsphere-provider"))
	return env
}

func (test *TestFramework) runAndStreamCommand(cmd *exec.Cmd) error {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("error creating stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("error creating stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("error starting command: %w", err)
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			fmt.Println(test.sanitizeOutput(scanner.Text()))
		}
	}()

	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			fmt.Fprintln(os.Stderr, test.sanitizeOutput(scanner.Text()))
		}
	}()

	wg.Wait()

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("error waiting for command: %w", err)
	}

	return nil
}

func (test *TestFramework) sanitizeOutput(output string) string {
	sanitized := output
	if test.Config.VspherePassword != "" {
		sanitized = strings.ReplaceAll(sanitized, test.Config.VspherePassword, "[REDACTED]")
	}
	if test.Config.OCPPassword != "" {
		sanitized = strings.ReplaceAll(sanitized, test.Config.OCPPassword, "[REDACTED]")
	}
	if test.Config.StoragePassword != "" {
		sanitized = strings.ReplaceAll(sanitized, test.Config.StoragePassword, "[REDACTED]")
	}
	return sanitized
}

func (test *TestFramework) sourceConfigFile(configFile string) {
	file, err := os.Open(configFile)
	if err != nil {
		test.T.Logf("Warning: Could not open config file %s: %v", configFile, err)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Remove 'export ' prefix if it exists
		line = strings.TrimPrefix(line, "export ")

		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])

			// Remove surrounding quotes from value
			if len(value) > 1 && ((strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'")) || (strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\""))) {
				value = value[1 : len(value)-1]
			}

			// Only set the environment variable if it's not already set.
			// This prioritizes variables set in the shell over the config file.
			if _, ok := os.LookupEnv(key); !ok {
				os.Setenv(key, value)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		test.T.Logf("Warning: Error reading config file %s: %v", configFile, err)
	}
}

// Utility functions

func getEnv(key string) string {
	return os.Getenv(key)
}

func getEnvOrFail(t *testing.T, key string) string {
	t.Helper()
	value := getEnv(key)
	if value == "" {
		t.Fatalf("Environment variable %s is required", key)
	}
	return value
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := getEnv(key); value != "" {
		return value
	}
	return defaultValue
}

func (test *TestFramework) generateSummaryReport() {
	test.mu.Lock()
	defer test.mu.Unlock()

	var overallStatus string
	passedCount := 0
	failedCount := 0

	for _, result := range test.results {
		if result.Status == "Passed" {
			passedCount++
		} else {
			failedCount++
		}
	}

	if failedCount > 0 {
		overallStatus = "FAILED"
	} else {
		overallStatus = "PASSED"
	}

	// Create summary report content
	summaryContent := fmt.Sprintf(`
================================================================================
                          E2E Test Execution Summary
================================================================================
Test Name: %s
Overall Status: %s
Total Tests: %d | Passed: %d | Failed: %d
Total Duration: %v
Start Time: %s
End Time: %s
VM Name: %s
Disk Type: %s
--------------------------------------------------------------------------------

`, test.T.Name(), overallStatus, len(test.results), passedCount, failedCount,
		time.Since(test.startTime), test.startTime.Format("2006-01-02 15:04:05"),
		time.Now().Format("2006-01-02 15:04:05"), test.vmName, test.Config.VMDiskType)

	// Detailed breakdown
	for _, result := range test.results {
		summaryContent += fmt.Sprintf("\n[ %s ] %s (%v)\n", result.Status, result.Name, result.Duration)
		for _, step := range result.Steps {
			summaryContent += fmt.Sprintf("  - %-50s [%s] (%v)\n", step.Name, step.Status, step.Duration)
			if step.Message != "" {
				summaryContent += fmt.Sprintf("    - Message: %s\n", step.Message)
			}
		}
	}

	summaryContent += "\n================================================================================\n"

	// Print to console
	fmt.Print(summaryContent)

	// Save detailed summary to logs
	test.saveSummaryToFile(summaryContent)

	if failedCount > 0 {
		test.T.Fail()
	}
}

func (test *TestFramework) saveSummaryToFile(summaryContent string) {
	// Use the same logs directory as the main logger
	logsDir := getEnvOrDefault("E2E_LOG_DIR", "/forklift/cmd/vsphere-xcopy-volume-populator/e2e-tests/logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		test.T.Logf("Warning: Failed to create logs directory: %v", err)
		return
	}

	// Create summary file with timestamp
	timestamp := time.Now().Format("20060102_150405")
	summaryFile := filepath.Join(logsDir, fmt.Sprintf("test_summary_%s_%s.log", test.T.Name(), timestamp))

	file, err := os.Create(summaryFile)
	if err != nil {
		test.T.Logf("Warning: Failed to create summary file: %v", err)
		return
	}
	defer file.Close()

	if _, err := file.WriteString(summaryContent); err != nil {
		test.T.Logf("Warning: Failed to write summary to file: %v", err)
		return
	}

	test.T.Logf("Test summary saved to: %s", summaryFile)
}
