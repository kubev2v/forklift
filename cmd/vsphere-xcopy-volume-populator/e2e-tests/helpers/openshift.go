package helpers

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// OpenShiftClient handles OpenShift operations
type OpenShiftClient struct {
	logger *Logger
}

// SanitizeName sanitizes a name to be DNS-1123 compliant.
// DNS-1123 label requirements:
// - contain only lowercase alphanumeric characters or '-'
// - start and end with an alphanumeric character
// - be no more than 63 characters long
// This function:
// 1. Converts to lowercase
// 2. Replaces any character not in [a-z0-9-] with '-'
// 3. Collapses consecutive '-' to a single '-'
// 4. Trims leading/trailing '-' to ensure start/end with alphanumeric
// 5. Enforces max length of 63 chars with truncation preserving trailing alphanumeric
// 6. Returns "default" if result is empty
func SanitizeName(name string) string {
	if name == "" {
		return "default"
	}

	// Remove quotes first, then convert to lowercase
	name = strings.Trim(name, `"'`)
	name = strings.ToLower(name)

	// Replace any character not [a-z0-9-] with '-'
	var result strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			result.WriteRune(r)
		} else {
			result.WriteRune('-')
		}
	}

	sanitized := result.String()

	// Collapse consecutive '-' to single '-'
	for strings.Contains(sanitized, "--") {
		sanitized = strings.ReplaceAll(sanitized, "--", "-")
	}

	// Trim leading/trailing '-'
	sanitized = strings.Trim(sanitized, "-")

	// If empty after trimming, return default
	if sanitized == "" {
		return "default"
	}

	// Enforce max length (DNS-1123 limit)
	if len(sanitized) > DNS1123MaxLength {
		sanitized = sanitized[:DNS1123MaxLength]
		// Ensure it ends with alphanumeric after truncation
		sanitized = strings.TrimRight(sanitized, "-")
		if sanitized == "" {
			return "default"
		}
	}

	return sanitized
}

// sanitizeCommand sanitizes a command string by redacting sensitive flags using regex patterns.
// This approach handles quoted arguments, environment variables, and various flag formats robustly.
func sanitizeCommand(command string) string {
	// Use regex patterns to redact sensitive information while preserving command structure
	patterns := []struct {
		pattern     string
		replacement string
	}{
		// Environment variable assignments (KUBECONFIG=path, TOKEN=value, etc.)
		{`(?i)\b(kubeconfig|token|password|secret|key|auth|credential|cert|tls|ssl|private)=\S+`, `${1}=REDACTED`},

		// Long flags with equals: --token=value, --password="quoted value", etc.
		{`(?i)--?(token|password|secret|key|auth|credential|kubeconfig|cert|tls|ssl|private)=(?:"[^"]*"|'[^']*'|\S+)`, `--${1}=REDACTED`},

		// Long flags with space: --token value, --password "quoted value", etc.
		{`(?i)(--?(token|password|secret|key|auth|credential|kubeconfig|cert|tls|ssl|private))\s+(?:"[^"]*"|'[^']*'|\S+)`, `${1} REDACTED`},

		// Short flags with space: -t value, -p "quoted value", etc.
		{`(?i)(-[tpks])\s+(?:"[^"]*"|'[^']*'|\S+)`, `${1} REDACTED`},

		// Base64 encoded tokens (common in Kubernetes contexts)
		{`(?i)(token|bearer):\s*[A-Za-z0-9+/]{20,}={0,2}`, `${1}: REDACTED`},

		// API keys and long hex strings that might be secrets
		{`\b[A-Fa-f0-9]{32,}\b`, `REDACTED`},

		// JWT tokens (three base64 parts separated by dots)
		{`\beyJ[A-Za-z0-9+/]*\.eyJ[A-Za-z0-9+/]*\.[A-Za-z0-9+/\-_]*`, `REDACTED_JWT`},
	}

	result := command
	for _, p := range patterns {
		re := regexp.MustCompile(p.pattern)
		result = re.ReplaceAllString(result, p.replacement)
	}

	return result
}

// ExecRemoteCommand executes an arbitrary command on the remote host if SSH_HOST is set.
// If SSH_HOST is not set, it executes the command locally.
func (o *OpenShiftClient) ExecRemoteCommand(command string, args ...string) *exec.Cmd {
	sshHost := strings.Trim(os.Getenv("SSH_HOST"), `"'`)
	// If SSH_HOST is not set, run command locally using secure exec
	if sshHost == "" {
		return SecureExecCommand(command, args...)
	}

	// If SSH_HOST is set, build a shell-safe command to run over SSH
	sshUser := strings.Trim(os.Getenv("SSH_USER"), `"'`)
	sshTarget := strings.Trim(sshHost, `"'`)
	if u := strings.Trim(sshUser, `"'`); u != "" {
		sshTarget = fmt.Sprintf("%s@%s", u, sshTarget)
	}

	// Build remote command with proper shell quoting using strconv.Quote
	// Note: For SSH remote execution, we must pass through shell, but we use
	// strconv.Quote for safer quoting than the old shellEscape approach
	var remoteCommandParts []string
	remoteCommandParts = append(remoteCommandParts, strconv.Quote(command))
	for _, arg := range args {
		remoteCommandParts = append(remoteCommandParts, strconv.Quote(arg))
	}
	remoteCommand := strings.Join(remoteCommandParts, " ")

	// Prepend KUBECONFIG if needed with safe quoting
	if command == "oc" {
		if remoteKubeconfigPath := os.Getenv("REMOTE_KUBECONFIG_PATH"); remoteKubeconfigPath != "" {
			kubeconfig := strings.Trim(remoteKubeconfigPath, `"'`)
			remoteCommand = fmt.Sprintf("KUBECONFIG=%s %s", strconv.Quote(kubeconfig), remoteCommand)
		}
	}

	o.logger.LogDebug("Executing remote command via SSH: ssh %s %s", sshTarget, sanitizeCommand(remoteCommand))

	// Build SSH command with individual arguments (secure approach)
	sshArgs := []string{
		"-o", "LogLevel=ERROR",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
	}

	if sshKeyPath := strings.Trim(os.Getenv("SSH_KEY_PATH"), `"'`); sshKeyPath != "" {
		sshArgs = append(sshArgs, "-i", sshKeyPath)
	}

	// Add target and the command as separate arguments
	sshArgs = append(sshArgs, sshTarget, remoteCommand)

	return SecureExecCommand("ssh", sshArgs...)
}

// execScriptLocally executes a shell script locally using a temporary file.
func (o *OpenShiftClient) execScriptLocally(scriptContent string) ([]byte, error) {
	// Create a temporary local file for the script
	localScript, err := os.CreateTemp("", "local-script-*.sh")
	if err != nil {
		return nil, fmt.Errorf("failed to create local temp script file: %w", err)
	}
	defer os.Remove(localScript.Name())
	defer localScript.Close()

	_, err = localScript.WriteString(scriptContent)
	if err != nil {
		return nil, fmt.Errorf("failed to write to local temp script file: %w", err)
	}

	// Make the script executable (owner only for security)
	err = os.Chmod(localScript.Name(), 0700)
	if err != nil {
		return nil, fmt.Errorf("failed to make local script executable: %w", err)
	}

	// Execute the script locally
	cmd := SecureExecCommand("bash", localScript.Name())
	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("failed to execute local script: %w", err)
	}

	return output, nil
}

