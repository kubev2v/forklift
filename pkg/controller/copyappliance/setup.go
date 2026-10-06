package copyappliance

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/nbd-container/announce"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// setupConnectTimeout bounds how long the setup pod waits for sshd. The guest
// reported an address before the pod was created, so sshd is seconds away.
const setupConnectTimeout = 5 * time.Minute

// setupConnectInterval is how often the login is retried while waiting.
const setupConnectInterval = 5 * time.Second

// setupStartTimeout bounds how long the setup pod waits for systemd to report
// the supervisor running after installing it.
const setupStartTimeout = 2 * time.Minute

// setupStartInterval is how often systemd is asked while waiting.
const setupStartInterval = 3 * time.Second

// SetupRequest is everything the setup pod is told about the appliance it is
// setting up. The controller writes these onto the pod as environment.
type SetupRequest struct {
	// Appliance is the name of the CopyAppliance, for messages.
	Appliance string
	// Address the appliance VM is reached on.
	Address string
	// PullSpec is the digest-pinned reference the exporter image is read from.
	PullSpec string
	// Tag is the reference the image is filed under in the appliance's podman
	// store, and the one the supervisor is rendered to run.
	Tag string
	// KeyDir is where the appliance secret is mounted.
	KeyDir string
	// NbdSsl requires mutual TLS on nbdkit exports.
	NbdSsl bool
}

// RunSetup loads the appliance's container image into its podman store and
// installs the supervisor that runs it, over one SSH login. Every step is
// idempotent, so a pod that replaces a failed one re-enters whatever state the
// appliance was left in.
func RunSetup(ctx context.Context, req SetupRequest) (err error) {
	secret, err := readKeyDir(req.KeyDir)
	if err != nil {
		return
	}
	// Enough of an appliance context for the login and the supervisor: neither
	// touches vCenter, and Close guards the connection it does not have.
	ac := &ApplianceContext{
		Appliance: &api.CopyAppliance{
			ObjectMeta: meta.ObjectMeta{Name: req.Appliance},
			Status: api.CopyApplianceStatus{
				Addresses:     []api.ApplianceAddress{{IP: req.Address}},
				ExporterImage: req.Tag,
			},
		},
		ApplianceSecret: secret,
		NbdSsl:          req.NbdSsl,
		Log:             log,
	}

	img, err := registryImage(ctx, req.PullSpec)
	if err != nil {
		return
	}
	tag, err := makeTag(img)
	if err != nil {
		return
	}
	// The supervisor is rendered to run req.Tag, so an image that files itself
	// under anything else would leave a unit pointing at nothing in the store.
	if tag.Name() != req.Tag {
		err = liberr.New(
			"the image is not the one the controller resolved",
			"expected", req.Tag,
			"found", tag.Name())
		return
	}

	client, err := connectAppliance(ctx, ac)
	if err != nil {
		return
	}
	defer func() {
		_ = client.Close()
	}()

	log.Info("Streaming the exporter image.", "address", req.Address, "as", req.Tag)
	err = streamImage(client, img, tag)
	if err != nil {
		return
	}
	log.Info("Done streaming the exporter image.", "image", req.Tag)

	err = installSupervisor(&Orchestrator{context: ac, ssh: client})
	if err != nil {
		return
	}
	log.Info("Set the appliance up.", "address", req.Address)
	return
}

// readKeyDir reads the mounted appliance secret back into the shape the
// appliance context reads it from.
func readKeyDir(dir string) (secret *core.Secret, err error) {
	if dir == "" {
		err = liberr.New("no directory is configured for the appliance secret")
		return
	}
	secret = &core.Secret{
		ObjectMeta: meta.ObjectMeta{Name: dir},
		Data:       map[string][]byte{},
	}
	for _, key := range []string{
		sshPrivateKeyData,
		announce.CACert,
		announce.ServerCert,
		announce.ServerKey,
	} {
		path := filepath.Join(dir, key)
		value, rErr := os.ReadFile(path) // #nosec G304 -- the path is the mount the controller set
		if rErr != nil {
			secret = nil
			err = liberr.Wrap(rErr, "file", path)
			return
		}
		secret.Data[key] = value
	}
	return
}

// connectAppliance logs in, retrying while the appliance is not answering yet.
// sshd accepts connections some seconds after the guest reports the address the
// pod was given.
func connectAppliance(ctx context.Context, ac *ApplianceContext) (client *SSHClient, err error) {
	deadline := time.Now().Add(setupConnectTimeout)
	for {
		var ready bool
		client, ready, err = ac.SSHClient(ctx, SSHFileTransferTimeout)
		if err != nil || ready {
			return
		}
		if time.Now().After(deadline) {
			err = liberr.New(
				"the appliance is not answering on SSH",
				"appliance", ac.Appliance.Name,
				"waited", setupConnectTimeout.String())
			return
		}
		select {
		case <-ctx.Done():
			err = liberr.Wrap(ctx.Err())
			return
		case <-time.After(setupConnectInterval):
		}
	}
}

// installSupervisor puts the supervisor on the appliance and waits for systemd
// to report it running.
func installSupervisor(orch *Orchestrator) (err error) {
	installed, err := orch.Installed()
	if err != nil {
		return
	}
	if !installed {
		// Install ends by restarting the unit.
		err = orch.Install()
		if err != nil {
			return
		}
		return waitActive(orch)
	}
	active, err := orch.Active()
	if err != nil {
		return
	}
	if !active {
		err = orch.Start()
		if err != nil {
			return
		}
	}
	return waitActive(orch)
}

// waitActive polls systemd until it reports the supervisor running. The journal
// tail goes to stderr on one that does not come up: with
// TerminationMessageFallbackToLogsOnError that is what carries the reason onto
// the CopyAppliance.
func waitActive(orch *Orchestrator) (err error) {
	deadline := time.Now().Add(setupStartTimeout)
	for {
		active, aErr := orch.Active()
		if aErr != nil {
			err = aErr
			return
		}
		if active {
			return
		}
		if time.Now().After(deadline) {
			fmt.Fprintln(os.Stderr, orch.Log())
			err = liberr.New(
				"the appliance supervisor did not start",
				"unit", orchestratorUnit,
				"waited", setupStartTimeout.String())
			return
		}
		time.Sleep(setupStartInterval)
	}
}
