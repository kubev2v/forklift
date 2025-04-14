package main

import (
	"os/exec"
	"strings"
	"testing"
)

const (
	configFile = "read-config-files/static_values.yaml"
	envFile    = "read-config-files/dynamic_values.yaml"
)

func runMake(target string, envVars ...string) (string, error) {
	cmd := exec.Command("make", target)
	cmd.Env = append(cmd.Env, "CONFIG_FILE="+configFile, "ENV_FILE="+envFile)
	cmd.Env = append(cmd.Env, envVars...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func TestReadConfig(t *testing.T) {
	t.Run("Missing Files", func(t *testing.T) {
		invalidConfig := "/invalid/path/static_values.yaml"
		invalidEnv := "/invalid/path/dynamic_values.yaml"
		cmd := exec.Command("make", "read-config")
		cmd.Env = append(cmd.Env, "CONFIG_FILE="+invalidConfig, "ENV_FILE="+invalidEnv)
		output, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatalf("Expected error when files are missing, but got none")
		}
		if !strings.Contains(string(output), "Error") {
			t.Fatalf("Expected error message in output, got: %s", output)
		}
	})

	t.Run("Correct Values", func(t *testing.T) {
		output, err := runMake("read-config")
		if err != nil {
			t.Fatalf("Make command failed: %v", err)
		}
		if !strings.Contains(output, "target_namespace =") ||
			!strings.Contains(output, "storage_cendor =") ||
			!strings.Contains(output, "secret_name =") ||
			!strings.Contains(output, "kubeconfig =") ||
			!strings.Contains(output, "username =") {
			t.Fatalf("Unexpected output: %s", output)
		}
	})

	t.Run("Override Username", func(t *testing.T) {
		output, err := runMake("read-config", "USERNAME=overridden-user")
		if err != nil {
			t.Fatalf("Make command failed: %v", err)
		}
		if !strings.Contains(output, "username = overridden-user") {
			t.Fatalf("Expected overridden username in output, got: %s", output)
		}
	})
}