// ExecRemoteScript securely executes a shell script on the remote host if SSH_HOST is set.
// If SSH_HOST is not set, it executes the script locally.
func (o *OpenShiftClient) ExecRemoteScript(scriptContent string) ([]byte, error) {
	sshHost := strings.Trim(os.Getenv("SSH_HOST"), `"'`)
	if sshHost == "" {
		// If SSH_HOST is not set, run script locally
		return o.execScriptLocally(scriptContent)
	}

	// Create a temporary local file for the script
	localScript, err := os.CreateTemp("", "remote-script-*.sh")
	if err != nil {
		return nil, fmt.Errorf("failed to create local temp script file: %w", err)
	}
	defer os.Remove(localScript.Name())
	defer localScript.Close()

	_, err = localScript.WriteString(scriptContent)
	if err != nil {
		return nil, fmt.Errorf("failed to write to local temp script file: %w", err)
	}

	// Build SSH target and remote path
	sshUser := strings.Trim(os.Getenv("SSH_USER"), `"'`)
	sshTarget := sshHost
	if sshUser != "" {
		sshTarget = fmt.Sprintf("%s@%s", sshUser, sshHost)
	}
	remotePath := fmt.Sprintf("/tmp/remote-script-%d.sh", time.Now().UnixNano())

	sshKeyPath := strings.Trim(os.Getenv("SSH_KEY_PATH"), `"'`)
	scpArgs := []string{"-o", "LogLevel=ERROR", "-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null"}
	if sshKeyPath != "" {
		scpArgs = append(scpArgs, "-i", sshKeyPath)
	}
	scpArgs = append(scpArgs, localScript.Name(), fmt.Sprintf("%s:%s", sshTarget, remotePath))

	// 1. Copy the script to the remote host
	scpCmd := SecureExecCommand("scp", scpArgs...)
	if output, err := scpCmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("failed to scp script to remote host: %w. Output: %s", err, string(output))
	}

	sshArgs := []string{"-o", "LogLevel=ERROR", "-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null"}
	if sshKeyPath != "" {
		sshArgs = append(sshArgs, "-i", sshKeyPath)
	}
	sshArgs = append(sshArgs, sshTarget, "bash", remotePath)

	// 2. Execute the script on the remote host
	sshCmd := SecureExecCommand("ssh", sshArgs...)
	output, err := sshCmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			o.logger.LogError("Remote script execution failed. Stderr: %s. Stdout: %s", string(exitErr.Stderr), string(output))
		} else {
			o.logger.LogError("Failed to execute remote script: %v. Output: %s", err, string(output))
		}
	}

	cleanupArgs := []string{"-o", "LogLevel=ERROR", "-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null"}
	if sshKeyPath != "" {
		cleanupArgs = append(cleanupArgs, "-i", sshKeyPath)
	}
	cleanupArgs = append(cleanupArgs, sshTarget, "rm", remotePath)

	// 3. Clean up the script on the remote host
	cleanupCmd := SecureExecCommand("ssh", cleanupArgs...)
	if cleanupOutput, cleanupErr := cleanupCmd.CombinedOutput(); cleanupErr != nil {
		o.logger.LogWarn("Failed to remove remote script '%s': %v. Output: %s", remotePath, cleanupErr, string(cleanupOutput))
	}

	if err != nil {
		return output, fmt.Errorf("failed to execute remote script: %w", err)
	}

	return output, nil
}

// ExecOcCommand constructs an `oc` command, running it over SSH if configured.
func (o *OpenShiftClient) ExecOcCommand(args ...string) *exec.Cmd {
	return o.ExecRemoteCommand("oc", args...)
}

// NewOpenShiftClient creates a new OpenShift client
func NewOpenShiftClient(logger *Logger) *OpenShiftClient {
	return &OpenShiftClient{
		logger: logger,
	}
}

// Login logs into OpenShift cluster
func (o *OpenShiftClient) Login() error {
	apiURL := strings.Trim(os.Getenv("OCP_API_URL"), `"'`)
	token := strings.Trim(os.Getenv("OCP_TOKEN"), `"'`)

	// Trim quotes from all variables, as they can be included from env files
	fullAPIURL := strings.Trim(apiURL, `"'`)

	// Ensure the URL has a scheme. Default to https if not present.
	if !strings.HasPrefix(fullAPIURL, "https://") && !strings.HasPrefix(fullAPIURL, "http://") {
		fullAPIURL = "https://" + fullAPIURL
	}

	// Ensure the URL has a port. Default to 6443 if not present.
	parsedURL, err := url.Parse(fullAPIURL)
	if err == nil {
		if parsedURL.Port() == "" {
			parsedURL.Host = net.JoinHostPort(parsedURL.Hostname(), strconv.Itoa(DefaultOpenShiftAPIPort))
			fullAPIURL = parsedURL.String()
			o.logger.LogInfo("API URL has no port, appending default '%d'. New URL: %s", DefaultOpenShiftAPIPort, fullAPIURL)
		}
	}

	sshHost := strings.Trim(os.Getenv("SSH_HOST"), `"'`)
	if sshHost != "" {
		o.logger.LogInfo("Logging into OpenShift cluster at %s via remote host %s", fullAPIURL, sshHost)
	} else {
		o.logger.LogInfo("Logging into OpenShift cluster at %s", fullAPIURL)
	}

	var cmd *exec.Cmd
	if token != "" {
		o.logger.LogInfo("Using OCP_TOKEN for authentication.")
		cmd = o.ExecOcCommand("login", fullAPIURL, fmt.Sprintf("--token=%s", token), "--insecure-skip-tls-verify=true")
	} else {
		o.logger.LogInfo("Using OCP_USERNAME and OCP_PASSWORD for authentication.")
		username := strings.Trim(os.Getenv("OCP_USERNAME"), `"'`)
		password := strings.Trim(os.Getenv("OCP_PASSWORD"), `"'`)
		trimmedUser := strings.Trim(username, `"'`)
		trimmedPass := strings.Trim(password, `"'`)
		cmd = o.ExecOcCommand("login", fullAPIURL, "-u", trimmedUser, "--insecure-skip-tls-verify=true")
		cmd.Stdin = strings.NewReader(trimmedPass + "\n")
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		o.logger.LogError("Login command failed. Output: %s", string(output))
		return fmt.Errorf("failed to login to OpenShift cluster: %w", err)
	}

	o.logger.LogInfo("Successfully logged into OpenShift cluster")

	// Show current user
	cmd = o.ExecOcCommand("whoami")
	output, err = cmd.Output()
	if err == nil {
		o.logger.LogInfo("Current user: %s", strings.TrimSpace(string(output)))
	}

	return nil
}

// CheckForkliftInstallation checks if Forklift is installed and ready
func (o *OpenShiftClient) CheckForkliftInstallation() error {
	o.logger.LogInfo("Checking Forklift installation")

	namespace := strings.Trim(os.Getenv("FORKLIFT_NAMESPACE"), `"'`)
	if namespace == "" {
		namespace = "openshift-mtv"
	}

	// Check if namespace exists
	cmd := o.ExecOcCommand("get", "namespace", namespace)
	if _, err := cmd.Output(); err != nil {
		return fmt.Errorf("Forklift namespace '%s' not found: %v", namespace, err)
	}

	// Check if forklift controller exists
	cmd = o.ExecOcCommand("get", "deployment", "forklift-controller", "-n", namespace)
	if _, err := cmd.Output(); err != nil {
		return fmt.Errorf("Forklift controller deployment not found: %v", err)
	}

	// Wait for controller to be ready
	o.logger.LogInfo("Waiting for Forklift controller to be ready")
	cmd = o.ExecOcCommand("wait", "--for=condition=Available", "deployment/forklift-controller",
		"-n", namespace, "--timeout=300s")
	if _, err := cmd.Output(); err != nil {
		return fmt.Errorf("Forklift controller not ready within timeout: %v", err)
	}

	o.logger.LogInfo("Forklift installation verified")
	return nil
}

