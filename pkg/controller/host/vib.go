package host

import (
	"context"
	"fmt"
	"time"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	vspherelib "github.com/kubev2v/forklift/pkg/lib/client/vsphere"
	"github.com/kubev2v/forklift/pkg/lib/client/vsphere/vmware"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"golang.org/x/crypto/ssh"
	core "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	DefaultVIBPath  = "/usr/local/share/forklift/vmkfstools-wrapper.vib"
	sshTimeout      = 30 * time.Second
	hostdWait       = 30 * time.Second
	annVIBDatastore = "forklift.konveyor.io/vib-datastore"
)

// ensureVIB handles VIB installation on a single host.
// Step 1: Upload + install VIB on disk via vCenter API (InstallVib).
// Step 2: Restart hostd via SSH using the Host CR's ESXi credentials.
// Step 3: Verify loaded version via vCenter API in the same reconcile loop or after requeue (hostdWait).
func (r *Reconciler) ensureVIB(ctx context.Context, host *api.Host) (requeueAfter time.Duration, err error) {
	if !host.Spec.InstallVIB {
		return 0, nil
	}

	secret := host.Referenced.Secret
	if secret == nil {
		return 0, nil
	}

	provider := host.Referenced.Provider.Source
	if provider == nil {
		return 0, nil
	}

	providerSecret := &core.Secret{}
	err = r.Get(ctx, client.ObjectKey{
		Namespace: provider.Spec.Secret.Namespace,
		Name:      provider.Spec.Secret.Name,
	}, providerSecret)
	if err != nil {
		return 0, liberr.Wrap(err)
	}

	vClient, err := vmware.NewClient(
		provider.Spec.URL,
		string(providerSecret.Data["user"]),
		string(providerSecret.Data["password"]),
	)
	if err != nil {
		r.Log.Error(err, "VIB install: failed to create vCenter client", "host", host.Spec.IpAddress)
		host.Status.SetCondition(libcnd.Condition{
			Type:     VIBInstallFailed,
			Status:   True,
			Reason:   "VCenterConnectionFailed",
			Category: Warn,
			Message:  fmt.Sprintf("VIB installation failed: vCenter connection error: %v", err),
			Durable:  true,
		})
		return 0, nil
	}
	defer func() {
		// Use background context for logout to avoid cancellation issues
		if logoutErr := vClient.Logout(context.Background()); logoutErr != nil {
			r.Log.V(2).Info("vCenter client logout failed", "error", logoutErr)
		}
	}()

	// Check if VIB is already at the desired version (verification after hostd restart).
	esx := vClient.GetHostByRef(ctx, host.Spec.Ref.ID)
	loadedVersion, verr := vspherelib.GetLoadedVIBVersion(ctx, vClient, esx)
	if verr == nil && loadedVersion == vspherelib.VibVersion {
		r.Log.Info("VIB verified at desired version", "host", host.Spec.IpAddress, "version", loadedVersion)
		host.Status.DeleteCondition(VIBInstallFailed)
		host.Status.SetCondition(libcnd.Condition{
			Type:     VIBInstalled,
			Status:   True,
			Reason:   "Installed",
			Category: Required,
			Message:  fmt.Sprintf("VIB %s installed and active.", vspherelib.VibVersion),
			Durable:  true,
		})
		return 0, nil
	}

	// VIB installation is a long operation.
	installCtx, installCancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer installCancel()

	// Step 1: Upload + install via vCenter API.
	// Try the cached datastore name from annotations first; fall back to discovery if missing or
	// if the upload fails (e.g. datastore was remounted), then update the annotation.
	cachedDS := host.Annotations[annVIBDatastore]
	installErr := error(nil)
	if cachedDS != "" {
		installErr = vspherelib.InstallVibToDatastore(installCtx, vClient, esx, DefaultVIBPath, cachedDS, host.Spec.IpAddress)
	}
	if cachedDS == "" || installErr != nil {
		if installErr != nil {
			r.Log.Info("VIB install failed with cached datastore, rediscovering", "host", host.Spec.IpAddress, "datastore", cachedDS, "error", installErr)
		}
		dsName, dsErr := vspherelib.GetHostDatastore(ctx, esx)
		if dsErr != nil {
			r.Log.Error(dsErr, "VIB install: failed to find mounted datastore", "host", host.Spec.IpAddress)
			host.Status.SetCondition(libcnd.Condition{
				Type:     VIBInstallFailed,
				Status:   True,
				Reason:   "InstallFailed",
				Category: Critical,
				Message:  fmt.Sprintf("VIB installation failed on host %s: %v", host.Spec.IpAddress, dsErr),
				Durable:  true,
			})
			return 0, nil
		}
		installErr = vspherelib.InstallVibToDatastore(installCtx, vClient, esx, DefaultVIBPath, dsName, host.Spec.IpAddress)
		if installErr == nil {
			if host.Annotations == nil {
				host.Annotations = map[string]string{}
			}
			host.Annotations[annVIBDatastore] = dsName
			if err = r.Update(ctx, host); err != nil {
				return 0, liberr.Wrap(err)
			}
		}
	}
	if installErr != nil {
		r.Log.Error(installErr, "VIB install failed", "host", host.Spec.IpAddress)
		host.Status.SetCondition(libcnd.Condition{
			Type:     VIBInstallFailed,
			Status:   True,
			Reason:   "InstallFailed",
			Category: Critical,
			Message:  fmt.Sprintf("VIB installation failed on host %s: %v", host.Spec.IpAddress, installErr),
			Durable:  true,
		})
		return 0, nil
	}

	// Step 2: Restart hostd via SSH to load the VIB into memory.
	if err = vspherelib.RestartHostd(installCtx, host.Spec.IpAddress, sshConfigFromSecret(secret)); err != nil {
		r.Log.Error(err, "hostd restart failed", "host", host.Spec.IpAddress)
		host.Status.SetCondition(libcnd.Condition{
			Type:     VIBInstallFailed,
			Status:   True,
			Reason:   "HostdRestartFailed",
			Category: Critical,
			Message:  fmt.Sprintf("VIB installed but hostd restart failed on %s: %v", host.Spec.IpAddress, err),
			Durable:  true,
		})
		return 0, nil
	}

	// Step 3: Requeue to verify loaded version after hostd restart.
	r.Log.Info("VIB installed, requeueing to verify after hostd restart", "host", host.Spec.IpAddress)
	host.Status.DeleteCondition(VIBInstallFailed)
	return hostdWait, nil
}

func sshConfigFromSecret(secret *core.Secret) *ssh.ClientConfig {
	password := string(secret.Data["password"])
	return &ssh.ClientConfig{
		User: string(secret.Data["user"]),
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
			ssh.KeyboardInteractive(func(name, instruction string, questions []string, echos []bool) ([]string, error) {
				answers := make([]string, len(questions))
				for i := range answers {
					answers[i] = password
				}
				return answers, nil
			}),
		},
		// TODO(security): InsecureIgnoreHostKey skips ESXi host key verification, leaving the
		// hostd restart SSH connection open to MITM. Should we store the host fingerprint in
		// the Host CR spec or secret and validate it here? @tech-lead please advise.
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec
		Timeout:         sshTimeout,
	}
}
