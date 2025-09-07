//go:build e2e
// +build e2e

// Package tests contains the main test framework and test cases for the vSphere XCOPY volume populator.
// This package implements end-to-end tests that validate copy-offload functionality for VM disk migrations
// from VMware vSphere to OpenShift using XCOPY technology.
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

// TestStep represents a single step within a test case execution.
// Each step tracks its own status, duration, and any associated messages.
type TestStep struct {
	Name      string        // Human-readable name of the test step
	Status    string        // Current status: "Passed", "Failed", "Skipped"
	Duration  time.Duration // Time taken to complete this step
	Message   string        // Additional information or error message
	startTime time.Time     // Internal timestamp for duration calculation
}

// TestResult holds the complete results and metadata for a single test case.
// It tracks the overall status, duration, and all individual steps within the test.
type TestResult struct {
	Name      string        // Name of the test case
	Status    string        // Overall test status
	Duration  time.Duration // Total time taken for the test
	Steps     []*TestStep   // Individual steps executed during the test
	Test      *testing.T    // Reference to the Go testing framework
	startTime time.Time     // Internal timestamp for duration calculation
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
	step := &TestStep{
		Name:      name,
		Status:    "Running",
		startTime: time.Now(),
	}
	tr.Steps = append(tr.Steps, step)
	return step
}

func (step *TestStep) Complete(status string, message ...string) {
	step.Status = status
	step.Duration = time.Since(step.startTime)
	if len(message) > 0 {
		step.Message = message[0]
	}
}

func (tr *TestResult) End() {
	tr.Duration = time.Since(tr.startTime)
	if tr.Test.Failed() {
		tr.Status = "Failed"
	} else {
		tr.Status = "Passed"
	}
}

// TestConfig holds all configuration parameters needed for the e2e tests.
// Configuration is loaded from environment variables and used to configure
// both the vSphere and OpenShift environments for testing.
type TestConfig struct {
	// VM Configuration
	VMNamePrefix   string // Prefix for generated VM names
	VMOSType       string // Operating system type for the test VM
	VMISOPath      string // Path to ISO file for VM installation
	VMTemplateName string // Name of VM template to clone from
	VMDiskSizeGB   string // Size of VM disk in GB
	VMDiskType     string // Type of disk provisioning (thin, thick, eagerzeroedthick)
	VMMemoryMB     string // VM memory allocation in MB
	VMCPUCount     string // Number of CPUs for the VM

	// vSphere Configuration
	VsphereHost       string // vCenter server hostname or IP
	VsphereUsername   string // vSphere authentication username
	VspherePassword   string // vSphere authentication password
	VsphereDatacenter string // vSphere datacenter name
	VsphereDatastore  string // vSphere datastore for VM storage
	VsphereNetwork    string // vSphere network for VM connectivity

	// OpenShift Configuration
	OCPAPIUrl       string // OpenShift API server URL
	OCPUsername     string // OpenShift authentication username
	OCPPassword     string // OpenShift authentication password
	OCPNamespace    string // OpenShift namespace for migrated VMs
	OCPStorageClass string // Storage class for persistent volumes

	// Storage Array Configuration (for XCOPY)
	StorageVendorProduct string // Storage vendor/product (e.g., "ontap", "vantara")
	StorageHostname      string // Storage array management hostname
	StorageUsername      string // Storage array authentication username
	StoragePassword      string // Storage array authentication password
	ONTAPSVM             string // NetApp ONTAP Storage Virtual Machine name

	// Test Execution Configuration
	TargetVMName        string // Name of existing VM to use (skip creation)
	MigrationTimeoutMin int    // Timeout for migration operations in minutes
}