// EnableCopyOffloadFeature enables the copy-offload feature flag if it's not already enabled.
func (o *OpenShiftClient) EnableCopyOffloadFeature() error {
	o.logger.LogInfo("Checking copy-offload feature flag status...")

	namespace := strings.Trim(os.Getenv("FORKLIFT_NAMESPACE"), `"'`)
	if namespace == "" {
		namespace = "openshift-mtv"
	}

	// Check the current state of the feature flag
	checkCmd := o.ExecOcCommand("get", "forkliftcontrollers.forklift.konveyor.io", "forklift-controller",
		"-n", namespace, "-o", `jsonpath={.spec.feature_copy_offload}`)

	output, err := checkCmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			o.logger.LogWarn("Checking feature flag returned an error, but continuing. Stderr: %s", string(exitErr.Stderr))
		}
	}
	// If the feature is already enabled, we don't need to do anything.
	if strings.TrimSpace(string(output)) == "true" {
		o.logger.LogInfo("✅ Copy-offload feature is already enabled.")
		return nil
	}

	o.logger.LogInfo("Copy-offload feature not enabled. Enabling it now...")

	// Enable the feature flag by patching the controller resource
	patchCmd := o.ExecOcCommand("patch", "forkliftcontrollers.forklift.konveyor.io", "forklift-controller",
		"--type", "merge", "-p", `{"spec":{"feature_copy_offload":true}}`, "-n", namespace)

	patchOutput, err := patchCmd.CombinedOutput()
	if err != nil {
		o.logger.LogError("Failed to enable copy-offload feature flag. Output: %s", string(patchOutput))
		return fmt.Errorf("failed to enable copy-offload feature flag: %w", err)
	}

	o.logger.LogInfo("Copy-offload feature flag enabled. Waiting for controller to apply changes...")

	// Use exponential backoff or polling instead of fixed sleep
	ctx := context.Background()
	err = WaitForCondition(ctx, func() bool {
		// Check if controller deployment is available
		cmd := o.ExecOcCommand("get", "deployment", "forklift-controller", "-n", namespace, "-o", "jsonpath={.status.conditions[?(@.type=='Available')].status}")
		output, _ := cmd.Output()
		return string(output) == "True"
	}, 30*time.Second, 2*time.Second, "controller deployment to become available", o.logger)

	if err != nil {
		o.logger.LogError("Failed to wait for forklift controller deployment to become available: %v", err)
		return fmt.Errorf("failed to wait for forklift controller deployment to become available: %w", err)
	}

	o.logger.LogInfo("✅ Forklift controller is ready with copy-offload enabled.")
	return nil
}

// CreateStorageSecretWithName creates the storage secret for copy-offload with the specified name.
func (o *OpenShiftClient) CreateStorageSecretWithName(secretName string) error {
	if secretName == "" {
		return fmt.Errorf("secret name cannot be empty")
	}

	secretName = SanitizeName(secretName)
	namespace := strings.Trim(os.Getenv("FORKLIFT_NAMESPACE"), `"'`)
	if namespace == "" {
		namespace = "openshift-mtv"
	}

	o.logger.LogInfo("Checking if storage secret '%s' exists...", secretName)
	// Check if the secret already exists.
	checkCmd := o.ExecOcCommand("get", "secret", secretName, "-n", namespace)
	if _, err := checkCmd.Output(); err == nil {
		o.logger.LogInfo("✅ Storage secret '%s' already exists. Skipping creation.", secretName)
		return nil
	}

	o.logger.LogInfo("Storage secret '%s' not found. Creating it now...", secretName)

	// Create new secret based on storage vendor using secure YAML manifest
	storageVendor := strings.Trim(os.Getenv("STORAGE_VENDOR_PRODUCT"), `"'`)
	username := strings.Trim(os.Getenv("STORAGE_USERNAME"), `"'`)
	password := strings.Trim(os.Getenv("STORAGE_PASSWORD"), `"'`)
	hostname := strings.Trim(os.Getenv("STORAGE_HOSTNAME"), `"'`)
	ontapSvm := strings.Trim(os.Getenv("ONTAP_SVM"), `"'`)

	// Build Secret manifest using stringData to avoid exposing credentials in process args
	var secretYAML string
	switch storageVendor {
	case "ontap":
		secretYAML = fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: %s
  namespace: %s
type: Opaque
stringData:
  STORAGE_USERNAME: %s
  STORAGE_PASSWORD: %s
  STORAGE_HOSTNAME: %s
  ONTAP_SVM: %s
`, secretName, namespace, username, password, hostname, ontapSvm)
	case "vantara":
		secretYAML = fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: %s
  namespace: %s
type: Opaque
stringData:
  STORAGE_USERNAME: %s
  STORAGE_PASSWORD: %s
  STORAGE_HOSTNAME: %s
`, secretName, namespace, username, password, hostname)
	default:
		return fmt.Errorf("unsupported storage vendor: %s", storageVendor)
	}

	// Apply the Secret manifest via stdin to avoid credential exposure
	cmd := o.ExecOcCommand("apply", "-f", "-")
	cmd.Stdin = strings.NewReader(secretYAML)
	output, err := cmd.CombinedOutput()
	if err != nil {
		o.logger.LogError("Failed to create storage secret. Output: %s", string(output))
		return fmt.Errorf("failed to create storage secret: %w", err)
	}

	o.logger.LogInfo("✅ Storage secret '%s' created successfully.", secretName)
	return nil
}

// CreateStorageSecret creates the storage secret for copy-offload if it doesn't already exist.
// This method reads the secret name from the STORAGE_SECRET_NAME environment variable.
// For better test isolation, use CreateStorageSecretWithName instead.
func (o *OpenShiftClient) CreateStorageSecret() error {
	secretName := strings.Trim(os.Getenv("STORAGE_SECRET_NAME"), `"'`)
	if secretName == "" {
		return fmt.Errorf("STORAGE_SECRET_NAME must be set before creating a storage secret")
	}
	return o.CreateStorageSecretWithName(secretName)
}

// CreateStorageMapWithSecret creates or updates StorageMap for copy-offload with the specified storage secret.
func (o *OpenShiftClient) CreateStorageMapWithSecret(storageMapName, datastoreID, storageSecretName string) error {
	o.logger.LogInfo("Creating StorageMap '%s' for copy-offload", storageMapName)

	namespace := strings.Trim(os.Getenv("FORKLIFT_NAMESPACE"), `"'`)
	if namespace == "" {
		namespace = "openshift-mtv"
	}

	storageClass := strings.Trim(os.Getenv("OCP_STORAGE_CLASS"), `"'`)
	secretName := SanitizeName(storageSecretName)
	storageVendor := strings.Trim(os.Getenv("STORAGE_VENDOR_PRODUCT"), `"'`)
	hostProviderName := SanitizeName(os.Getenv("HOST_PROVIDER_NAME"))
	vsphereProviderName := SanitizeName(os.Getenv("VSPHERE_PROVIDER_NAME"))

	storageMapYAML := fmt.Sprintf(`
apiVersion: forklift.konveyor.io/v1beta1
kind: StorageMap
metadata:
  name: %s
  namespace: %s
spec:
  map:
  - destination:
      storageClass: %s
      accessMode: ReadWriteOnce
    offloadPlugin:
      vsphereXcopyConfig:
        secretRef: %s
        storageVendorProduct: %s
    source:
      id: %s
  provider:
    destination:
      apiVersion: forklift.konveyor.io/v1beta1
      kind: Provider
      name: %s
      namespace: %s
    source:
      apiVersion: forklift.konveyor.io/v1beta1
      kind: Provider
      name: %s
      namespace: %s
`, storageMapName, namespace, storageClass, secretName, storageVendor, datastoreID,
		hostProviderName, namespace, vsphereProviderName, namespace)

	// Apply the StorageMap
	cmd := o.ExecOcCommand("apply", "-f", "-")
	cmd.Stdin = strings.NewReader(storageMapYAML)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create StorageMap: %v. Output: %s", err, string(output))
	}

	o.logger.LogInfo("StorageMap '%s' created successfully", storageMapName)
	return nil
}

// CreateStorageMap creates or updates StorageMap for copy-offload.
// This method reads the storage secret name from the STORAGE_SECRET_NAME environment variable.
// For better test isolation, use CreateStorageMapWithSecret instead.
func (o *OpenShiftClient) CreateStorageMap(storageMapName, datastoreID string) error {
	storageSecretName := strings.Trim(os.Getenv("STORAGE_SECRET_NAME"), `"'`)
	if storageSecretName == "" {
		return fmt.Errorf("STORAGE_SECRET_NAME must be set before creating a storage map")
	}
	return o.CreateStorageMapWithSecret(storageMapName, datastoreID, storageSecretName)
}

