package provider

import (
	"context"
	"crypto/x509"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/base"
	"github.com/kubev2v/forklift/pkg/controller/provider/container"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	libref "github.com/kubev2v/forklift/pkg/lib/ref"
	"github.com/kubev2v/forklift/pkg/lib/util"
	core "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubev2v/forklift/pkg/controller/provider/web"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/vsphere"
	"github.com/kubev2v/forklift/pkg/lib/sshkeys"
)

// Types
const (
	UrlNotValid             = "UrlNotValid"
	TypeNotSupported        = "ProviderTypeNotSupported"
	SecretNotValid          = "SecretNotValid"
	SettingsNotValid        = "SettingsNotValid"
	Validated               = "Validated"
	ConnectionAuthFailed    = "ConnectionAuthFailed"
	ConnectionTestSucceeded = "ConnectionTestSucceeded"
	ConnectionTestFailed    = "ConnectionTestFailed"
	InventoryCreated        = "InventoryCreated"
	LoadInventory           = "LoadInventory"
	InventoryError          = "InventoryError"
	ConnectionInsecure      = "ConnectionInsecure"
	SSHReadiness            = "SSHReadiness"
)

// Categories
const (
	Required = libcnd.Required
	Advisory = libcnd.Advisory
	Critical = libcnd.Critical
	Error    = libcnd.Error
	Warn     = libcnd.Warn
)

// Reasons
const (
	NotSet              = "NotSet"
	NotFound            = "NotFound"
	NotSupported        = "NotSupported"
	DataErr             = "DataErr"
	Malformed           = "Malformed"
	Completed           = "Completed"
	Tested              = "Tested"
	Started             = "Started"
	SkipTLSVerification = "SkipTLSVerification"
)

// Phases
const (
	ValidationFailed = "ValidationFailed"
	ConnectionFailed = "ConnectionFailed"
	Ready            = "Ready"
	Staging          = "Staging"
)

// Statuses
const (
	True  = libcnd.True
	False = libcnd.False
)

// Validate the provider resource.
func (r *Reconciler) validate(provider *api.Provider) error {
	err := r.validateType(provider)
	if err != nil {
		return liberr.Wrap(err)
	}
	err = r.validateURL(provider)
	if err != nil {
		return liberr.Wrap(err)
	}
	secret, err := r.validateSecret(provider)
	if err != nil {
		return liberr.Wrap(err)
	}
	err = r.testConnection(provider, secret)
	if err != nil {
		return liberr.Wrap(err)
	}
	err = r.inventoryCreated(provider)
	if err != nil {
		return liberr.Wrap(err)
	}

	// Validate SSH readiness for vSphere providers
	err = r.validateSSHReadiness(provider, secret)
	if err != nil {
		return liberr.Wrap(err)
	}

	if !provider.Status.HasBlockerCondition() {
		provider.Status.SetCondition(
			libcnd.Condition{
				Type:     Validated,
				Status:   True,
				Reason:   Completed,
				Category: Advisory,
				Message:  "Validation has been completed.",
			})
	}

	return nil
}

// Validate types.
func (r *Reconciler) validateType(provider *api.Provider) error {
	for _, p := range api.ProviderTypes {
		if p == provider.Type() {
			return nil
		}
	}

	provider.Status.Phase = ValidationFailed
	provider.Status.SetCondition(
		libcnd.Condition{
			Type:     TypeNotSupported,
			Status:   True,
			Reason:   NotSupported,
			Category: Critical,
			Message:  fmt.Sprintf("The `type` must be: %s", api.ProviderTypes),
		})

	return nil
}

// Validate the URL.
func (r *Reconciler) validateURL(provider *api.Provider) error {
	if provider.IsHost() {
		return nil
	}
	if provider.Spec.URL == "" {
		provider.Status.Phase = ValidationFailed
		provider.Status.SetCondition(
			libcnd.Condition{
				Type:     UrlNotValid,
				Status:   True,
				Reason:   NotSet,
				Category: Critical,
				Message:  "The `url` is not valid.",
			})
	}
	if provider.Type() == api.Ova {
		if !isValidNFSPath(provider.Spec.URL) {
			provider.Status.Phase = ValidationFailed
			provider.Status.SetCondition(
				libcnd.Condition{
					Type:     UrlNotValid,
					Status:   True,
					Reason:   Malformed,
					Category: Critical,
					Message:  "The NFS path is malformed",
				})
		}
		return nil
	}
	_, err := url.Parse(provider.Spec.URL)
	if err != nil {
		provider.Status.Phase = ValidationFailed
		provider.Status.SetCondition(
			libcnd.Condition{
				Type:     UrlNotValid,
				Status:   True,
				Reason:   Malformed,
				Category: Critical,
				Message:  fmt.Sprintf("The `url` is malformed: %s", err.Error()),
			})
	}

	return nil
}

