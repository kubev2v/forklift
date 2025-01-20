package customize

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/konveyor/forklift-controller/pkg/virt-v2v/global"
)

type MockEmbedTool struct {
	files                 map[string][]byte // A map of file paths to their content
	shouldFailCreateFiles bool              // Flag to simulate failure in CreateFilesFromFS
	shouldFailWriteFile   bool              // Flag to simulate failure in writeFileFromFS
}

// CreateFilesFromFS mocks the creation of files from the embedded filesystem
func (m *MockEmbedTool) CreateFilesFromFS(dstDir string) error {
	if m.shouldFailCreateFiles {
		return fmt.Errorf("mock error in CreateFilesFromFS")
	}
	for file, content := range m.files {
		dstFilePath := filepath.Join(dstDir, file)
		if err := m.writeFileFromFS(file, dstFilePath); err != nil {
			return err
		}
		if err := os.WriteFile(dstFilePath, content, 0755); err != nil {
			return err
		}
	}
	return nil
}

// writeFileFromFS mocks writing a file from the embedded filesystem to the disk
func (m *MockEmbedTool) writeFileFromFS(_, dst string) error {
	if m.shouldFailWriteFile {
		return fmt.Errorf("mock error in writeFileFromFS")
	}
	// Simulate directory creation
	dstDir := filepath.Dir(dst)
	if _, err := os.Stat(dstDir); os.IsNotExist(err) {
		if err := os.MkdirAll(dstDir, 0755); err != nil {
			return err
		}
	}
	return nil
}

func mockCustomizeDomainExec(args ...string) error {
	// Simulate the behavior of CustomizeDomainExec for testing
	fmt.Printf("Mock CustomizeDomainExec called with args: %v\n", args)
	return nil
}

// Test the CustomizeRHEL function using the mocked EmbedTool
func TestCustomizeRHELWithMock(t *testing.T) {
	// Create a temporary directory and add dummy .sh files
	tempDir := t.TempDir()

	mockFiles := map[string][]byte{
		"scripts/rhel/run/test1.sh":       []byte("#!/bin/bash\necho 'Running test1'"),
		"scripts/rhel/firstboot/test2.sh": []byte("#!/bin/bash\necho 'Running firstboot test2'"),
	}

	mockTool := &MockEmbedTool{
		files: mockFiles,
	}

	// Run the CustomizeRHEL function
	disks := []string{"disk1", "disk2"}
	err := CustomizeLinux(mockCustomizeDomainExec, disks, tempDir, mockTool)
	if err != nil {
		t.Fatalf("CustomizeRHEL returned an error: %v", err)
	}

	// Check if the files have been created on the disk
	for filePath := range mockFiles {
		expectedPath := filepath.Join(tempDir, filePath)
		if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
			t.Fatalf("Expected file %s does not exist", expectedPath)
		}
	}
}

func TestHandleStaticIPConfiguration(t *testing.T) {
	// Mock the environment variable for static IPs
	os.Setenv("V2V_staticIPs", "00:11:22:33:44:55:ip:192.168.1.100")
	defer os.Unsetenv("V2V_staticIPs")

	// Create a temporary directory and add dummy .sh files
	tempDir := t.TempDir()

	extraArgs := []string{}
	err := handleStaticIPConfiguration(&extraArgs, tempDir)
	if err != nil {
		t.Fatalf("handleStaticIPConfiguration returned an error: %v", err)
	}

	expectedFilePath := filepath.Join(tempDir, "macToIP")
	if _, err := os.Stat(expectedFilePath); os.IsNotExist(err) {
		t.Fatalf("Expected file %s does not exist", expectedFilePath)
	}

	content, _ := os.ReadFile(expectedFilePath)
	expectedContent := "00:11:22:33:44:55:ip:192.168.1.100\n"
	if string(content) != expectedContent {
		t.Fatalf("Content of %s is incorrect: got %s, want %s", expectedFilePath, string(content), expectedContent)
	}

	if len(extraArgs) == 0 || !contains(extraArgs, "--upload") {
		t.Fatalf("extraArgs does not contain expected '--upload' argument")
	}
}

func TestAddFirstbootScripts(t *testing.T) {
	// Create a temporary directory and add dummy .sh files
	tempDir := t.TempDir()
	err := os.MkdirAll(filepath.Join(tempDir, "scripts", "rhel", "firstboot"), 0755)
	if err != nil {
		t.Fatalf("Error MkdirAll: %v", err)
	}
	err = os.WriteFile(filepath.Join(tempDir, "scripts", "rhel", "firstboot", "test.sh"), []byte{}, 0755)
	if err != nil {
		t.Fatalf("Error WriteFile: %v", err)
	}

	extraArgs := []string{}
	err = addRhelFirstbootScripts(&extraArgs, tempDir)
	if err != nil {
		t.Fatalf("addRhelFirstbootScripts returned an error: %v", err)
	}

	if len(extraArgs) == 0 || !contains(extraArgs, "--firstboot") {
		t.Fatalf("extraArgs does not contain expected '--firstboot' argument")
	}
}

func TestAddRunScripts(t *testing.T) {
	// Create a temporary directory and add dummy .sh files
	tempDir := t.TempDir()
	err := os.MkdirAll(filepath.Join(tempDir, "scripts", "rhel", "run"), 0755)
	if err != nil {
		t.Fatalf("Error MkdirAll: %v", err)
	}
	err = os.WriteFile(filepath.Join(tempDir, "scripts", "rhel", "run", "test.sh"), []byte{}, 0755)
	if err != nil {
		t.Fatalf("Error WriteFile: %v", err)
	}

	extraArgs := []string{}
	err = addRhelRunScripts(&extraArgs, tempDir)
	if err != nil {
		t.Fatalf("addRhelRunScripts returned an error: %v", err)
	}

	if len(extraArgs) == 0 || !contains(extraArgs, "--run") {
		t.Fatalf("extraArgs does not contain expected '--run' argument")
	}
}