// TestFramework implements the main test logic and orchestrates the end-to-end test execution.
// It manages the test lifecycle from VM creation through migration verification and cleanup.
type TestFramework struct {
	Config            *TestConfig              // Test configuration parameters
	T                 *testing.T               // Go testing framework reference
	vmName            string                   // Generated or specified VM name
	storageSecretName string                   // Storage secret name for this test
	startTime         time.Time                // Test execution start time
	logger            *helpers.Logger          // Structured logger instance
	openshiftClient   *helpers.OpenShiftClient // OpenShift API client
	results           []*TestResult            // Collection of test results
	mu                sync.Mutex               // Mutex for thread-safe operations
}

// NewTestFramework creates and initializes a new test framework instance.
// It validates the input parameters, sets up logging, and initializes the OpenShift client.
// The diskType parameter must be one of the valid disk provisioning types.
func NewTestFramework(t *testing.T, diskType string) *TestFramework {
	if t == nil {
		panic("testing.T cannot be nil")
	}

	// Validate diskType
	validDiskTypes := []string{helpers.DiskThin, helpers.DiskThick, helpers.DiskEagerZeroedThick}
	diskTypeValid := false
	for _, validType := range validDiskTypes {
		if diskType == validType {
			diskTypeValid = true
			break
		}
	}
	if !diskTypeValid {
		t.Fatalf("Invalid disk type '%s'. Valid types are: %v", diskType, validDiskTypes)
	}

	framework := &TestFramework{
		T:         t,
		startTime: time.Now(),
	}

	// Initialize logger with configurable log directory
	logDir := getEnvOrDefault(helpers.EnvLogDir, helpers.DefaultLogDir)
	logger, err := helpers.NewLogger(logDir, t.Name(), false)
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

// Run executes the complete end-to-end test workflow.
// This includes VM provisioning, OpenShift setup, migration execution,
// and cleanup of all resources.
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

func (test *TestFramework) runTest(name string, f func(t *testing.T, result *TestResult)) {
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

		f(t, result)
	})
}