// CreateNetworkMap creates a NetworkMap for the migration.
func (o *OpenShiftClient) CreateNetworkMap(networkMapName string) error {
	o.logger.LogInfo("Creating NetworkMap '%s'", networkMapName)

	namespace := strings.Trim(os.Getenv("FORKLIFT_NAMESPACE"), `"'`)
	if namespace == "" {
		namespace = "openshift-mtv"
	}

	vsphereProviderName := SanitizeName(os.Getenv("VSPHERE_PROVIDER_NAME"))
	hostProviderName := SanitizeName(os.Getenv("HOST_PROVIDER_NAME"))
	sourceNetwork := strings.Trim(os.Getenv("VSPHERE_NETWORK"), `"'`)

	networkMapYAML := fmt.Sprintf(`
apiVersion: forklift.konveyor.io/v1beta1
kind: NetworkMap
metadata:
  name: %s
  namespace: %s
spec:
  map:
    - destination:
        name: pod
        type: pod
      source:
        name: %s
  provider:
    destination:
      name: %s
      namespace: %s
    source:
      name: %s
      namespace: %s
`, networkMapName, namespace, sourceNetwork, hostProviderName, namespace, vsphereProviderName, namespace)

	// Apply the NetworkMap
	cmd := o.ExecOcCommand("apply", "-f", "-")
	cmd.Stdin = strings.NewReader(networkMapYAML)
	if output, err := cmd.CombinedOutput(); err != nil {
		o.logger.LogError("Failed to create NetworkMap. Output: %s", string(output))
		return fmt.Errorf("failed to create NetworkMap: %w", err)
	}

	o.logger.LogInfo("✅ NetworkMap '%s' created successfully", networkMapName)
	return nil
}

// CreateMigrationPlan creates migration plan
func (o *OpenShiftClient) CreateMigrationPlan(ctx context.Context, planName, vmName, storageMapName, networkMapName string) error {
	o.logger.LogInfo("Creating migration plan '%s'", planName)

	namespace := strings.Trim(os.Getenv("FORKLIFT_NAMESPACE"), `"'`)
	if namespace == "" {
		namespace = "openshift-mtv"
	}
	vsphereProviderName := SanitizeName(os.Getenv("VSPHERE_PROVIDER_NAME"))
	hostProviderName := SanitizeName(os.Getenv("HOST_PROVIDER_NAME"))

	// Get the VM's Managed Object ID from the vSphere provider.
	vmID := o.getVMIDFromProvider(vmName)
	if vmID == "" {
		return fmt.Errorf("could not find VM ID for VM named '%s'", vmName)
	}
	o.logger.LogInfo("Found vSphere VM ID for '%s': %s", vmName, vmID)

	planYAML := fmt.Sprintf(`
apiVersion: forklift.konveyor.io/v1beta1
kind: Plan
metadata:
  name: %s
  namespace: %s
spec:
  provider:
    source:
      name: %s
      namespace: %s
    destination:
      name: %s
      namespace: %s
  targetNamespace: %s
  pvcNameTemplate: "{{.VmName}}-disk-{{.DiskIndex}}"
  map:
    storage:
      name: %s
      namespace: %s
    network:
      name: %s
      namespace: %s
  vms:
  - id: "%s"
`, planName, namespace, vsphereProviderName, namespace, hostProviderName, namespace, namespace, storageMapName, namespace, networkMapName, namespace, vmID)

	// Apply the plan
	cmd := o.ExecOcCommand("apply", "-f", "-")
	cmd.Stdin = strings.NewReader(planYAML)

	if output, err := cmd.CombinedOutput(); err != nil {
		o.logger.LogError("Failed to apply migration plan. API server output: %s", string(output))
		return fmt.Errorf("failed to create migration plan: %w", err)
	}

	o.logger.LogInfo("✅ Migration plan '%s' created successfully.", planName)

	// Now, wait for the plan to become ready before proceeding.
	return o.WaitForPlanReady(ctx, planName)
}

// WaitForPlanReady waits for a migration plan to become ready with improved diagnostics.
func (o *OpenShiftClient) WaitForPlanReady(ctx context.Context, planName string) error {
	timeout := DefaultPlanTimeoutSeconds * time.Second
	if t := os.Getenv("PLAN_TIMEOUT"); t != "" {
		if parsedTimeout, err := time.ParseDuration(t); err == nil {
			timeout = parsedTimeout
		} else {
			o.logger.LogWarn("Could not parse PLAN_TIMEOUT value '%s', using default.", t)
		}
	}

	o.logger.LogInfo("Waiting for plan '%s' to be ready (timeout: %v)", planName, timeout)

	namespace := strings.Trim(os.Getenv("FORKLIFT_NAMESPACE"), `"'`)
	if namespace == "" {
		namespace = "openshift-mtv"
	}

	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(DefaultPollingIntervalSeconds * time.Second)
	defer ticker.Stop()

	var lastConditionsJSON string

	for {
		select {
		case <-waitCtx.Done():
			o.logger.LogError("Timed out waiting for plan '%s' to become ready.", planName)
			if lastConditionsJSON != "" {
				o.logger.LogError("Final conditions for plan '%s': %s", planName, lastConditionsJSON)
			}
			return fmt.Errorf("timed out waiting for plan '%s' to become ready. Last known conditions: %s", planName, lastConditionsJSON)
		case <-ticker.C:
			// Check context before executing command
			if waitCtx.Err() != nil {
				return waitCtx.Err()
			}

			cmd := o.ExecOcCommand("get", "plan", planName, "-n", namespace, "-o", "json")
			output, err := cmd.Output()
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					o.logger.LogWarn("Failed to get plan '%s': %v. Stderr: %s", planName, err, string(exitErr.Stderr))
				} else {
					o.logger.LogWarn("Failed to get plan '%s': %v", planName, err)
				}
				continue
			}

			var plan struct {
				Status struct {
					Conditions []struct {
						Type     string `json:"type"`
						Status   string `json:"status"`
						Category string `json:"category"`
						Message  string `json:"message"`
					} `json:"conditions"`
				} `json:"status"`
			}

			if err := json.Unmarshal(output, &plan); err != nil {
				o.logger.LogWarn("Failed to unmarshal plan status for '%s': %v", planName, err)
				continue
			}

			conditionsJSON, _ := json.MarshalIndent(plan.Status.Conditions, "", "  ")
			lastConditionsJSON = string(conditionsJSON)
			o.logger.LogDebug("Current conditions for plan '%s':\n%s", planName, lastConditionsJSON)

			isReady := false
			for _, c := range plan.Status.Conditions {
				// Check for blocker conditions that would prevent the plan from ever becoming ready.
				if (c.Category == "Critical" || c.Category == "Error") && c.Status == "True" {
					return fmt.Errorf("plan '%s' has a blocking condition (Type: %s, Category: %s): %s",
						planName, c.Type, c.Category, c.Message)
				}
				// Check if the plan has reached the desired ready state.
				if c.Type == "Ready" && c.Status == "True" {
					isReady = true
				}
			}

			if isReady {
				o.logger.LogInfo("✅ Plan '%s' is ready.", planName)
				return nil
			}

			o.logger.LogInfo("Plan '%s' not yet ready, continuing to wait...", planName)
		}
	}
}