func (r *Reconciler) validateConnectionStatus(provider *api.Provider, secret *core.Secret) {
	if base.GetInsecureSkipVerifyFlag(secret) {
		provider.Status.SetCondition(libcnd.Condition{
			Type:     ConnectionInsecure,
			Status:   True,
			Reason:   SkipTLSVerification,
			Category: Warn,
			Message:  "TLS is susceptible to machine-in-the-middle attacks when certificate verification is skipped.",
		})
	} else {
		_, err := base.VerifyTLSConnection(provider.Spec.URL, secret)
		if err != nil {
			provider.Status.SetCondition(libcnd.Condition{
				Type:     ConnectionTestFailed,
				Status:   True,
				Reason:   Tested,
				Category: Critical,
				Message:  err.Error(),
			})
		}
	}
}

// Validate secret (ref).
//  1. The references is complete.
//  2. The secret exists.
//  3. the content of the secret is valid.
func (r *Reconciler) validateSecret(provider *api.Provider) (secret *core.Secret, err error) {
	if provider.IsHost() {
		return
	}
	// NotSet
	newCnd := libcnd.Condition{
		Type:     SecretNotValid,
		Status:   True,
		Reason:   NotSet,
		Category: Critical,
		Message:  "The `secret` is not valid.",
	}
	ref := provider.Spec.Secret
	if !libref.RefSet(&ref) {
		provider.Status.Phase = ValidationFailed
		provider.Status.SetCondition(newCnd)
		return
	}
	// NotFound
	secret = &core.Secret{}
	key := client.ObjectKey{
		Namespace: ref.Namespace,
		Name:      ref.Name,
	}
	err = r.Get(context.TODO(), key, secret)
	if k8serrors.IsNotFound(err) {
		err = nil
		newCnd.Reason = NotFound
		provider.Status.Phase = ValidationFailed
		provider.Status.SetCondition(newCnd)
		return
	}
	if err != nil {
		err = liberr.Wrap(err)
	}
	// DataErr
	keyList := []string{}
	switch provider.Type() {
	case api.OpenShift:
		keyList = []string{"token"}

	case api.VSphere:
		keyList = []string{
			"user",
			"password",
		}

		r.validateConnectionStatus(provider, secret)

		var providerUrl *url.URL
		providerUrl, err = url.Parse(provider.Spec.URL)
		if err != nil {
			return
		}
		var crt *x509.Certificate
		crt, err = util.GetTlsCertificate(providerUrl, secret)
		if err != nil {
			provider.Status.Phase = ConnectionFailed
			provider.Status.SetCondition(libcnd.Condition{
				Type:     ConnectionTestFailed,
				Status:   True,
				Reason:   Tested,
				Category: Critical,
				Message:  err.Error(),
			})
			return
		}
		provider.Status.Fingerprint = util.Fingerprint(crt)
	case api.OVirt:
		keyList = []string{
			"user",
			"password",
		}

		if base.GetInsecureSkipVerifyFlag(secret) {
			provider.Status.SetCondition(libcnd.Condition{
				Type:     ConnectionInsecure,
				Status:   True,
				Reason:   SkipTLSVerification,
				Category: Warn,
				Message:  "TLS is susceptible to machine-in-the-middle attacks when certificate verification is skipped.",
			})
		} else {
			keyList = append(keyList, "cacert")
		}
	case api.Ova:
		keyList = []string{
			"url",
		}
	}
	for _, key := range keyList {
		if _, found := secret.Data[key]; !found {
			newCnd.Items = append(newCnd.Items, key)
		}
	}
	if len(newCnd.Items) > 0 {
		newCnd.Reason = DataErr
		provider.Status.Phase = ValidationFailed
		provider.Status.SetCondition(newCnd)
	}

	return
}

