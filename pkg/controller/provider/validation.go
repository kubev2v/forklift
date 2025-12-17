package provider

import (
	"context"
	"crypto/x509"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/base"
	"github.com/kubev2v/forklift/pkg/controller/provider/container"
	vsphere "github.com/kubev2v/forklift/pkg/controller/provider/model/vsphere"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/lib/inventory/model"
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
	SSHReady                = "SSHReady"
	SSHNotReady             = "SSHNotReady"
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

func (r *Reconciler) validateConnectionStatus(provider *api.Provider, secret *core.Secret) bool {
	if base.GetInsecureSkipVerifyFlag(secret) {
		provider.Status.SetCondition(libcnd.Condition{
			Type:     ConnectionInsecure,
			Status:   True,
			Reason:   SkipTLSVerification,
			Category: Warn,
			Message:  "TLS is susceptible to machine-in-the-middle attacks when certificate verification is skipped.",
		})
		return true
	}

	// Check if cacert is provided when TLS verification is enabled
	if len(secret.Data["cacert"]) == 0 {
		// Return false to signal caller to skip further validation (GetTlsCertificate)
		// The keyList validation will handle the error
		return false
	}

	// Verify TLS connection with provided cacert
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
	return true
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

		// validateConnectionStatus returns false if cacert is missing
		if !r.validateConnectionStatus(provider, secret) {
			keyList = append(keyList, "cacert")
			break
		}

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
	id   string
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
				Type:     SSHNotReady,
				Status:   True,
				Reason:   "InvalidProviderURL",
				Category: Warn,
				Message:  fmt.Sprintf("Cannot validate SSH readiness (checked because 'esxiCloneMethod' setting is set to 'ssh'): failed to parse provider URL: %v", err),
			})
			return nil
		}

		// Extract hostname/IP from URL (could be hostname:port or just hostname)
		hostIP := providerURL.Hostname()
		if hostIP == "" {
			provider.Status.SetCondition(libcnd.Condition{
				Type:     SSHNotReady,
				Status:   True,
				Reason:   "NoHostIPInURL",
				Category: Warn,
				Message:  fmt.Sprintf("Cannot validate SSH readiness (checked because 'esxiCloneMethod' setting is set to 'ssh'): no host/IP found in provider URL: %s", provider.Spec.URL),
			})
			return nil
		}

		r.Log.Info("SSH validation: extracted IP from provider URL",
			"provider", provider.Name,
			"hostIP", hostIP)

		// Single host for direct ESXi
		return []hostInfo{{id: hostIP, name: "ESXi", ip: hostIP}}
	}

	// For vCenter connections, get ESXi hosts from inventory
	r.Log.Info("SSH validation: vCenter connection, getting hosts from inventory", "provider", provider.Name, "namespace", provider.Namespace)

	collector, found := r.container.Get(provider)
	if !found {
		r.Log.Error(nil, "SSH validation: collector not found", "provider", provider.Name)
		return nil
	}

	// Get hosts from inventory
	db := collector.DB()
	var inventoryHosts []vsphere.Host
	listOptions := model.ListOptions{Detail: model.MaxDetail}
	err := db.List(&inventoryHosts, listOptions)
	if err != nil {
		r.Log.Error(err, "SSH validation: failed to list hosts from inventory", "provider", provider.Name)
		provider.Status.SetCondition(libcnd.Condition{
			Type:     SSHNotReady,
			Status:   True,
			Reason:   "HostListError",
			Category: Warn,
			Message:  fmt.Sprintf("Cannot validate SSH readiness (checked because 'esxiCloneMethod' setting is set to 'ssh'): failed to list hosts from inventory: %v", err),
		})
		return nil
	}

	r.Log.Info("SSH validation: found hosts in inventory",
		"provider", provider.Name,
		"hostCount", len(inventoryHosts))

	if len(inventoryHosts) == 0 {
		r.Log.Info("SSH validation: no hosts in inventory", "provider", provider.Name)
		provider.Status.SetCondition(libcnd.Condition{
			Type:     SSHNotReady,
			Status:   True,
			Reason:   "NoHostsFound",
			Category: Warn,
			Message:  "Cannot validate SSH readiness (checked because 'esxiCloneMethod' setting is set to 'ssh'): no ESXi hosts found in inventory.",
		})
		return nil
	}

	// Load Host CRDs to check for migration network IPs (optional override)
	hostCRDIPs := r.loadHostIPs(provider)
	r.Log.Info("SSH validation: loaded Host resources for migration network",
		"provider", provider.Name,
		"hostCRDCount", len(hostCRDIPs))

	// Build list of hosts to test
	var hostsToTest []hostInfo
	hostsWithIP := 0
	hostsWithoutIP := 0

	for i := range inventoryHosts {
		invHost := &inventoryHosts[i]
		var hostIP string

		// First check if there's a Host CRD with IpAddress (migration network)
		if ip, found := hostCRDIPs[invHost.ID]; found && ip != "" {
			hostIP = ip
			r.Log.V(3).Info("SSH validation: using Host IpAddress (migration network)",
				"hostName", invHost.Name,
				"hostID", invHost.ID,
				"ipAddress", hostIP)
		} else if len(invHost.ManagementIPs) > 0 {
			// Fall back to ManagementIPs from inventory
			hostIP = invHost.ManagementIPs[0]
			r.Log.V(3).Info("SSH validation: using ManagementIP from inventory",
				"hostName", invHost.Name,
				"hostID", invHost.ID,
				"ipAddress", hostIP)
		}

		if hostIP != "" {
			hostsWithIP++
			hostsToTest = append(hostsToTest, hostInfo{id: invHost.ID, name: invHost.Name, ip: hostIP})
		} else {
			hostsWithoutIP++
			r.Log.V(3).Info("SSH validation: host has no IP",
				"hostName", invHost.Name,
				"hostID", invHost.ID)
		}
	}

	r.Log.Info("SSH validation: host summary",
		"provider", provider.Name,
		"totalHosts", len(inventoryHosts),
		"hostsWithIP", hostsWithIP,
		"hostsWithoutIP", hostsWithoutIP)

	if hostsWithIP == 0 {
		r.Log.Error(nil, "SSH validation: no hosts with IP found", "provider", provider.Name)
		provider.Status.SetCondition(libcnd.Condition{
			Type:     SSHNotReady,
			Status:   True,
			Reason:   "NoHostIP",
			Category: Warn,
			Message:  fmt.Sprintf("Cannot validate SSH readiness (checked because 'esxiCloneMethod' setting is set to 'ssh'): no ESXi hosts with IP address found (found %d hosts total).", len(inventoryHosts)),
		})
		return nil
	}

	return hostsToTest
}