// StartMigration starts migration
func (o *OpenShiftClient) StartMigration(planName, migrationName string) error {
	if migrationName == "" {
		migrationName = planName + "-migration"
	}

	o.logger.LogInfo("Starting migration '%s' for plan '%s'", migrationName, planName)

	namespace := strings.Trim(os.Getenv("FORKLIFT_NAMESPACE"), `"'`)
	if namespace == "" {
		namespace = "openshift-mtv"
	}

	migrationYAML := fmt.Sprintf(`
apiVersion: forklift.konveyor.io/v1beta1
kind: Migration
metadata:
  name: %s
  namespace: %s
spec:
  plan:
    apiVersion: forklift.konveyor.io/v1beta1
    kind: Plan
    name: %s
    namespace: %s
`, migrationName, namespace, planName, namespace)

	// Apply the Migration
	cmd := o.ExecOcCommand("apply", "-f", "-")
	cmd.Stdin = strings.NewReader(migrationYAML)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to start migration: %v. Output: %s", err, string(output))
	}

	o.logger.LogInfo("Migration '%s' started", migrationName)
	return nil
}

// WaitForMigrationCompletion waits for migration to complete and reports its final state.
func (o *OpenShiftClient) WaitForMigrationCompletion(ctx context.Context, migrationName string) error {
	timeout := DefaultMigrationTimeoutSeconds * time.Second
	if t := os.Getenv("MIGRATION_TIMEOUT"); t != "" {
		if parsedTimeout, err := time.ParseDuration(t); err == nil {
			timeout = parsedTimeout
		} else {
			o.logger.LogWarn("Could not parse MIGRATION_TIMEOUT value '%s', using default.", t)
		}
	}

	o.logger.LogInfo("Waiting for migration '%s' to complete (timeout: %v)", migrationName, timeout)

	namespace := strings.Trim(os.Getenv("FORKLIFT_NAMESPACE"), `"'`)
	if namespace == "" {
		namespace = "openshift-mtv"
	}

	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-waitCtx.Done():
			return fmt.Errorf("timed out waiting for migration '%s' to complete", migrationName)
		case <-ticker.C:
			cmd := o.ExecOcCommand("get", "migration", migrationName, "-n", namespace, "-o", "json")
			output, err := cmd.Output()
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					o.logger.LogWarn("Failed to get migration '%s': %v. Stderr: %s", migrationName, err, string(exitErr.Stderr))
				} else {
					o.logger.LogWarn("Failed to get migration '%s': %v", migrationName, err)
				}
				continue
			}

			var migration struct {
				Status struct {
					Conditions []struct {
						Type     string `json:"type"`
						Status   string `json:"status"`
						Category string `json:"category"`
						Message  string `json:"message"`
					} `json:"conditions"`
				} `json:"status"`
			}

			if err := json.Unmarshal(output, &migration); err != nil {
				o.logger.LogWarn("Failed to unmarshal migration status for '%s': %v", migrationName, err)
				continue
			}

			for _, c := range migration.Status.Conditions {
				if c.Status == "True" {
					switch c.Type {
					case "Succeeded":
						o.logger.LogInfo("✅ Migration '%s' completed successfully.", migrationName)
						return nil
					case "Failed":
						o.logger.LogError("Migration '%s' failed. Reason: %s", migrationName, c.Message)
						return fmt.Errorf("migration '%s' failed: %s", migrationName, c.Message)
					case "Canceled":
						o.logger.LogWarn("Migration '%s' was canceled. Reason: %s", migrationName, c.Message)
						return fmt.Errorf("migration '%s' was canceled: %s", migrationName, c.Message)
					}
				}
			}
			o.logger.LogInfo("Migration '%s' not yet finished, continuing to wait...", migrationName)
		}
	}
}

// VerifyXCopyUsage verifies XCOPY was used in migration
func (o *OpenShiftClient) VerifyXCopyUsage(migrationName string) error {
	o.logger.LogInfo("Verifying XCOPY was used in migration '%s'", migrationName)

	namespace := strings.Trim(os.Getenv("FORKLIFT_NAMESPACE"), `"'`)
	if namespace == "" {
		namespace = "openshift-mtv"
	}

	// Check for VSphereXcopyVolumePopulator resources
	cmd := o.ExecOcCommand("get", "vsphereXcopyVolumePopulator", "-n", namespace, "--no-headers")
	output, err := cmd.Output()
	if err != nil {
		o.logger.LogWarn("Failed to check for VSphereXcopyVolumePopulator resources: %v", err)
		return fmt.Errorf("failed to check populator resources: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	populatorCount := 0
	if len(lines) > 0 && lines[0] != "" {
		populatorCount = len(lines)
	}

	if populatorCount == 0 {
		o.logger.LogWarn("No VSphereXcopyVolumePopulator resources found - XCOPY may not have been used")
		return fmt.Errorf("no XCOPY populator resources found")
	}

	o.logger.LogInfo("Found %d VSphereXcopyVolumePopulator resource(s)", populatorCount)

	// Check populator logs for XCOPY operations
	cmd = o.ExecOcCommand("get", "pods", "-n", namespace, "-l", "app=vsphere-xcopy-volume-populator", "--no-headers", "-o", "name")
	output, err = cmd.Output()
	if err == nil && len(output) > 0 {
		o.logger.LogInfo("Checking populator pod logs for XCOPY evidence")
		pods := strings.Split(strings.TrimSpace(string(output)), "\n")
		for _, pod := range pods {
			if pod != "" {
				cmd = o.ExecOcCommand("logs", pod, "-n", namespace)
				logOutput, err := cmd.Output()
				if err == nil && (strings.Contains(string(logOutput), "xcopy") || strings.Contains(string(logOutput), "XCOPY")) {
					o.logger.LogInfo("XCOPY usage confirmed in %s logs", pod)
					return nil
				}
			}
		}
	}

	// Check migration logs for copy-offload evidence
	cmd = o.ExecOcCommand("logs", "-n", namespace, "deployment/forklift-controller")
	output, err = cmd.Output()
	if err == nil {
		logLines := strings.Split(string(output), "\n")
		for i := len(logLines) - 10; i < len(logLines); i++ {
			if i >= 0 && i < len(logLines) {
				line := strings.ToLower(logLines[i])
				if strings.Contains(line, "copy.offload") || strings.Contains(line, "xcopy") || strings.Contains(line, "populator") {
					o.logger.LogInfo("Copy-offload evidence found in controller logs")
					return nil
				}
			}
		}
	}

	o.logger.LogWarn("Could not definitively verify XCOPY usage")
	return fmt.Errorf("could not verify XCOPY usage")
}

func (o *OpenShiftClient) getProviderUIDByName(apiHost, token, providerName string) (string, error) {
	// Query the top-level providers endpoint, which mirrors the user's successful manual command.
	providersURL := fmt.Sprintf("https://%s/providers/", apiHost)
	o.logger.LogInfo("Looking up provider UID for '%s' at: %s", providerName, providersURL)

	script := fmt.Sprintf(`
		curl -s -k \
		-H "Authorization: Bearer %s" \
		-H "Accept: application/json" \
		%s
	`, token, providersURL)

	body, err := o.ExecRemoteScript(script)
	if err != nil {
		return "", fmt.Errorf("failed to get provider list via remote script: %w. Output: %s", err, string(body))
	}

	if len(strings.TrimSpace(string(body))) == 0 {
		return "", fmt.Errorf("received empty response from provider list endpoint")
	}

	// Define a struct to hold the full, nested JSON response from the /providers/ endpoint.
	type apiProvider struct {
		UID  string `json:"uid"`
		Name string `json:"name"`
	}
	type allProvidersResponse struct {
		VsphereProviders []apiProvider `json:"vsphere"`
	}

	var response allProvidersResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("failed to unmarshal provider list: %w. Body: %s", err, string(body))
	}

	// Now, iterate through the list of vSphere providers found in the JSON.
	for _, p := range response.VsphereProviders {
		if p.Name == providerName {
			o.logger.LogInfo("✅ Found UID for provider '%s': %s", providerName, p.UID)
			return p.UID, nil
		}
	}

	o.logger.LogWarn("Provider with name '%s' not found in API response.", providerName)
	return "", nil
}

// getVMIDFromProvider fetches the VM's Managed Object ID by querying the Forklift API.
func (o *OpenShiftClient) getVMIDFromProvider(vmName string) string {
	namespace := strings.Trim(os.Getenv("FORKLIFT_NAMESPACE"), `"'`)
	if namespace == "" {
		namespace = "openshift-mtv"
	}
	providerName := SanitizeName(os.Getenv("VSPHERE_PROVIDER_NAME"))
	apiRouteName := strings.Trim(os.Getenv("FORKLIFT_API_ROUTE"), `"'`)
	if apiRouteName == "" {
		apiRouteName = "forklift-inventory" // Default to the most likely route name.
	}

	// 1. Get an authentication token for the API.
	tokenCmd := o.ExecOcCommand("whoami", "--show-token")
	tokenBytes, err := tokenCmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			o.logger.LogError("Could not get OpenShift auth token: %v. Stderr: %s", err, string(exitErr.Stderr))
		} else {
			o.logger.LogError("Could not get OpenShift auth token: %v.", err)
		}
		return ""
	}
	token := strings.TrimSpace(string(tokenBytes))
	if token == "" {
		o.logger.LogError("Got empty token from 'oc whoami --show-token'.")
		return ""
	}

	// 2. Find the hostname of the Forklift API server.
	routeCmd := o.ExecOcCommand("get", "route", apiRouteName, "-n", namespace, "-o", `jsonpath={.spec.host}`)
	out, err := routeCmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			o.logger.LogError("Could not find route for '%s': %v. Stderr: %s", apiRouteName, err, string(exitErr.Stderr))
		} else {
			o.logger.LogError("Could not find route for '%s': %v", apiRouteName, err)
		}
		return ""
	}
	apiHost := strings.TrimSpace(string(out))
	if apiHost == "" {
		o.logger.LogError("'%s' route is empty.", apiRouteName)
		return ""
	}

	// 3. Get the UID of the provider by its name.
	providerUID, err := o.getProviderUIDByName(apiHost, token, providerName)
	if err != nil {
		o.logger.LogError("Failed to get provider UID: %v", err)
		return ""
	}
	if providerUID == "" {
		return "" // The warning is logged in the helper function.
	}

	// 4. Construct the API request URL using the provider's UID.
	apiURL := fmt.Sprintf("https://%s/providers/vsphere/%s/vms", apiHost, providerUID)
	o.logger.LogInfo("Querying for VM list at: %s", apiURL)

	// 5. Make the authenticated HTTP GET request via a remote script.
	script := fmt.Sprintf(`
		curl -s -k \
		-H "Authorization: Bearer %s" \
		-H "Accept: application/json" \
		%s
	`, token, apiURL)

	body, err := o.ExecRemoteScript(script)
	if err != nil {
		return ""
	}

	// It's possible for the API to return an empty body if the inventory is empty.
	if len(strings.TrimSpace(string(body))) == 0 {
		o.logger.LogWarn("Forklift API returned an empty response for the VM list. This can happen if the provider's inventory is empty or has not yet been synchronized. Please check the provider's status.")
		return ""
	}

	// 6. Define a struct to unmarshal the VM list and find the ID.
	type apiVM struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	var vms []apiVM
	if err := json.Unmarshal(body, &vms); err != nil {
		o.logger.LogError("Failed to unmarshal VM list from API response: %v. Body: %s", err, string(body))
		return ""
	}

	if len(vms) == 0 {
		o.logger.LogWarn("Forklift API returned 0 VMs for provider '%s'. Please check that the provider is correctly configured, has finished synchronizing, and that the target VM exists in vSphere.", providerName)
	}

	for _, vm := range vms {
		if vm.Name == vmName {
			o.logger.LogInfo("✅ Found VM ID via Forklift API: %s", vm.ID)
			return vm.ID
		}
	}

	o.logger.LogWarn("VM '%s' not found in API response after checking %d VMs.", vmName, len(vms))
	return ""
}

