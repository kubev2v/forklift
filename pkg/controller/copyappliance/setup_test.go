package copyappliance

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kubev2v/forklift/pkg/nbd-container/announce"
)

// testKeyDir is the appliance secret as the pod finds it mounted.
func testKeyDir(t *testing.T, keys ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, key := range keys {
		path := filepath.Join(dir, key)
		if err := os.WriteFile(path, []byte(key+" contents"), 0o600); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	return dir
}

// everyKey is what the controller projects into the setup pod.
func everyKey() []string {
	return []string{sshPrivateKeyData, announce.CACert, announce.ServerCert, announce.ServerKey}
}

func TestReadKeyDir(t *testing.T) {
	t.Run("the mounted secret reads back whole", func(t *testing.T) {
		dir := testKeyDir(t, everyKey()...)

		secret, err := readKeyDir(dir)
		if err != nil {
			t.Fatalf("readKeyDir: %v", err)
		}

		for _, key := range everyKey() {
			if got := string(secret.Data[key]); got != key+" contents" {
				t.Errorf("%s = %q, want what was on disk", key, got)
			}
		}
	})

	// A key the pod was not given is a secret the operator built by hand or a
	// projection that changed, and the login or the TLS install fails on it
	// later with nothing naming the file.
	t.Run("a key that is not mounted fails naming the file", func(t *testing.T) {
		dir := testKeyDir(t, sshPrivateKeyData, announce.CACert, announce.ServerCert)

		_, err := readKeyDir(dir)

		if err == nil {
			t.Fatal("readKeyDir succeeded with a key missing")
		}
		if !errorMentions(t, err, filepath.Join(dir, announce.ServerKey)) {
			t.Errorf("error = %q, want it to name the missing file", err)
		}
	})

	t.Run("no directory at all fails", func(t *testing.T) {
		_, err := readKeyDir("")
		if err == nil {
			t.Fatal("readKeyDir succeeded with no directory")
		}
	})
}

func TestRegistryImage(t *testing.T) {
	t.Run("an image reached without a registry CA is the image that comes back", func(t *testing.T) {
		spec, want := pushImage(t, testRegistry(t, nil))

		img, err := registryImage(context.TODO(), spec)
		if err != nil {
			t.Fatalf("registryImage: %v", err)
		}

		if digestOf(t, img) != digestOf(t, want) {
			t.Errorf("digest = %s, want %s", digestOf(t, img), digestOf(t, want))
		}
	})

	// The pod exits on this, and the message is all that reaches the
	// CopyAppliance, so it has to name what could not be read.
	t.Run("an image that is not there fails naming it", func(t *testing.T) {
		spec := testRegistry(t, nil) + "/forklift/copy-appliance:never-pushed"

		_, err := registryImage(context.TODO(), spec)

		if err == nil {
			t.Fatal("registryImage succeeded for an image that was never pushed")
		}
		if !errorMentions(t, err, spec) {
			t.Errorf("error = %q, want it to name %q", err, spec)
		}
	})
}

// The controller records the tag in Status.ExporterImage and the unit the pod
// installs is rendered to run it. An image filed under anything else leaves a
// supervisor pointing at nothing in the appliance's store, which would surface
// as exports that never appear rather than as a pod that failed.
func TestRunSetupRefusesAnotherImage(t *testing.T) {
	spec, _ := pushImage(t, testRegistry(t, nil))

	err := RunSetup(context.TODO(), SetupRequest{
		Appliance: "appliance",
		Address:   "192.0.2.10",
		PullSpec:  spec,
		Tag:       "copy-appliance:some-other-digest",
		KeyDir:    testKeyDir(t, everyKey()...),
	})

	if err == nil {
		t.Fatal("RunSetup succeeded with an image the controller did not resolve")
	}
	if !errorMentions(t, err, "copy-appliance:some-other-digest") {
		t.Errorf("error = %q, want it to name the tag it expected", err)
	}
}

// The secret is read before anything else, so a projection the pod did not get
// fails it immediately rather than part way through the install.
func TestRunSetupRequiresTheSecret(t *testing.T) {
	err := RunSetup(context.TODO(), SetupRequest{Appliance: "appliance", KeyDir: t.TempDir()})

	if err == nil {
		t.Fatal("RunSetup succeeded with nothing mounted")
	}
	if !errorMentions(t, err, sshPrivateKeyData) {
		t.Errorf("error = %q, want it to name the key that is missing", err)
	}
}