func (test *TestFramework) setupTestVM(t *testing.T, result *TestResult) {
	if test.Config.TargetVMName != "" {
		step := result.Step("Use existing target VM")
		test.vmName = test.Config.TargetVMName
		t.Logf("Using target VM for migration: %s", test.vmName)
		step.Complete("Passed", fmt.Sprintf("Using target VM: %s", test.vmName))
	} else {
		step1 := result.Step("Generate VM name")
		test.generateVMName(t)
		step1.Complete("Passed", fmt.Sprintf("Generated VM name: %s", test.vmName))

		step2 := result.Step("Create test VM in vSphere")
		test.createTestVM(t)
		if t.Failed() {
			step2.Complete("Failed", "Failed to create test VM")
		} else {
			step2.Complete("Passed", fmt.Sprintf("VM created successfully: %s", test.vmName))
		}
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

	vmwareHost := getEnvOrFail(test.T, "VMWARE_HOST")
	vmwareHost = strings.TrimPrefix(vmwareHost, "https://")
	vmwareHost = strings.TrimSuffix(vmwareHost, "/")

	// Parse migration timeout from environment variable
	migrationTimeout := helpers.DefaultMigrationTimeoutMin
	if timeoutStr := getEnvOrDefault("MIGRATION_TIMEOUT_MIN", fmt.Sprintf("%d", helpers.DefaultMigrationTimeoutMin)); timeoutStr != "" {
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
		VMDiskSizeGB:         getEnvOrDefault("VM_DISK_SIZE_GB", fmt.Sprintf("%d", helpers.DefaultVMDiskSizeGB)),
		VMMemoryMB:           getEnvOrDefault("VM_MEMORY_MB", fmt.Sprintf("%d", helpers.DefaultVMMemoryMB)),
		VMCPUCount:           getEnvOrDefault("VM_CPU_COUNT", fmt.Sprintf("%d", helpers.DefaultVMCPUCount)),
		VsphereHost:          vmwareHost,
		VsphereUsername:      getEnvOrFail(test.T, "VMWARE_USER"),
		VspherePassword:      getEnvOrFail(test.T, "VMWARE_PASSWORD"),
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
	suffix, err := helpers.GenerateRandomString(helpers.VMNameRandomSuffixLength)
	if err != nil {
		t.Fatalf("Failed to generate random string: %v", err)
	}
	test.vmName = fmt.Sprintf("%s-%s-%s", test.Config.VMNamePrefix, timestamp, suffix)

	t.Logf("Generated VM name: %s", test.vmName)
}

func (test *TestFramework) createTestVM(t *testing.T) {
	t.Log("🔄 Step 1: Creating test VM in VMware")

	// Build minimal environment for VM creation with only necessary variables
	additionalVars := map[string]string{
		"VM_NAME":      test.vmName,
		"VM_DISK_TYPE": test.Config.VMDiskType,
	}
	env := test.buildAnsibleEnvForOperation("create_vm", additionalVars)

	// Run Ansible playbook to create VM
	ansiblePath := filepath.Join(test.getProjectRoot(), "ansible")
	cmd := helpers.SecureExecCommand("ansible-playbook", "-i", "localhost,", "setup-vm.yml", "-vvv")
	cmd.Dir = ansiblePath
	// Safely merge existing cmd.Env (with hardened PATH) with new environment variables
	cmd.Env = mergeEnvironments(cmd.Env, env)

	if err := test.runAndStreamCommand(cmd); err != nil {
		t.Fatalf("Failed to create test VM: %v", err)
	}

	t.Logf("✅ VM created successfully: %s", test.vmName)
}

func (test *TestFramework) setupOpenShiftEnvironment(t *testing.T, result *TestResult) {
	t.Log("🔄 Step 2: Setting up OpenShift and Forklift environment")

	step1 := result.Step("Initialize OpenShift connection")
	if err := test.openshiftClient.InitOpenShift(); err != nil {
		step1.Complete("Failed", fmt.Sprintf("Failed to initialize OpenShift: %v", err))
		t.Fatalf("Failed to initialize OpenShift environment: %v", err)
	}
	step1.Complete("Passed", "OpenShift connection established")

	if test.Config.TargetVMName != "" {
		step2 := result.Step("Validate storage secret for target VM")
		test.storageSecretName = os.Getenv("STORAGE_SECRET_NAME")
		if test.storageSecretName == "" {
			step2.Complete("Failed", "STORAGE_SECRET_NAME not set for target VM")
			t.Fatalf("STORAGE_SECRET_NAME must be set when using a target VM")
		}
		step2.Complete("Passed", fmt.Sprintf("Using existing storage secret: %s", test.storageSecretName))
	} else {
		step2 := result.Step("Create storage secret")
		randSuffix, _ := helpers.GenerateRandomString(4)
		test.storageSecretName = fmt.Sprintf("%s-storage-secret-%s", test.vmName, randSuffix)

		if err := test.openshiftClient.CreateStorageSecretWithName(test.storageSecretName); err != nil {
			step2.Complete("Failed", fmt.Sprintf("Failed to create storage secret: %v", err))
			t.Fatalf("Failed to create storage secret: %v", err)
		}
		step2.Complete("Passed", fmt.Sprintf("Storage secret created: %s", test.storageSecretName))
	}

	t.Log("✅ OpenShift environment ready for migration")
}

func (test *TestFramework) createCopyOffloadConfiguration(t *testing.T, result *TestResult) {
	t.Log("🔄 Step 3: Creating copy-offload configuration")

	if test.Config.TargetVMName == "" {
		step1 := result.Step("Create network map")
		networkMapName := test.vmName + "-network-map"
		if err := test.openshiftClient.CreateNetworkMap(networkMapName); err != nil {
			step1.Complete("Failed", fmt.Sprintf("Failed to create network map: %v", err))
			t.Fatalf("Failed to create network map: %v", err)
		}
		step1.Complete("Passed", fmt.Sprintf("Network map created: %s", networkMapName))

		step2 := result.Step("Create storage map")
		storageMapName := test.vmName + "-storage-map"
		if err := test.openshiftClient.CreateStorageMapWithSecret(storageMapName, test.Config.VsphereDatastore, test.storageSecretName); err != nil {
			step2.Complete("Failed", fmt.Sprintf("Failed to create storage map: %v", err))
			t.Fatalf("Failed to create storage map: %v", err)
		}
		step2.Complete("Passed", fmt.Sprintf("Storage map created: %s", storageMapName))
	}

	t.Log("✅ Copy-offload configuration created")
}

func (test *TestFramework) executeMigration(t *testing.T, result *TestResult) {
	t.Log("🔄 Step 4: Creating and executing migration")

	planName := test.vmName + "-plan"
	migrationName := test.vmName + "-migration"
	storageMapName := test.vmName + "-storage-map"
	networkMapName := test.vmName + "-network-map"

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(test.Config.MigrationTimeoutMin)*time.Minute)
	defer cancel()

	// Create migration plan
	step1 := result.Step("Create migration plan")
	if err := test.openshiftClient.CreateMigrationPlan(ctx, planName, test.vmName, storageMapName, networkMapName); err != nil {
		step1.Complete("Failed", fmt.Sprintf("Failed to create migration plan: %v", err))
		t.Fatalf("Failed to create migration plan: %v", err)
	}
	step1.Complete("Passed", fmt.Sprintf("Migration plan created: %s", planName))

	// Start migration
	step2 := result.Step("Start migration")
	if err := test.openshiftClient.StartMigration(planName, migrationName); err != nil {
		step2.Complete("Failed", fmt.Sprintf("Failed to start migration: %v", err))
		t.Fatalf("Failed to start migration: %v", err)
	}
	step2.Complete("Passed", fmt.Sprintf("Migration started: %s", migrationName))

	// Wait for migration to complete
	step3 := result.Step("Wait for migration completion")
	if err := test.openshiftClient.WaitForMigrationCompletion(ctx, migrationName); err != nil {
		step3.Complete("Failed", fmt.Sprintf("Migration failed: %v", err))

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
	step3.Complete("Passed", fmt.Sprintf("Migration completed successfully: %s", migrationName))

	t.Log("✅ Migration executed successfully")
}

func (test *TestFramework) verifyVMInOpenShift(t *testing.T, result *TestResult) {
	t.Helper()
	t.Log("🔄 Step 5: Verifying VM in OpenShift")

	step1 := result.Step("Check VM status in OpenShift")
	running, err := test.openshiftClient.CheckVMStatusInOpenShift(test.vmName)
	if err != nil {
		step1.Complete("Failed", fmt.Sprintf("Failed to check VM status: %v", err))
		t.Fatalf("Failed to check VM status in OpenShift: %v", err)
	}

	if !running {
		step1.Complete("Passed", "VM found but not running")

		step2 := result.Step("Start VM in OpenShift")
		// Try to start the VM if it's not running
		t.Logf("VM '%s' is not running, attempting to start it...", test.vmName)
		if err := test.openshiftClient.StartVMInOpenShift(test.vmName); err != nil {
			step2.Complete("Failed", fmt.Sprintf("Failed to start VM: %v", err))
			t.Fatalf("Failed to start VM in OpenShift: %v", err)
		}
		step2.Complete("Passed", fmt.Sprintf("VM started successfully: %s", test.vmName))
	} else {
		step1.Complete("Passed", fmt.Sprintf("VM is already running: %s", test.vmName))
	}

	t.Log("✅ VM is running in OpenShift")
}

func (test *TestFramework) cleanup() {
	// Defensive programming: handle nil pointers gracefully
	if test == nil {
		fmt.Fprintf(os.Stderr, "Warning: TestFramework is nil, cannot perform cleanup\n")
		return
	}
	if test.T == nil {
		fmt.Fprintf(os.Stderr, "Warning: TestFramework.T is nil, cannot perform cleanup\n")
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
	secretName := test.storageSecretName

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
		additionalVars := map[string]string{
			"VM_NAME":      test.vmName,
			"FORCE_DELETE": "true",
		}
		env := test.buildAnsibleEnvForOperation("delete_vm", additionalVars)

		ansiblePath := filepath.Join(test.getProjectRoot(), "ansible")
		cmd := helpers.SecureExecCommand("ansible-playbook", "-i", "localhost,", "teardown-vm.yml")
		cmd.Dir = ansiblePath
		// Safely merge existing cmd.Env (with hardened PATH) with new environment variables
		cmd.Env = mergeEnvironments(cmd.Env, env)

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

// mergeEnvironments safely merges existing cmd.Env with new environment variables,
// preserving the hardened PATH from SecureExecCommand while adding/overriding specified keys
func mergeEnvironments(existingEnv, newEnv []string) []string {
	// Create a map from existing environment for easy lookup
	envMap := make(map[string]string)

	// Parse existing environment variables
	for _, env := range existingEnv {
		if parts := strings.SplitN(env, "=", 2); len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	// Add/override with new environment variables
	for _, env := range newEnv {
		if parts := strings.SplitN(env, "=", 2); len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	// Convert back to slice format
	result := make([]string, 0, len(envMap))
	for key, value := range envMap {
		result = append(result, fmt.Sprintf("%s=%s", key, value))
	}

	return result
}

// buildAnsibleEnvForOperation builds a minimal environment for a specific Ansible operation
// to minimize exposure of sensitive credentials
func (test *TestFramework) buildAnsibleEnvForOperation(operation string, additionalVars map[string]string) []string {
	test.T.Helper()

	env := test.getBaseSystemEnvironment()
	env = append(env, test.getOperationSpecificVars(operation)...)
	env = append(env, test.formatAdditionalVars(additionalVars)...)

	return env
}

// getBaseSystemEnvironment returns essential system variables that Ansible might need
func (test *TestFramework) getBaseSystemEnvironment() []string {
	var env []string
	systemVars := []string{"PATH", "HOME", "USER", "SHELL", "LANG", "LC_ALL", "TERM"}

	for _, varName := range systemVars {
		if value := os.Getenv(varName); value != "" {
			env = append(env, helpers.FormatEnvVar(varName, value))
		}
	}
	return env
}

// getOperationSpecificVars returns environment variables specific to the operation type
func (test *TestFramework) getOperationSpecificVars(operation string) []string {
	switch operation {
	case "create_vm", "delete_vm":
		return test.getVSphereEnvironmentVars()
	default:
		return []string{}
	}
}

// getVSphereEnvironmentVars returns vSphere-related environment variables
func (test *TestFramework) getVSphereEnvironmentVars() []string {
	var env []string

	// VMware global environment variables (automatically recognized by Ansible VMware modules)
	env = append(env, fmt.Sprintf("VMWARE_HOST=%s", test.Config.VsphereHost))
	env = append(env, fmt.Sprintf("VMWARE_USER=%s", test.Config.VsphereUsername))
	env = append(env, fmt.Sprintf("VMWARE_PASSWORD=%s", test.Config.VspherePassword))

	// vSphere-specific configuration (not global VMware module vars)
	env = append(env, fmt.Sprintf("VSPHERE_DATACENTER=%s", test.Config.VsphereDatacenter))
	env = append(env, fmt.Sprintf("VSPHERE_DATASTORE=%s", test.Config.VsphereDatastore))
	env = append(env, fmt.Sprintf("VSPHERE_NETWORK=%s", test.Config.VsphereNetwork))

	// SSL verification setting - default to true for secure connections
	// Set VMWARE_VALIDATE_CERTS=false explicitly to disable for lab/CI environments with self-signed certificates
	vmwareValidateCerts := os.Getenv("VMWARE_VALIDATE_CERTS")
	if vmwareValidateCerts == "" {
		vmwareValidateCerts = "true"
	}
	env = append(env, fmt.Sprintf("VMWARE_VALIDATE_CERTS=%s", vmwareValidateCerts))

	// VM configuration
	env = append(env, fmt.Sprintf("VM_NAME_PREFIX=%s", test.Config.VMNamePrefix))
	env = append(env, fmt.Sprintf("VM_OS_TYPE=%s", test.Config.VMOSType))
	env = append(env, fmt.Sprintf("VM_ISO_PATH=%s", test.Config.VMISOPath))
	env = append(env, fmt.Sprintf("VM_TEMPLATE_NAME=%s", test.Config.VMTemplateName))
	env = append(env, fmt.Sprintf("VM_DISK_SIZE_GB=%s", test.Config.VMDiskSizeGB))
	env = append(env, fmt.Sprintf("VM_MEMORY_MB=%s", test.Config.VMMemoryMB))
	env = append(env, fmt.Sprintf("VM_CPU_COUNT=%s", test.Config.VMCPUCount))

	return env
}

// formatAdditionalVars converts a map of additional variables to environment variable format
func (test *TestFramework) formatAdditionalVars(additionalVars map[string]string) []string {
	return helpers.FormatEnvVars(additionalVars)
}

func (test *TestFramework) runAndStreamCommand(cmd *exec.Cmd) error {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("error creating stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		// Ensure stdout is closed if stderr pipe creation fails
		stdout.Close()
		return fmt.Errorf("error creating stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		// Ensure pipes are closed if command start fails
		stdout.Close()
		stderr.Close()
		return fmt.Errorf("error starting command: %w", err)
	}

	var wg sync.WaitGroup
	wg.Add(2)

	// Channel to collect any scanner errors
	errChan := make(chan error, 2)

	go func() {
		defer wg.Done()
		defer stdout.Close() // Ensure stdout is always closed
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			sanitizedLine := test.sanitizeOutput(scanner.Text())
			if test.logger != nil {
				test.logger.LogInfo("%s", sanitizedLine)
			} else {
				fmt.Println(sanitizedLine)
			}
		}
		if err := scanner.Err(); err != nil {
			errChan <- fmt.Errorf("stdout scanner error: %w", err)
		}
	}()

	go func() {
		defer wg.Done()
		defer stderr.Close() // Ensure stderr is always closed
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			sanitizedLine := test.sanitizeOutput(scanner.Text())
			if test.logger != nil {
				test.logger.LogError("%s", sanitizedLine)
			} else {
				fmt.Fprintln(os.Stderr, sanitizedLine)
			}
		}
		if err := scanner.Err(); err != nil {
			errChan <- fmt.Errorf("stderr scanner error: %w", err)
		}
	}()

	// Wait for goroutines to complete
	wg.Wait()
	close(errChan)

	// Check for scanner errors
	for scanErr := range errChan {
		if test.logger != nil {
			test.logger.LogWarn("Scanner error: %v", scanErr)
		}
	}

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

	// Defensive copy to avoid potential race conditions during iteration
	resultsCopy := make([]*TestResult, len(test.results))
	copy(resultsCopy, test.results)

	for _, result := range resultsCopy {
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

`, test.T.Name(), overallStatus, len(resultsCopy), passedCount, failedCount,
		time.Since(test.startTime), test.startTime.Format("2006-01-02 15:04:05"),
		time.Now().Format("2006-01-02 15:04:05"), test.vmName, test.Config.VMDiskType)

	// Detailed breakdown using the defensive copy
	for _, result := range resultsCopy {
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
	logsDir := getEnvOrDefault(helpers.EnvLogDir, helpers.DefaultLogDir)
	if err := os.MkdirAll(logsDir, helpers.DefaultDirPermissions); err != nil {
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
