package provider

import (
	"context"
	"crypto/x509"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/kubev2v/forklift/pkg/lib/inventory/model"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/base"
	"github.com/kubev2v/forklift/pkg/controller/provider/container"
	vsphere "github.com/kubev2v/forklift/pkg/controller/provider/model/vsphere"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	libref "github.com/kubev2v/forklift/pkg/lib/ref"
	"github.com/kubev2v/forklift/pkg/lib/util"
	core "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

	// Validate SSH readiness for vSphere providers when SSH method is enabled
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
			err = nil
			//nolint:nilerr
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

func isValidNFSPath(nfsPath string) bool {
	nfsRegex := `^[^:]+:\/[^:].*$`
	re := regexp.MustCompile(nfsRegex)
	return re.MatchString(nfsPath)
}

// hostInfo holds information about a host for SSH testing
type hostInfo struct {
	name string
	ip   string
}

// getHostsForSSHValidation retrieves the list of hosts to test for SSH readiness
// Returns a slice of hostInfo with name and IP, or sets a condition and returns nil on error
func (r *Reconciler) getHostsForSSHValidation(provider *api.Provider) []hostInfo {
	sdkEndpoint := provider.Spec.Settings[api.SDK]
	isDirectESXi := (sdkEndpoint == api.ESXI)

	r.Log.Info("SSH validation: provider connection type",
		"provider", provider.Name,
		"sdkEndpoint", sdkEndpoint,
		"isDirectESXi", isDirectESXi)

	if isDirectESXi {
		// For direct ESXi connections, extract IP from provider URL
		r.Log.Info("SSH validation: direct ESXi connection detected, extracting IP from provider URL",
			"provider", provider.Name,
			"providerURL", provider.Spec.URL)

		providerURL, err := url.Parse(provider.Spec.URL)
		if err != nil {
			provider.Status.SetCondition(libcnd.Condition{
				Type:     SSHReadiness,
				Status:   False,
				Reason:   "InvalidProviderURL",
				Category: Warn,
				Message:  fmt.Sprintf("Cannot validate SSH readiness: failed to parse provider URL: %v", err),
			})
			return nil
		}

		// Extract hostname/IP from URL (could be hostname:port or just hostname)
		hostIP := providerURL.Hostname()
		if hostIP == "" {
			provider.Status.SetCondition(libcnd.Condition{
				Type:     SSHReadiness,
				Status:   False,
				Reason:   "NoHostIPInURL",
				Category: Warn,
				Message:  fmt.Sprintf("Cannot validate SSH readiness: no host/IP found in provider URL: %s", provider.Spec.URL),
			})
			return nil
		}

		r.Log.Info("SSH validation: extracted IP from provider URL",
			"provider", provider.Name,
			"hostIP", hostIP)

		// Single host for direct ESXi
		return []hostInfo{{name: "ESXi", ip: hostIP}}
	}

	// For vCenter connections, use inventory to get ESXi hosts
	r.Log.Info("SSH validation: vCenter connection, attempting to get collector", "provider", provider.Name, "namespace", provider.Namespace)
	collector, found := r.container.Get(provider)
	if !found {
		r.Log.Error(nil, "SSH validation: collector not found for provider - SSH readiness check failed", "provider", provider.Name, "namespace", provider.Namespace)
		provider.Status.SetCondition(libcnd.Condition{
			Type:     SSHReadiness,
			Status:   False,
			Reason:   "InventoryCollectorNotFound",
			Category: Warn,
			Message:  "Cannot validate SSH readiness: inventory collector not found. Ensure the provider is properly configured and inventory collection has started.",
		})
		return nil
	}

	r.Log.Info("SSH validation: collector found, checking parity", "provider", provider.Name, "hasParity", collector.HasParity())
	if !collector.HasParity() {
		r.Log.Error(nil, "SSH validation: collector does not have parity yet - SSH readiness check failed", "provider", provider.Name)
		provider.Status.SetCondition(libcnd.Condition{
			Type:     SSHReadiness,
			Status:   False,
			Reason:   "InventoryNotReady",
			Category: Warn,
			Message:  "Cannot validate SSH readiness: inventory collection has not completed (parity not reached). Wait for inventory to finish loading.",
		})
		return nil
	}

	r.Log.Info("SSH validation: listing hosts from database", "provider", provider.Name)

	// Get hosts from inventory
	db := collector.DB()
	var hosts []vsphere.Host
	listOptions := model.ListOptions{Detail: model.MaxDetail}
	r.Log.Info("SSH validation: listing hosts with options",
		"provider", provider.Name,
		"listOptions", listOptions)
	err := db.List(&hosts, listOptions)
	if err != nil {
		r.Log.Error(err, "SSH validation: failed to list hosts from inventory - SSH readiness check failed", "provider", provider.Name)
		provider.Status.SetCondition(libcnd.Condition{
			Type:     SSHReadiness,
			Status:   False,
			Reason:   "HostListError",
			Category: Warn,
			Message:  fmt.Sprintf("Cannot validate SSH readiness: failed to list hosts from inventory: %v", err),
		})
		return nil
	}

	r.Log.Info("SSH validation: hosts retrieved from inventory", "provider", provider.Name, "hostCount", len(hosts))

	if len(hosts) == 0 {
		r.Log.Error(nil, "SSH validation: no hosts in inventory - SSH readiness check failed", "provider", provider.Name)
		provider.Status.SetCondition(libcnd.Condition{
			Type:     SSHReadiness,
			Status:   False,
			Reason:   "NoHostsFound",
			Category: Warn,
			Message:  "Cannot validate SSH readiness: no ESXi hosts found in inventory. Ensure hosts are properly added to the vSphere provider.",
		})
		return nil
	}

	// Helper function to get host IP from ManagementIPs
	getHostIP := func(host *vsphere.Host) string {
		if len(host.ManagementIPs) > 0 {
			r.Log.V(3).Info("SSH validation: using ManagementIP for host",
				"provider", provider.Name,
				"hostName", host.Name,
				"managementIPs", host.ManagementIPs,
				"selectedIP", host.ManagementIPs[0])
			return host.ManagementIPs[0]
		}
		r.Log.V(3).Info("SSH validation: no ManagementIPs for host",
			"provider", provider.Name,
			"hostName", host.Name)
		return ""
	}

	// Log detailed host information for debugging
	r.Log.V(3).Info("SSH validation: analyzing host inventory", "provider", provider.Name)
	var hostsToTest []hostInfo
	hostsWithIP := 0
	hostsWithoutIP := 0
	for i := range hosts {
		hostIP := getHostIP(&hosts[i])
		r.Log.V(3).Info("SSH validation: host details",
			"provider", provider.Name,
			"hostIndex", i,
			"hostName", hosts[i].Name,
			"hostID", hosts[i].ID,
			"managementIPsCount", len(hosts[i].ManagementIPs),
			"managementIPs", hosts[i].ManagementIPs,
			"resolvedIP", hostIP,
			"productVersion", hosts[i].ProductVersion,
			"status", hosts[i].Status)
		hostsWithIP++
		hostsToTest = append(hostsToTest, hostInfo{name: hosts[i].Name, ip: hostIP})
	}

	r.Log.V(2).Info("SSH validation: host IP summary",
		"provider", provider.Name,
		"totalHosts", len(hosts),
		"hostsWithIP", hostsWithIP,
		"hostsWithoutIP", hostsWithoutIP)

	if hostsWithIP == 0 {
		r.Log.Error(nil, "SSH validation: no hosts with IP found - SSH readiness check failed",
			"provider", provider.Name,
			"totalHosts", len(hosts),
			"hostsWithIP", hostsWithIP,
			"hostsWithoutIP", hostsWithoutIP)
		provider.Status.SetCondition(libcnd.Condition{
			Type:     SSHReadiness,
			Status:   False,
			Reason:   "NoHostIP",
			Category: Warn,
			Message:  fmt.Sprintf("Cannot validate SSH readiness: no ESXi hosts with management IP address found in inventory (found %d hosts total, but none have management IP addresses). Check that VirtualNicManager configuration is being collected.", len(hosts)),
		})
		return nil
	}

	return hostsToTest
}

// validateSSHReadiness validates SSH readiness for vSphere providers when SSH method is enabled
func (r *Reconciler) validateSSHReadiness(provider *api.Provider, secret *core.Secret) error {
	// Only validate SSH for vSphere providers
	if provider.Type() != api.VSphere {
		r.Log.V(3).Info("SSH validation: skipping non-vSphere provider",
			"provider", provider.Name,
			"providerType", provider.Type())
		return nil
	}

	// Check if ESXiCloneMethod is set to "ssh"
	esxiCloneMethod, methodSet := provider.Spec.Settings[api.ESXiCloneMethod]
	if !methodSet || esxiCloneMethod != "ssh" {
		r.Log.V(3).Info("SSH validation: SSH method not enabled, skipping",
			"provider", provider.Name,
			"esxiCloneMethod", esxiCloneMethod,
			"methodSet", methodSet)
		// SSH method not enabled, remove any existing SSH readiness conditions
		provider.Status.DeleteCondition(SSHReadiness)
		return nil
	}

	r.Log.Info("SSH validation: starting validation for vSphere provider with SSH method enabled",
		"provider", provider.Name,
		"namespace", provider.Namespace,
		"providerType", provider.Type())

	// Check if SSH keys exist (they should be created by ensureSSHKeys)
	privateSecretName, err := util.GenerateSSHPrivateSecretName(provider.Name)
	if err != nil {
		return liberr.Wrap(err)
	}
	publicSecretName, err := util.GenerateSSHPublicSecretName(provider.Name)
	if err != nil {
		return liberr.Wrap(err)
	}

	// Try to get the SSH key secrets
	privateSecret := &core.Secret{}
	err = r.Get(context.TODO(), client.ObjectKey{
		Namespace: provider.Namespace,
		Name:      privateSecretName,
	}, privateSecret)

	publicSecret := &core.Secret{}
	err2 := r.Get(context.TODO(), client.ObjectKey{
		Namespace: provider.Namespace,
		Name:      publicSecretName,
	}, publicSecret)

	if err != nil || err2 != nil {
		// SSH keys don't exist yet - this is expected on first reconcile
		// They will be created by ensureSSHKeys after validation
		provider.Status.SetCondition(libcnd.Condition{
			Type:     SSHReadiness,
			Status:   False,
			Reason:   "SSHKeysNotFound",
			Category: Warn,
			Message: fmt.Sprintf(
				"SSH method is enabled but SSH keys are being generated. "+
					"After keys are created, you must manually install the public key on each ESXi host. "+
					"Expected secrets: %s, %s",
				privateSecretName, publicSecretName),
		})
		return nil
	}

	// Get public key content
	publicKeyBytes, ok := publicSecret.Data["public-key"]
	if !ok {
		provider.Status.SetCondition(libcnd.Condition{
			Type:     SSHReadiness,
			Status:   False,
			Reason:   "SSHPublicKeyInvalid",
			Category: Warn,
			Message:  fmt.Sprintf("SSH public key secret '%s' does not contain 'public-key' data", publicSecretName),
		})
		return nil
	}
	publicKey := string(publicKeyBytes)

	publicKeyPreview := publicKey
	if len(publicKey) > 60 {
		publicKeyPreview = publicKey[:60] + "..."
	}
	r.Log.V(3).Info("SSH validation: loaded public key from secret",
		"provider", provider.Name,
		"publicKeyLength", len(publicKey),
		"publicKeyPreview", publicKeyPreview)

	// Get private key for testing
	privateKeyBytes, ok := privateSecret.Data["private-key"]
	if !ok {
		provider.Status.SetCondition(libcnd.Condition{
			Type:     SSHReadiness,
			Status:   False,
			Reason:   "SSHPrivateKeyInvalid",
			Category: Warn,
			Message:  fmt.Sprintf("SSH private key secret '%s' does not contain 'private-key' data", privateSecretName),
		})
		return nil
	}

	privateKeyPreview := string(privateKeyBytes)
	if len(privateKeyBytes) > 60 {
		privateKeyPreview = string(privateKeyBytes[:60]) + "..."
	}
	r.Log.V(3).Info("SSH validation: loaded private key from secret",
		"provider", provider.Name,
		"privateKeyLength", len(privateKeyBytes),
		"privateKeyPreview", privateKeyPreview)

	// Get list of hosts to test based on provider connection type
	hostsToTest := r.getHostsForSSHValidation(provider)
	if hostsToTest == nil {
		// Error condition already set by getHostsForSSHValidation
		return nil
	}

	r.Log.Info("SSH validation: hosts to test",
		"provider", provider.Name,
		"hostCount", len(hostsToTest))

	// Build the restricted key format with dynamic datastore routing
	restrictedKey := fmt.Sprintf(`command="%s",no-port-forwarding,no-agent-forwarding,no-X11-forwarding %s`, util.RestrictedSSHCommandTemplate, publicKey)

	// Log the keys being used
	r.Log.V(3).Info("SSH validation: key details for testing",
		"provider", provider.Name,
		"publicKeyLength", len(publicKey),
		"publicKeyPrefix", func() string {
			if len(publicKey) > 40 {
				return publicKey[:40] + "..."
			}
			return publicKey
		}(),
		"restrictedKeyLength", len(restrictedKey),
		"privateKeyLength", len(privateKeyBytes))

	// Test SSH connectivity on ALL hosts (don't stop early)
	r.Log.Info("SSH validation: testing all hosts for complete status",
		"provider", provider.Name,
		"totalHosts", len(hostsToTest))
	failedHosts := []string{}
	successHosts := []string{}

	// Test all hosts to provide complete status
	for i := range hostsToTest {
		host := &hostsToTest[i]
		if host.ip == "" {
			r.Log.Info("SSH validation: host has no management IP - marking as failed",
				"provider", provider.Name,
				"hostName", host.name)
			failedHosts = append(failedHosts, fmt.Sprintf("%s (no management IP)", host.name))
			continue
		}
		r.Log.V(3).Info("SSH validation: testing host",
			"provider", provider.Name,
			"hostName", host.name,
			"hostIP", host.ip,
			"hostIndex", i+1,
			"totalHosts", len(hostsToTest))
		hostResult := r.testSSHConnectivity(host.ip, privateKeyBytes)
		r.Log.V(3).Info("SSH validation: host test result",
			"provider", provider.Name,
			"hostName", host.name,
			"hostIP", host.ip,
			"success", hostResult)

		if hostResult {
			successHosts = append(successHosts, fmt.Sprintf("%s (%s)", host.name, host.ip))
		} else {
			failedHosts = append(failedHosts, fmt.Sprintf("%s (%s)", host.name, host.ip))
		}
	}

	r.Log.Info("SSH validation: all hosts tested",
		"provider", provider.Name,
		"successCount", len(successHosts),
		"failedCount", len(failedHosts))

	// If all hosts passed, we're done
	if len(failedHosts) == 0 {
		r.Log.Info("SSH validation: all hosts have SSH working",
			"provider", provider.Name,
			"hostCount", len(hostsToTest))
		provider.Status.DeleteCondition(SSHReadiness)
		return nil
	}

	// Build detailed message with both success and failure lists
	var message strings.Builder
	if len(successHosts) == 0 {
		message.WriteString(fmt.Sprintf("SSH connectivity failed for all %d ESXi host(s). Manual SSH key installation required.\n\n", len(hostsToTest)))
	} else {
		message.WriteString(fmt.Sprintf("SSH connectivity: %d succeeded, %d failed (of %d total host(s)).\n\n",
			len(successHosts), len(failedHosts), len(hostsToTest)))
	}

	if len(successHosts) > 0 {
		message.WriteString(fmt.Sprintf("✓ SSH WORKING (%d hosts):\n", len(successHosts)))
		for _, host := range successHosts {
			message.WriteString(fmt.Sprintf("  • %s\n", host))
		}
		message.WriteString("\n")
	}

	if len(failedHosts) > 0 {
		message.WriteString(fmt.Sprintf("✗ SSH SETUP NEEDED (%d hosts):\n", len(failedHosts)))
		for _, host := range failedHosts {
			message.WriteString(fmt.Sprintf("  • %s\n", host))
		}
		message.WriteString("\n")
	}

	message.WriteString("SETUP INSTRUCTIONS:\n\n")
	message.WriteString("1. Enable SSH on each ESXi host:\n")
	message.WriteString("   vim-cmd hostsvc/enable_ssh\n")
	message.WriteString("   vim-cmd hostsvc/start_ssh\n\n")
	message.WriteString("2. Add the following line to /etc/ssh/keys-root/authorized_keys on each ESXi host:\n\n")
	message.WriteString(restrictedKey + "\n\n")

	reason := "SSHConnectivityFailed"
	if len(successHosts) > 0 {
		reason = "SSHPartiallyConfigured"
	}

	provider.Status.SetCondition(libcnd.Condition{
		Type:     SSHReadiness,
		Status:   False,
		Reason:   reason,
		Category: Warn,
		Message:  message.String(),
		Items:    failedHosts,
	})
	return nil
}

// testSSHConnectivity tests SSH connectivity to an ESXi host
// This matches the exact implementation from the populator's testSSHConnectivity
func (r *Reconciler) testSSHConnectivity(hostIP string, privateKey []byte) bool {
	return util.TestSSHConnectivity(context.Background(), hostIP, privateKey, r.Log)
}