// CheckVMStatusInOpenShift checks VM status in OpenShift
func (o *OpenShiftClient) CheckVMStatusInOpenShift(vmName string) (bool, error) {
	namespace := strings.Trim(os.Getenv("OCP_NAMESPACE"), `"'`)
	o.logger.LogInfo("Checking VM '%s' status in OpenShift", vmName)

	// Check if VirtualMachine resource exists
	cmd := o.ExecOcCommand("get", "vm", SanitizeName(vmName), "-n", namespace)
	if _, err := cmd.Output(); err != nil {
		o.logger.LogError("VirtualMachine '%s' not found in namespace '%s'", SanitizeName(vmName), namespace)
		return false, fmt.Errorf("VirtualMachine not found: %v", err)
	}

	// Check VM status
	cmd = o.ExecOcCommand("get", "vm", SanitizeName(vmName), "-n", namespace, "-o", "jsonpath={.status.printableStatus}")
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to get VM status: %v", err)
	}

	status := strings.TrimSpace(string(output))
	o.logger.LogInfo("VM '%s' status: %s", SanitizeName(vmName), status)

	return status == "Running", nil
}

// StartVMInOpenShift starts VM in OpenShift
func (o *OpenShiftClient) StartVMInOpenShift(vmName string) error {
	namespace := strings.Trim(os.Getenv("OCP_NAMESPACE"), `"'`)
	o.logger.LogInfo("Starting VM '%s' in OpenShift", vmName)

	cmd := o.ExecOcCommand("patch", "vm", SanitizeName(vmName), "-n", namespace,
		"--type", "merge", "-p", `{"spec":{"runStrategy":"Always"}}`)
	if _, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to start VM '%s': %v", SanitizeName(vmName), err)
	}

	// Wait for VM to be running
	timeout := DefaultVMBootTimeoutSeconds * time.Second
	if t := os.Getenv("VM_BOOT_TIMEOUT"); t != "" {
		if parsedTimeout, err := time.ParseDuration(t); err == nil {
			timeout = parsedTimeout
		}
	}

	checkVMRunning := func() bool {
		running, err := o.CheckVMStatusInOpenShift(vmName)
		return err == nil && running
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := WaitForCondition(ctx, checkVMRunning, timeout, DefaultPollingIntervalSeconds*time.Second, "VM to start", o.logger); err != nil {
		return fmt.Errorf("VM failed to start: %v", err)
	}

	o.logger.LogInfo("VM '%s' started successfully", vmName)
	return nil
}