// Test connection.
func (r *Reconciler) testConnection(provider *api.Provider, secret *core.Secret) error {
	if provider.Status.HasBlockerCondition() {
		return nil
	}
	rl := container.Build(nil, provider, secret)
	status, err := rl.Test()
	if err == nil {
		log.Info(
			"Connection test succeeded.")
		provider.Status.SetCondition(
			libcnd.Condition{
				Type:     ConnectionTestSucceeded,
				Status:   True,
				Reason:   Tested,
				Category: Required,
				Message:  "Connection test, succeeded.",
			})
	} else {
		// When the status is unauthorized controller stops the reconciliation, so the user account does not get locked.
		// Providing bad credentials when requesting the token results in 400, and not 401.
		if status == http.StatusUnauthorized || status == http.StatusBadRequest {
			provider.Status.Phase = ConnectionFailed
			provider.Status.SetCondition(
				libcnd.Condition{
					Type:     ConnectionAuthFailed,
					Status:   True,
					Reason:   Tested,
					Category: Critical,
					Message: fmt.Sprintf(
						"Connection auth failed, error: %s",
						err.Error()),
				})
			return nil
		}
		log.Info(
			"Connection test failed.",
			"reason",
			err.Error())
		provider.Status.Phase = ConnectionFailed
		provider.Status.SetCondition(
			libcnd.Condition{
				Type:     ConnectionTestFailed,
				Status:   True,
				Reason:   Tested,
				Category: Critical,
				Message: fmt.Sprintf(
					"Connection test, failed: %s",
					err.Error()),
			})
	}

	return nil
}

// Validate inventory created.
func (r *Reconciler) inventoryCreated(provider *api.Provider) error {
	if provider.Status.HasBlockerCondition() {
		return nil
	}
	if r, found := r.container.Get(provider); found {
		// Check for runtime errors from any collector that supports them
		if errors := r.GetRuntimeErrors(); len(errors) > 0 {
			// Set inventory error condition for any runtime errors
			var errorMessages []string
			for kind, err := range errors {
				errorMessages = append(errorMessages, fmt.Sprintf("%s: %s", kind, err.Error()))
			}
			provider.Status.SetCondition(
				libcnd.Condition{
					Type:     InventoryError,
					Status:   True,
					Reason:   "RuntimeError",
					Category: Error,
					Message:  fmt.Sprintf("Inventory runtime errors: %v", errorMessages),
				})
			return nil
		}

		if r.HasParity() {
			provider.Status.SetCondition(
				libcnd.Condition{
					Type:     InventoryCreated,
					Status:   True,
					Reason:   Completed,
					Category: Required,
					Message:  "The inventory has been loaded.",
				})
		} else {
			provider.Status.SetCondition(
				libcnd.Condition{
					Type:     LoadInventory,
					Status:   True,
					Reason:   Started,
					Category: Advisory,
					Message:  "Loading the inventory.",
				})
		}
	} else {
		log.Info("No collector found", "provider", provider)
	}

	return nil
}

func (r *Reconciler) handleServerCreationFailure(provider *api.Provider, err error) {
	provider.Status.Phase = ConnectionFailed
	msg := fmt.Sprint("The OVA provider server creation failed: ", err)
	provider.Status.SetCondition(
		libcnd.Condition{
			Type:     ConnectionFailed,
			Status:   True,
			Category: Critical,
			Message:  msg,
		})
	if updateErr := r.Status().Update(context.TODO(), provider.DeepCopy()); updateErr != nil {
		log.Error(updateErr, "Failed to update provider status")
	}
}

func isValidNFSPath(nfsPath string) bool {
	nfsRegex := `^[^:]+:\/[^:].*$`
	re := regexp.MustCompile(nfsRegex)
	return re.MatchString(nfsPath)
}