func TestGetScripts(t *testing.T) {
	// Create a temporary directory and add dummy .sh files
	tempDir := t.TempDir()
	err := os.WriteFile(filepath.Join(tempDir, "test1.sh"), []byte{}, 0755)
	if err != nil {
		t.Fatalf("Error WriteFile: %v", err)
	}
	err = os.WriteFile(filepath.Join(tempDir, "test2.sh"), []byte{}, 0755)
	if err != nil {
		t.Fatalf("Error WriteFile: %v", err)
	}

	scripts, err := getScriptsWithSuffix(tempDir, global.SHELL_SUFFIX)
	if err != nil {
		t.Fatalf("getScriptsWithSuffix returned an error: %v", err)
	}

	expectedScripts := []string{
		filepath.Join(tempDir, "test1.sh"),
		filepath.Join(tempDir, "test2.sh"),
	}
	if !reflect.DeepEqual(scripts, expectedScripts) {
		t.Fatalf("getScriptsWithSuffix returned incorrect scripts: got %v, want %v", scripts, expectedScripts)
	}
}

func TestAddDisksToCustomize(t *testing.T) {
	disks := []string{"disk1", "disk2"}
	extraArgs := []string{}

	addDisksToCustomize(&extraArgs, disks)
	if !contains(extraArgs, "disk1") || !contains(extraArgs, "disk2") {
		t.Fatalf("extraArgs does not contain expected disk arguments")
	}
}

// Helper function to check if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func TestAddFirstbootScripts_NoScripts(t *testing.T) {
	// Create a temporary directory with no scripts
	tempDir := t.TempDir()
	err := os.MkdirAll(filepath.Join(tempDir, "scripts", "rhel", "firstboot"), 0755)
	if err != nil {
		t.Fatalf("Error MkdirAll: %v", err)
	}

	extraArgs := []string{}
	err = addRhelFirstbootScripts(&extraArgs, tempDir)
	if err != nil {
		t.Fatalf("addRhelFirstbootScripts returned an error: %v", err)
	}

	// Ensure no "--firstboot" argument is added when no scripts are found
	if contains(extraArgs, "--firstboot") {
		t.Fatalf("extraArgs contains '--firstboot' argument when no scripts should have been found")
	}
}

func TestAddRunScripts_NoScripts(t *testing.T) {
	// Create a temporary directory with no scripts
	tempDir := t.TempDir()
	err := os.MkdirAll(filepath.Join(tempDir, "scripts", "rhel", "run"), 0755)
	if err != nil {
		t.Fatalf("Error MkdirAll: %v", err)
	}

	extraArgs := []string{}
	err = addRhelRunScripts(&extraArgs, tempDir)
	if err != nil {
		t.Fatalf("addRhelRunScripts returned an error: %v", err)
	}

	// Ensure no "--run" argument is added when no scripts are found
	if contains(extraArgs, "--run") {
		t.Fatalf("extraArgs contains '--run' argument when no scripts should have been found")
	}
}

func TestCustomizeRHEL_CreateFilesFromFSFails(t *testing.T) {
	mockTool := &MockEmbedTool{
		files:                 map[string][]byte{},
		shouldFailCreateFiles: true, // Simulate failure in CreateFilesFromFS
	}

	// Run the CustomizeRHEL function
	disks := []string{"disk1", "disk2"}
	err := CustomizeLinux(mockCustomizeDomainExec, disks, t.TempDir(), mockTool)
	if err == nil {
		t.Fatalf("Expected error in CustomizeRHEL due to CreateFilesFromFS failure, got nil")
	}

	expectedError := "failed to create files from filesystem: mock error in CreateFilesFromFS"
	if err.Error() != expectedError {
		t.Fatalf("Expected error message '%s', got '%s'", expectedError, err.Error())
	}
}

func TestHandleStaticIPConfiguration_WriteFileFails(t *testing.T) {
	// Mock the environment variable for static IPs
	os.Setenv("V2V_staticIPs", "00:11:22:33:44:55:ip:192.168.1.100")
	defer os.Unsetenv("V2V_staticIPs")

	// Use an invalid directory to force a write failure
	tempDir := "/invalid-dir"

	extraArgs := []string{}
	err := handleStaticIPConfiguration(&extraArgs, tempDir)
	if err == nil {
		t.Fatalf("Expected error in handleStaticIPConfiguration due to write failure, got nil")
	}
}

func TestAddFirstbootScripts_ReadDirFails(t *testing.T) {
	// Use an invalid directory to force a read failure
	tempDir := "/invalid-dir"

	extraArgs := []string{}
	err := addRhelFirstbootScripts(&extraArgs, tempDir)
	if err == nil {
		t.Fatalf("Expected error in addRhelFirstbootScripts due to read failure, got nil")
	}
}

func TestAddRunScripts_ReadDirFails(t *testing.T) {
	// Use an invalid directory to force a read failure
	tempDir := "/invalid-dir"

	extraArgs := []string{}
	err := addRhelRunScripts(&extraArgs, tempDir)
	if err == nil {
		t.Fatalf("Expected error in addRhelRunScripts due to read failure, got nil")
	}
}