// CleanupOpenShiftResources cleans up OpenShift resources
func (o *OpenShiftClient) CleanupOpenShiftResources(planName, migrationName, storageMapName, networkMapName, vmName, secretName string) error {
	o.logger.LogInfo("Cleaning up OpenShift resources")

	namespace := strings.Trim(os.Getenv("FORKLIFT_NAMESPACE"), `"'`)
	if namespace == "" {
		namespace = "openshift-mtv"
	}
	ocpNamespace := strings.Trim(os.Getenv("OCP_NAMESPACE"), `"'`)

	// Delete migration
	if migrationName != "" {
		cmd := o.ExecOcCommand("delete", "migration", migrationName, "-n", namespace, "--ignore-not-found=true")
		if output, err := cmd.CombinedOutput(); err != nil {
			o.logger.LogWarn("Failed to delete migration '%s': %v. Output: %s", migrationName, err, string(output))
		} else {
			o.logger.LogInfo("✅ Deleted migration '%s'", migrationName)
		}
	}

	// Clean up populator resources first (before deleting plan)
	if planName != "" {
		o.cleanupPopulatorResources(planName, namespace)
		o.cleanupPopulatorPods(planName, namespace)
	}

	// Clean up all pods related to this test BEFORE cleaning PVCs
	if vmName != "" {
		o.cleanupAllPods(vmName, namespace, ocpNamespace)
		// Wait a bit for pods to be fully terminated before deleting PVCs
		time.Sleep(5 * time.Second)
	}

	// Clean up all PVCs related to this test (after pods are deleted)
	if vmName != "" {
		o.cleanupAllPVCs(vmName, ocpNamespace)
	}

	// Clean up all secrets related to this test
	if vmName != "" || secretName != "" {
		o.cleanupAllSecrets(vmName, secretName, namespace, ocpNamespace)
	}

	// Delete VM
	if vmName != "" {
		cmd := o.ExecOcCommand("delete", "vm", SanitizeName(vmName), "-n", ocpNamespace, "--ignore-not-found=true")
		if output, err := cmd.CombinedOutput(); err != nil {
			o.logger.LogWarn("Failed to delete VM '%s': %v. Output: %s", SanitizeName(vmName), err, string(output))
		} else {
			o.logger.LogInfo("✅ Deleted VM '%s'", SanitizeName(vmName))
		}
	}

	// Delete plan
	if planName != "" {
		cmd := o.ExecOcCommand("delete", "plan", planName, "-n", namespace, "--ignore-not-found=true")
		if output, err := cmd.CombinedOutput(); err != nil {
			o.logger.LogWarn("Failed to delete plan '%s': %v. Output: %s", planName, err, string(output))
		} else {
			o.logger.LogInfo("✅ Deleted plan '%s'", planName)
		}
	}

	// Delete storage map
	if storageMapName != "" {
		cmd := o.ExecOcCommand("delete", "storagemap", storageMapName, "-n", namespace, "--ignore-not-found=true")
		if output, err := cmd.CombinedOutput(); err != nil {
			o.logger.LogWarn("Failed to delete storage map '%s': %v. Output: %s", storageMapName, err, string(output))
		} else {
			o.logger.LogInfo("✅ Deleted storage map '%s'", storageMapName)
		}
	}

	// Delete network map
	if networkMapName != "" {
		cmd := o.ExecOcCommand("delete", "networkmap", networkMapName, "-n", namespace, "--ignore-not-found=true")
		if output, err := cmd.CombinedOutput(); err != nil {
			o.logger.LogWarn("Failed to delete network map '%s': %v. Output: %s", networkMapName, err, string(output))
		} else {
			o.logger.LogInfo("✅ Deleted network map '%s'", networkMapName)
		}
	}

	// Delete storage secret
	if secretName != "" {
		cmd := o.ExecOcCommand("delete", "secret", secretName, "-n", namespace, "--ignore-not-found=true")
		if output, err := cmd.CombinedOutput(); err != nil {
			o.logger.LogWarn("Failed to delete storage secret '%s': %v. Output: %s", secretName, err, string(output))
		} else {
			o.logger.LogInfo("✅ Deleted storage secret '%s'", secretName)
		}
	}

	o.logger.LogInfo("OpenShift resource cleanup completed")
	return nil
}

// cleanupPopulatorPods deletes populator pods associated with a specific migration plan.
func (o *OpenShiftClient) cleanupPopulatorPods(planName, namespace string) {
	o.logger.LogInfo("Cleaning up populator pods for plan '%s'", planName)

	// Clean by label selector
	labelSelector := fmt.Sprintf("plan.forklift.konveyor.io/name=%s", planName)
	deleteCmd := o.ExecOcCommand("delete", "pod", "-n", namespace, "-l", labelSelector, "--ignore-not-found=true", "--now")
	if output, err := deleteCmd.CombinedOutput(); err != nil {
		o.logger.LogWarn("Could not delete populator pods by label for plan '%s': %v. Output: %s", planName, err, string(output))
	} else {
		o.logger.LogInfo("Successfully cleaned up labeled populator pods for plan '%s'.", planName)
	}
}

// cleanupPopulatorResources removes populator custom resources and their PVCs.
func (o *OpenShiftClient) cleanupPopulatorResources(planName, namespace string) {
	o.logger.LogInfo("Cleaning up VSphereXcopyVolumePopulator resources for plan '%s'", planName)

	// Find all populator custom resources that match the plan name prefix.
	getPopulatorsCmd := o.ExecOcCommand("get", "vspherexcopyvolumepopulator", "-n", namespace, "-o", "json")
	output, err := getPopulatorsCmd.Output()
	if err != nil {
		o.logger.LogWarn("Could not list VSphereXcopyVolumePopulator resources to clean up.")
		return
	}

	var populatorList struct {
		Items []struct {
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
			Status struct {
				PVC string `json:"pvc"`
			} `json:"status"`
		} `json:"items"`
	}

	if err := json.Unmarshal(output, &populatorList); err != nil {
		o.logger.LogWarn("Could not unmarshal VSphereXcopyVolumePopulator list: %v", err)
		return
	}

	// Extract VM name from plan name (remove "-plan" suffix)
	vmName := strings.TrimSuffix(planName, "-plan")
	diskPrefix := vmName + "-disk-"

	for _, populator := range populatorList.Items {
		populatorName := populator.Metadata.Name
		// VSphereXcopyVolumePopulator names follow pattern: {vmName}-disk-{index}-{uuid}
		if strings.HasPrefix(populatorName, diskPrefix) {
			o.logger.LogInfo("Deleting VSphereXcopyVolumePopulator: %s", populatorName)
			deletePopulatorCmd := o.ExecOcCommand("delete", "vspherexcopyvolumepopulator", populatorName, "-n", namespace, "--ignore-not-found=true")
			if output, err := deletePopulatorCmd.CombinedOutput(); err != nil {
				o.logger.LogError("Failed to delete VSphereXcopyVolumePopulator '%s': %v. Output: %s", populatorName, err, string(output))
			} else {
				o.logger.LogInfo("Successfully deleted VSphereXcopyVolumePopulator '%s'. Output: %s", populatorName, string(output))
			}

			// Also delete the associated PVC if it's found in the status.
			if populator.Status.PVC != "" {
				pvcName := populator.Status.PVC
				o.logger.LogInfo("Deleting associated PVC: %s", pvcName)
				deletePvcCmd := o.ExecOcCommand("delete", "pvc", pvcName, "-n", namespace, "--ignore-not-found=true")
				if output, err := deletePvcCmd.CombinedOutput(); err != nil {
					o.logger.LogError("Failed to delete PVC '%s': %v. Output: %s", pvcName, err, string(output))
				} else {
					o.logger.LogInfo("Successfully deleted PVC '%s'. Output: %s", pvcName, string(output))
				}
			}
		}
	}
}

// InitOpenShift initializes OpenShift environment
func (o *OpenShiftClient) InitOpenShift() error {
	o.logger.LogInfo("Initializing OpenShift environment")

	// Check required variables
	requiredVars := []string{
		"OCP_API_URL",
		"FORKLIFT_NAMESPACE",
		"OCP_STORAGE_CLASS",
	}

	// Conditionally check for username/password or token
	if os.Getenv("OCP_TOKEN") == "" {
		requiredVars = append(requiredVars, "OCP_USERNAME", "OCP_PASSWORD")
	}

	if err := CheckRequiredVars(requiredVars...); err != nil {
		return fmt.Errorf("required variables check failed: %v", err)
	}

	// Login to cluster
	if err := o.Login(); err != nil {
		return fmt.Errorf("OpenShift login failed: %v", err)
	}

	// Check Forklift installation
	if err := o.CheckForkliftInstallation(); err != nil {
		return fmt.Errorf("Forklift installation check failed: %v", err)
	}

	// Enable copy-offload feature
	if err := o.EnableCopyOffloadFeature(); err != nil {
		return fmt.Errorf("copy-offload feature enablement failed: %v", err)
	}

	o.logger.LogInfo("OpenShift environment initialized successfully")
	return nil
}