// Validate SSH readiness for vSphere providers with ESXi 7 or lower hosts.
// Uses the exact same SSH key management and connectivity patterns as vsphere-xcopy-volume-populator.
func (r *Reconciler) validateSSHReadiness(provider *api.Provider, secret *core.Secret) error {
	// CRITICAL: Only check for vSphere providers - return immediately for all others
	if provider.Type() != api.VSphere {
		return nil
	}

	sshMethodEnabled := !r.isSSHMethodEnabled(provider)
	r.Log.Info(">>>>>>>>>>>>>>>>>> SSHMethodEnabled setting", "provider", provider.Name, "enabled", sshMethodEnabled)
	if !sshMethodEnabled {
		r.Log.V(1).Info("Skipping validation of SSH method since it is not enabled", "provider", provider.Name)
		return nil
	}

	// Log with proper nil handling for secret
	secretName := "<nil>"
	if secret != nil {
		secretName = secret.Name
	}
	r.Log.Info(">>>>>>>>>>>>>>>>>> validating SSHReadiness", "provider", provider.Name, "secret", secretName)

	// Check if provider has ESXi hosts and their versions
	hasOldESXiHosts, err := r.hasESXi7OrLowerHosts(provider, secret)
	if err != nil {
		// If we can't determine ESXi versions, set SSH readiness to false
		r.Log.V(1).Info("Could not determine ESXi versions, setting SSH readiness to false", "error", err)
		provider.Status.SetCondition(libcnd.Condition{
			Type:     SSHReadiness,
			Status:   False,
			Reason:   "InventoryCheckFailed",
			Category: Error,
			Message:  fmt.Sprintf("Failed to check ESXi host versions from inventory: %v", err),
		})
		return nil
	}

	// Check if ESXiAutoKeyInstall is enabled (default is true)
	autoKeyInstallEnabled := !r.isAutoKeyInstallDisabled(provider)
	r.Log.Info(">>>>>>>>>>>>>>>>>> ESXiAutoKeyInstall setting", "provider", provider.Name, "enabled", autoKeyInstallEnabled)

	if autoKeyInstallEnabled {
		// ESXiAutoKeyInstall is true:
		// If all ESXi version 7 hosts pass connectivity test, provider is ready
		// because for ESXi version 8+ autoinstall will occur
		if !hasOldESXiHosts {
			// No ESXi 7 or lower hosts, SSH readiness is not required
			provider.Status.SetCondition(libcnd.Condition{
				Type:     SSHReadiness,
				Status:   True,
				Reason:   "NoOldESXiHosts",
				Category: Advisory,
				Message:  "No ESXi 7 or lower hosts detected, SSH not required.",
			})
			return nil
		}

		// Test SSH connectivity only for ESXi 7 or lower hosts
		return r.validateSSHConnectivityForHosts(provider, secret, true)
	} else {
		// ESXiAutoKeyInstall is false:
		// Treat all ESXi hosts equally - if any don't pass connectivity test, provider is not ready
		return r.validateSSHConnectivityForHosts(provider, secret, false)
	}
}

// validateSSHConnectivityForHosts validates SSH connectivity for ESXi hosts
// oldHostsOnly: if true, only test ESXi 7 or lower hosts; if false, test all hosts
func (r *Reconciler) validateSSHConnectivityForHosts(provider *api.Provider, secret *core.Secret, oldHostsOnly bool) error {
	// Check if SSH keys are available using provider name pattern
	privateKey, err := sshkeys.GetSSHPrivateKey(r.Client, provider.Name, provider.Namespace)
	if err != nil {
		var errorMessage string
		if oldHostsOnly {
			errorMessage = fmt.Sprintf("SSH keys not configured for provider with ESXi 7 or lower hosts: %v\n\n", err)
		} else {
			errorMessage = fmt.Sprintf("SSH keys not configured for provider: %v\n\n", err)
		}
		errorMessage += "To enable storage offload, SSH keys must be generated. The controller will automatically create SSH keys when the provider is ready.\n"
		errorMessage += "If keys exist but this error persists, check that the secret name follows the pattern: offload-ssh-keys-<provider-name>-private"

		provider.Status.SetCondition(libcnd.Condition{
			Type:     SSHReadiness,
			Status:   False,
			Reason:   "SSHKeysNotConfigured",
			Category: Error,
			Message:  errorMessage,
		})
		return nil
	}

	// Test SSH connectivity
	sshConnectivityOK, failedHosts, err := r.testSSHConnectivity(provider, privateKey, secret, oldHostsOnly)
	if err != nil {
		provider.Status.SetCondition(libcnd.Condition{
			Type:     SSHReadiness,
			Status:   False,
			Reason:   "SSHTestError",
			Category: Error,
			Message:  fmt.Sprintf("SSH connectivity test failed: %v", err),
		})
		return nil
	}

	if !sshConnectivityOK {
		hostDescription := "ESXi hosts"
		if oldHostsOnly {
			hostDescription = "ESXi 7 or lower hosts"
		}
		return r.setSSHConnectivityFailedCondition(provider, failedHosts, hostDescription)
	}

	// SSH connectivity test passed
	var successMessage string
	if oldHostsOnly {
		successMessage = "SSH connectivity verified for ESXi 7 or lower hosts."
	} else {
		successMessage = "SSH connectivity verified for all ESXi hosts."
	}

	provider.Status.SetCondition(libcnd.Condition{
		Type:     SSHReadiness,
		Status:   True,
		Reason:   "SSHTestSucceeded",
		Category: Required,
		Message:  successMessage,
	})

	return nil
}