// loadHostIPs loads Host CRDs for the provider and returns a map of host ID/Name to IpAddress
func (r *Reconciler) loadHostIPs(provider *api.Provider) map[string]string {
	result := make(map[string]string)

	hostList := &api.HostList{}
	err := r.List(
		context.TODO(),
		hostList,
		&client.ListOptions{
			Namespace: provider.Namespace,
		},
	)
	if err != nil {
		r.Log.V(3).Info("SSH validation: failed to list Host resources", "error", err)
		return result
	}

	for i := range hostList.Items {
		host := &hostList.Items[i]

		// Skip hosts that don't belong to this provider
		if !libref.Equals(&host.Spec.Provider, &core.ObjectReference{
			Kind:      "Provider",
			Namespace: provider.Namespace,
			Name:      provider.Name,
		}) {
			continue
		}

		// Skip hosts that are not ready or have no IpAddress
		if !host.Status.HasCondition(libcnd.Ready) || host.Spec.IpAddress == "" {
			continue
		}

		// Map by ID for lookup
		if host.Spec.ID != "" {
			result[host.Spec.ID] = host.Spec.IpAddress
		}
	}

	return result
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

	// Skip SSH validation if inventory is not ready yet
	inventoryCondition := provider.Status.FindCondition(InventoryCreated)
	if inventoryCondition == nil || inventoryCondition.Status != True {
		r.Log.V(3).Info("SSH validation: skipping - inventory not ready yet",
			"provider", provider.Name,
			"hasInventoryCondition", inventoryCondition != nil)
		return nil
	}

	// Check if ESXiCloneMethod is set to "ssh"
	esxiCloneMethod, methodSet := provider.Spec.Settings[api.ESXiCloneMethod]
	if !methodSet || esxiCloneMethod != api.ESXiCloneMethodSSH {
		r.Log.V(3).Info("SSH validation: SSH method not enabled, skipping",
			"provider", provider.Name,
			"esxiCloneMethod", esxiCloneMethod,
			"methodSet", methodSet)
		// SSH method not enabled, remove any existing SSH readiness conditions
		provider.Status.DeleteCondition(SSHReady)
		provider.Status.DeleteCondition(SSHNotReady)
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
			Type:     SSHNotReady,
			Status:   True,
			Reason:   "SSHKeysNotFound",
			Category: Warn,
			Message: fmt.Sprintf(
				"SSH keys are being generated (checked because 'esxiCloneMethod' setting is set to 'ssh'). "+
					"After keys are created, you must manually install the public key on each ESXi host. "+
					"Expected secrets: %s, %s",
				privateSecretName, publicSecretName),
		})

		//nolint:nilerr
		return nil
	}

	// Get public key content
	publicKeyBytes, ok := publicSecret.Data["public-key"]
	if !ok {
		provider.Status.SetCondition(libcnd.Condition{
			Type:     SSHNotReady,
			Status:   True,
			Reason:   "SSHPublicKeyInvalid",
			Category: Warn,
			Message:  fmt.Sprintf("SSH public key secret '%s' does not contain 'public-key' data (checked because 'esxiCloneMethod' setting is set to 'ssh')", publicSecretName),
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
			Type:     SSHNotReady,
			Status:   True,
			Reason:   "SSHPrivateKeyInvalid",
			Category: Warn,
			Message:  fmt.Sprintf("SSH private key secret '%s' does not contain 'private-key' data (checked because 'esxiCloneMethod' setting is set to 'ssh')", privateSecretName),
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

		// Store as "id|name|ip" so Plan can parse and reformat
		itemStr := fmt.Sprintf("%s|%s|%s", host.id, host.name, host.ip)
		if hostResult {
			successHosts = append(successHosts, itemStr)
		} else {
			failedHosts = append(failedHosts, itemStr)
		}
	}

	r.Log.Info("SSH validation: all hosts tested",
		"provider", provider.Name,
		"successCount", len(successHosts),
		"failedCount", len(failedHosts))

	// If all hosts passed, remove all SSH conditions - everything is working fine
	if len(failedHosts) == 0 {
		r.Log.Info("SSH validation: all hosts have SSH working",
			"provider", provider.Name,
			"hostCount", len(hostsToTest))
		provider.Status.DeleteCondition(SSHReady)
		provider.Status.DeleteCondition(SSHNotReady)
		return nil
	}

	// Handle successful hosts - set advisory condition only if there's a mix (some passed, some failed)
	if len(successHosts) > 0 {
		var successSuggestion strings.Builder
		successSuggestion.WriteString("ESXi hosts with SSH connectivity validated:\n\n")
		for _, item := range successHosts {
			// Parse "id|name|ip" format for human-readable display
			parts := strings.Split(item, "|")
			if len(parts) == 3 {
				successSuggestion.WriteString(fmt.Sprintf("  - %s (%s)\n", parts[1], parts[2]))
			} else {
				successSuggestion.WriteString(fmt.Sprintf("  - %s\n", item))
			}
		}
		successSuggestion.WriteString("\nTo use the xcopy volume populator, ensure your VMs are located on these ESXi hosts before starting the migration.\n")

		provider.Status.SetCondition(libcnd.Condition{
			Type:       SSHReady,
			Status:     True,
			Reason:     "SSHConnectivityValidated",
			Category:   Advisory,
			Message:    "SSH connectivity validated (checked because 'esxiCloneMethod' setting is set to 'ssh'). See the suggestion field in the Provider's YAML for the list of available ESXi hosts.",
			Suggestion: successSuggestion.String(),
			Items:      successHosts,
		})
	} else {
		provider.Status.DeleteCondition(SSHReady)
	}

	// Handle failed hosts - set warning condition
	var failSuggestion strings.Builder
	failSuggestion.WriteString("HOSTS REQUIRING SSH SETUP:\n\n")
	for _, item := range failedHosts {
		// Parse "id|name|ip" format for human-readable display
		parts := strings.Split(item, "|")
		if len(parts) == 3 {
			failSuggestion.WriteString(fmt.Sprintf("  - %s (%s)\n", parts[1], parts[2]))
		} else {
			failSuggestion.WriteString(fmt.Sprintf("  - %s\n", item))
		}
	}
	failSuggestion.WriteString("\n")

	failSuggestion.WriteString("SETUP INSTRUCTIONS:\n\n")
	failSuggestion.WriteString("1. Enable SSH on each ESXi host:\n")
	failSuggestion.WriteString("   vim-cmd hostsvc/enable_ssh\n")
	failSuggestion.WriteString("   vim-cmd hostsvc/start_ssh\n\n")
	failSuggestion.WriteString("2. Add the following line to /etc/ssh/keys-root/authorized_keys on each ESXi host:\n\n")
	failSuggestion.WriteString(restrictedKey + "\n\n")

	provider.Status.SetCondition(libcnd.Condition{
		Type:       SSHNotReady,
		Status:     True,
		Reason:     "SSHConnectivityFailed",
		Category:   Warn,
		Message:    "SSH readiness validation issue (checked because 'esxiCloneMethod' setting is set to 'ssh'). See the suggestion field in the Provider's YAML for details.",
		Suggestion: failSuggestion.String(),
		Items:      failedHosts,
	})

	return nil
}

// testSSHConnectivity tests SSH connectivity to an ESXi host
// This matches the exact implementation from the populator's testSSHConnectivity
func (r *Reconciler) testSSHConnectivity(hostIP string, privateKey []byte) bool {
	return util.TestSSHConnectivity(context.Background(), hostIP, privateKey, r.Log)
}