// GetPopulatorPodLogs retrieves logs from a populator pod associated with a PVC.
func (o *OpenShiftClient) GetPopulatorPodLogs(pvcName string) (string, error) {
	namespace := strings.Trim(os.Getenv("FORKLIFT_NAMESPACE"), `"'`)
	if namespace == "" {
		namespace = "openshift-mtv"
	}

	// Find the pod name using the PVC label selector.
	// Populator pods are labeled with `cdi.kubevirt.io/storage.populator.pvc.name=<pvc-name>`.
	labelSelector := fmt.Sprintf("cdi.kubevirt.io/storage.populator.pvc.name=%s", pvcName)
	cmd := o.ExecOcCommand("get", "pods", "-n", namespace, "-l", labelSelector, "-o", "jsonpath={.items[0].metadata.name}")
	podNameBytes, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to find populator pod for PVC %s: %w", pvcName, err)
	}

	podName := strings.TrimSpace(string(podNameBytes))
	if podName == "" {
		return "", fmt.Errorf("no populator pod found for PVC %s", pvcName)
	}

	// Retrieve the logs for the identified pod.
	logsCmd := o.ExecOcCommand("logs", podName, "-n", namespace)
	logsBytes, err := logsCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get logs for pod %s: %w. Output: %s", podName, err, string(logsBytes))
	}

	return string(logsBytes), nil
}

// cleanupAllPVCs deletes all PVCs related to a VM migration
func (o *OpenShiftClient) cleanupAllPVCs(vmName, namespace string) {
	if vmName == "" || namespace == "" {
		return
	}

	o.logger.LogInfo("Cleaning up all PVCs for VM '%s' in namespace '%s'", vmName, namespace)

	sanitizedVMName := SanitizeName(vmName)

	// Get all PVCs and check if they contain the VM name
	cmd := o.ExecOcCommand("get", "pvc", "-n", namespace, "--no-headers", "-o", "custom-columns=NAME:.metadata.name")
	output, err := cmd.Output()
	if err != nil {
		o.logger.LogWarn("Failed to list PVCs in namespace '%s': %v", namespace, err)
		return
	}

	pvcNames := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, pvcName := range pvcNames {
		pvcName = strings.TrimSpace(pvcName)
		if pvcName == "" || pvcName == "NAME" {
			continue
		}

		// Check if PVC name contains the VM name
		if strings.Contains(pvcName, sanitizedVMName) {
			o.logger.LogInfo("Deleting PVC: %s", pvcName)
			deleteCmd := o.ExecOcCommand("delete", "pvc", pvcName, "-n", namespace, "--ignore-not-found=true", "--timeout=60s")
			if output, err := deleteCmd.CombinedOutput(); err != nil {
				o.logger.LogWarn("Failed to delete PVC '%s': %v. Output: %s", pvcName, err, string(output))
				// If normal deletion fails due to finalizers, try to patch finalizers
				o.logger.LogInfo("Attempting to remove finalizers from PVC '%s'", pvcName)
				patchCmd := o.ExecOcCommand("patch", "pvc", pvcName, "-n", namespace, "--type=merge", "-p", `{"metadata":{"finalizers":null}}`)
				if patchOutput, patchErr := patchCmd.CombinedOutput(); patchErr != nil {
					o.logger.LogWarn("Failed to remove finalizers from PVC '%s': %v. Output: %s", pvcName, patchErr, string(patchOutput))
				} else {
					o.logger.LogInfo("Removed finalizers from PVC '%s'", pvcName)
				}
			} else {
				o.logger.LogInfo("✅ Deleted PVC '%s'", pvcName)
			}
		}
	}
}

// cleanupAllPods deletes all pods related to a VM migration
func (o *OpenShiftClient) cleanupAllPods(vmName, forkliftNamespace, ocpNamespace string) {
	if vmName == "" {
		return
	}

	o.logger.LogInfo("Cleaning up all pods related to VM '%s'", vmName)

	sanitizedVMName := SanitizeName(vmName)
	namespaces := []string{forkliftNamespace, ocpNamespace}

	for _, namespace := range namespaces {
		if namespace == "" {
			continue
		}

		o.logger.LogInfo("Checking for pods in namespace '%s'", namespace)

		// Get all pods in the namespace
		cmd := o.ExecOcCommand("get", "pods", "-n", namespace, "--no-headers", "-o", "custom-columns=NAME:.metadata.name")
		output, err := cmd.Output()
		if err != nil {
			o.logger.LogWarn("Failed to list pods in namespace '%s': %v", namespace, err)
			continue
		}

		podNames := strings.Split(strings.TrimSpace(string(output)), "\n")
		for _, podName := range podNames {
			podName = strings.TrimSpace(podName)
			if podName == "" || podName == "NAME" {
				continue
			}

			// Check if pod is related to our migration
			shouldDelete := false
			if strings.Contains(podName, sanitizedVMName) {
				shouldDelete = true
			} else if strings.Contains(podName, "populator") && strings.Contains(podName, sanitizedVMName) {
				shouldDelete = true
			} else if strings.Contains(podName, "conversion") && strings.Contains(podName, sanitizedVMName) {
				shouldDelete = true
			} else if strings.Contains(podName, "virt-launcher") && strings.Contains(podName, sanitizedVMName) {
				shouldDelete = true
			}

			if shouldDelete {
				o.logger.LogInfo("Deleting pod: %s (namespace: %s)", podName, namespace)
				deleteCmd := o.ExecOcCommand("delete", "pod", podName, "-n", namespace, "--ignore-not-found=true", "--force", "--grace-period=0")
				if output, err := deleteCmd.CombinedOutput(); err != nil {
					o.logger.LogWarn("Failed to delete pod '%s': %v. Output: %s", podName, err, string(output))
				} else {
					o.logger.LogInfo("✅ Deleted pod '%s'", podName)
				}
			}
		}
	}
}

// cleanupAllSecrets deletes all secrets related to a VM migration
func (o *OpenShiftClient) cleanupAllSecrets(vmName, storageSecretName, forkliftNamespace, ocpNamespace string) {
	o.logger.LogInfo("Cleaning up all secrets related to migration")

	namespaces := []string{forkliftNamespace, ocpNamespace}
	sanitizedVMName := ""
	if vmName != "" {
		sanitizedVMName = SanitizeName(vmName)
	}

	for _, namespace := range namespaces {
		if namespace == "" {
			continue
		}

		o.logger.LogInfo("Checking for secrets in namespace '%s'", namespace)

		// Get all secrets in the namespace
		cmd := o.ExecOcCommand("get", "secrets", "-n", namespace, "--no-headers", "-o", "custom-columns=NAME:.metadata.name")
		output, err := cmd.Output()
		if err != nil {
			o.logger.LogWarn("Failed to list secrets in namespace '%s': %v", namespace, err)
			continue
		}

		secretNames := strings.Split(strings.TrimSpace(string(output)), "\n")
		for _, secretName := range secretNames {
			secretName = strings.TrimSpace(secretName)
			if secretName == "" || secretName == "NAME" {
				continue
			}

			// Skip system secrets
			if strings.HasPrefix(secretName, "default-token-") ||
				strings.HasPrefix(secretName, "builder-token-") ||
				strings.HasPrefix(secretName, "deployer-token-") ||
				strings.HasPrefix(secretName, "sh.helm.release.") ||
				strings.Contains(secretName, "-dockercfg-") ||
				strings.Contains(secretName, "service-ca") {
				continue
			}

			// Check if secret is related to our migration
			shouldDelete := false
			if storageSecretName != "" && secretName == storageSecretName {
				shouldDelete = true
			} else if sanitizedVMName != "" && strings.Contains(secretName, sanitizedVMName) {
				shouldDelete = true
			} else if strings.Contains(secretName, "storage-secret") && sanitizedVMName != "" && strings.Contains(secretName, sanitizedVMName) {
				shouldDelete = true
			}

			if shouldDelete {
				o.logger.LogInfo("Deleting secret: %s (namespace: %s)", secretName, namespace)
				deleteCmd := o.ExecOcCommand("delete", "secret", secretName, "-n", namespace, "--ignore-not-found=true")
				if output, err := deleteCmd.CombinedOutput(); err != nil {
					o.logger.LogWarn("Failed to delete secret '%s': %v. Output: %s", secretName, err, string(output))
				} else {
					o.logger.LogInfo("✅ Deleted secret '%s'", secretName)
				}
			}
		}
	}
}