// setSSHConnectivityFailedCondition sets the SSH connectivity failed condition with appropriate error message
func (r *Reconciler) setSSHConnectivityFailedCondition(provider *api.Provider, failedHosts []string, hostDescription string) error {
	// Get the public key to include in the error message
	publicKeyContent := ""
	publicSecretName := sshkeys.GenerateSSHPublicSecretName(provider.Name)
	publicSecret := &core.Secret{}
	err := r.Get(context.TODO(), client.ObjectKey{Name: publicSecretName, Namespace: provider.Namespace}, publicSecret)
	if err == nil {
		if pubKey, exists := publicSecret.Data["public-key"]; exists {
			publicKeyContent = string(pubKey)
		}
	}

	failedHostList := ""
	if len(failedHosts) > 0 {
		failedHostList = fmt.Sprintf(" Failed host IPs: %v.", failedHosts)
	}

	// Create error message with instructions
	var errorMessage string
	errorMessage = fmt.Sprintf("SSH connectivity test failed for %s.%s To enable storage offload, configure SSH access on your ESXi hosts:\n\n", hostDescription, failedHostList)
	errorMessage += "1. SSH to each ESXi host as root\n"
	errorMessage += "2. Add the SSH key using this 	d:\n"

	if publicKeyContent != "" {
		// Use shared version constant from sshkeys package
		scriptPath := fmt.Sprintf("/vmfs/volumes/<DATASTORE>/secure-vmkfstools-wrapper-%s.py", sshkeys.SecureScriptVersion)
		sshKeyLine := fmt.Sprintf("command=\"python %s\",no-port-forwarding,no-agent-forwarding,no-X11-forwarding %s", scriptPath, publicKeyContent)
		errorMessage += fmt.Sprintf("   echo '%s' >> /etc/ssh/keys-root/authorized_keys\n", sshKeyLine)
		errorMessage += "   (Replace <DATASTORE> with your actual datastore name)\n"
	} else {
		errorMessage += fmt.Sprintf("   echo 'command=\"python /vmfs/volumes/<DATASTORE>/secure-vmkfstools-wrapper-%s.py\",no-port-forwarding,no-agent-forwarding,no-X11-forwarding <PUBLIC_KEY_CONTENT>' >> /etc/ssh/keys-root/authorized_keys\n", sshkeys.SecureScriptVersion)
		errorMessage += "   (Replace <DATASTORE> with your actual datastore name and <PUBLIC_KEY_CONTENT> with the actual public key)\n"
	}

	provider.Status.SetCondition(libcnd.Condition{
		Type:     SSHReadiness,
		Status:   False,
		Reason:   "SSHTestFailed",
		Category: Error,
		Message:  errorMessage,
	})
	return nil
}

// Check if provider has ESXi 7 or lower hosts by querying Forklift inventory.
func (r *Reconciler) hasESXi7OrLowerHosts(provider *api.Provider, secret *core.Secret) (bool, error) {
	r.Log.Info(">>>>>>>>>>>>>>>>>> checking for ESXi hosts via Forklift inventory", "provider", provider.Name)
	// Only check for vSphere providers
	if provider.Type() != api.VSphere {
		r.Log.Info(">>>>>>>>>>>>>>>>>> not vSphere provider, skipping", "type", provider.Type())
		return false, nil
	}

	// Use Forklift inventory instead of direct vSphere API calls
	hasOldHosts, err := r.checkESXiVersionsFromInventory(provider)
	if err != nil {
		r.Log.Info(">>>>>>>>>>>>>>>>>> Failed to check ESXi versions from inventory", "provider", provider.Name, "error", err)
		return false, err
	}

	return hasOldHosts, nil
}

// checkESXiVersionsFromInventory checks ESXi host versions from Forklift inventory
func (r *Reconciler) checkESXiVersionsFromInventory(provider *api.Provider) (bool, error) {
	r.Log.Info(">>>>>>>>>>>>>>>>>> Getting ESXi host versions from Forklift inventory", "provider", provider.Name)

	// Create inventory client to access the cached host data
	inventory, err := web.NewClient(provider)
	if err != nil {
		return false, fmt.Errorf("failed to create inventory client: %w", err)
	}

	// List all hosts from the provider inventory
	var hosts []vsphere.Host
	err = inventory.List(&hosts)
	if err != nil {
		return false, fmt.Errorf("failed to list hosts from inventory: %w", err)
	}

	// Check each host's version
	for _, host := range hosts {
		if host.ProductVersion != "" {
			r.Log.Info(">>>>>>>>>>>>>>>>>> Found host version", "host", host.Name, "version", host.ProductVersion)

			if r.isESXi7OrLower(host.ProductVersion) {
				r.Log.Info(">>>>>>>>>>>>>>>>>> Found ESXi 7 or lower host", "host", host.Name, "version", host.ProductVersion)
				return true, nil
			}
		} else {
			r.Log.Info(">>>>>>>>>>>>>>>>>> Host has no product info", "host", host.Name)
		}
	}

	r.Log.Info(">>>>>>>>>>>>>>>>>> No ESXi 7 or lower hosts found", "provider", provider.Name)
	return false, nil
}

// Check if the given version string represents ESXi 7 or lower.
func (r *Reconciler) isESXi7OrLower(version string) bool {
	if version == "" {
		return false
	}

	parts := strings.Split(version, ".")
	if len(parts) < 1 {
		return false
	}

	majorVersion, err := strconv.Atoi(parts[0])
	if err != nil {
		r.Log.V(1).Info("Failed to parse ESXi version", "version", version, "error", err)
		return false
	}

	return majorVersion <= 7
}

// Test SSH connectivity to ESXi hosts for the provider using robust connectivity testing.
// oldHostsOnly: if true, only test ESXi 7 or lower hosts; if false, test all hosts
func (r *Reconciler) testSSHConnectivity(provider *api.Provider, privateKey []byte, secret *core.Secret, oldHostsOnly bool) (bool, []string, error) {
	// Get target hosts to test SSH connectivity
	hostIPs, err := r.getESXiHostIPs(provider, secret, oldHostsOnly)
	if err != nil {
		if oldHostsOnly {
			return false, nil, fmt.Errorf("failed to get ESXi 7 or lower host IPs: %w", err)
		}
		return false, nil, fmt.Errorf("failed to get ESXi host IPs: %w", err)
	}

	if len(hostIPs) == 0 {
		if oldHostsOnly {
			r.Log.V(1).Info("No ESXi 7 or lower host IPs found for SSH testing, skipping")
		} else {
			r.Log.V(1).Info("No ESXi host IPs found for SSH testing, skipping")
		}
		return true, nil, nil
	}

	var failedHosts []string
	// Test SSH connectivity to at least one host using robust testing
	for _, hostIP := range hostIPs {
		if r.testSSHConnectivityToHost(hostIP, privateKey) {
			if oldHostsOnly {
				r.Log.V(1).Info("SSH connectivity test succeeded for ESXi 7 or lower host", "host", hostIP)
			} else {
				r.Log.V(1).Info("SSH connectivity test succeeded", "host", hostIP)
			}
			return true, nil, nil
		}
		if oldHostsOnly {
			r.Log.V(1).Info("SSH connectivity test failed for ESXi 7 or lower host", "host", hostIP)
		} else {
			r.Log.V(1).Info("SSH connectivity test failed", "host", hostIP)
		}
		failedHosts = append(failedHosts, hostIP)
	}

	return false, failedHosts, nil
}

// Test SSH connectivity to a specific host using robust SSH testing logic.
// This verifies that the SSH key actually works for authentication and command execution.
func (r *Reconciler) testSSHConnectivityToHost(hostname string, privateKey []byte) bool {
	r.Log.V(1).Info("Testing SSH connectivity and authentication", "host", hostname)

	// Use the shared SSH connectivity test function
	result := sshkeys.TestSSHConnectivity(hostname, privateKey)

	if result {
		r.Log.V(1).Info("SSH connectivity test passed", "host", hostname)
	} else {
		r.Log.V(1).Info("SSH connectivity test failed", "host", hostname)
	}

	return result
}

// Get ESXi host IP addresses from provider inventory.
// oldHostsOnly: if true, only return ESXi 7 or lower hosts; if false, return all hosts
func (r *Reconciler) getESXiHostIPs(provider *api.Provider, secret *core.Secret, oldHostsOnly bool) ([]string, error) {
	if oldHostsOnly {
		r.Log.Info(">>>>>>>>>>>>>>>>>> Getting ESXi 7 or lower host IPs from provider inventory", "provider", provider.Name)
	} else {
		r.Log.Info(">>>>>>>>>>>>>>>>>> Getting ESXi host IPs from provider inventory", "provider", provider.Name)
	}

	// Create inventory client to access the cached host data
	inventory, err := web.NewClient(provider)
	if err != nil {
		return nil, fmt.Errorf("failed to create inventory client: %w", err)
	}

	// List all hosts from the provider inventory
	var hosts []vsphere.Host
	err = inventory.List(&hosts)
	if err != nil {
		return nil, fmt.Errorf("failed to list hosts from inventory: %w", err)
	}

	var hostIPs []string
	for _, host := range hosts {
		// Filter hosts based on oldHostsOnly flag
		if oldHostsOnly {
			// Only include ESXi 7 or lower hosts
			if host.ProductVersion != "" && r.isESXi7OrLower(host.ProductVersion) {
				if host.ManagementServerIp != "" {
					hostIPs = append(hostIPs, host.ManagementServerIp)
					r.Log.Info(">>>>>>>>>>>>>>>>>> Found ESXi 7 or lower host IP from inventory", "host", host.Name, "version", host.ProductVersion, "ip", host.ManagementServerIp)
				} else {
					r.Log.V(1).Info(">>>>>>>>>>>>>>>>>> ESXi 7 or lower host has no ManagementServerIp", "host", host.Name, "version", host.ProductVersion)
				}
			} else {
				r.Log.V(1).Info(">>>>>>>>>>>>>>>>>> Skipping ESXi 8+ or unknown version host", "host", host.Name, "version", host.ProductVersion)
			}
		} else {
			// Include all hosts
			if host.ManagementServerIp != "" {
				hostIPs = append(hostIPs, host.ManagementServerIp)
				r.Log.Info(">>>>>>>>>>>>>>>>>> Found ESXi host IP from inventory (using ManagementServerIp)", "host", host.Name, "ip", host.ManagementServerIp)
			} else {
				r.Log.V(1).Info(">>>>>>>>>>>>>>>>>> Host has no ManagementServerIp", "host", host.Name)
			}
		}
	}

	if oldHostsOnly {
		r.Log.Info(">>>>>>>>>>>>>>>>>> Collected ESXi 7 or lower host IPs from inventory", "provider", provider.Name, "count", len(hostIPs))
	} else {
		r.Log.Info(">>>>>>>>>>>>>>>>>> Collected host IPs from inventory", "provider", provider.Name, "count", len(hostIPs))
	}
	return hostIPs, nil
}

// isAutoKeyInstallDisabled checks if automatic SSH key installation is disabled for this provider.
// Returns true if disabled, false if enabled (default is enabled).
func (r *Reconciler) isAutoKeyInstallDisabled(provider *api.Provider) bool {
	// Check if the provider has the ESXiAutoKeyInstall setting
	if provider.Spec.Settings != nil {
		if autoKeyInstall, exists := provider.Spec.Settings[api.ESXiAutoKeyInstall]; exists {
			// If the setting is explicitly set to "false", disable automatic installation
			if autoKeyInstall == "false" {
				r.Log.Info("Automatic SSH key installation disabled for provider", "provider", provider.Name)
				return true
			}
		}
	}

	// Default behavior: automatic key installation is enabled
	return false
}

func (r *Reconciler) isSSHMethodEnabled(provider *api.Provider) bool {
	if provider.Spec.Settings != nil {
		if esxiCloneMethod, exists := provider.Spec.Settings[api.ESXiCloneMethod]; exists {
			if esxiCloneMethod != "ssh" {
				r.Log.Info("SSH copy method is not enabled.", "provider", provider.Name)
				return true
			}
		}
	}

	// Default behavior: ssh method installation is disabled
	return false
}
