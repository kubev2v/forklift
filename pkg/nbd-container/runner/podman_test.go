package runner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kubev2v/forklift/pkg/nbd-container/blockdev"
)

// testDevice is one disk attached to the appliance, and testContainer the
// container name the runner derives from its WWID.
var testDevice = blockdev.Device{WWID: "wwn-abc", Path: "/dev/sdb", Size: 1 << 30}

const testContainer = "nbd-wwn-abc"

func TestExistingPort(t *testing.T) {
	t.Run("a running container is reused", func(t *testing.T) {
		podman := stubPodman(t)
		podman.running(testContainer)
		podman.port("10809")

		port, ok, err := New(testConfig()).existingPort(context.TODO(), testDevice.WWID)

		if err != nil {
			t.Fatalf("existingPort: %v", err)
		}
		if !ok || port != 10809 {
			t.Errorf("existingPort = (%d, %v), want the running container's port", port, ok)
		}
	})

	t.Run("a running container with restarts is not reused", func(t *testing.T) {
		podman := stubPodman(t)
		podman.running(testContainer)
		podman.port("10809")
		t.Setenv("PODMAN_STATE", "running 1")

		port, ok, err := New(testConfig()).existingPort(context.TODO(), testDevice.WWID)

		if err != nil {
			t.Fatalf("existingPort: %v", err)
		}
		if ok {
			t.Errorf("existingPort = (%d, true), want a crash-looping export not reused", port)
		}
	})

	t.Run("a stopped container is not reused", func(t *testing.T) {
		podman := stubPodman(t)
		podman.all(testContainer)

		port, ok, err := New(testConfig()).existingPort(context.TODO(), testDevice.WWID)

		if err != nil {
			t.Fatalf("existingPort: %v", err)
		}
		if ok {
			t.Errorf("existingPort = (%d, true), want a stopped container not reused", port)
		}
	})
}

// After an unclean shutdown, exited containers still hold the name; Reconcile
// must remove them and start a replacement.
func TestReconcileReplacesAStaleContainer(t *testing.T) {
	podman := stubPodman(t)
	podman.all(testContainer)

	exports, err := New(testConfig()).Reconcile(context.TODO(), []blockdev.Device{testDevice})

	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	want := Export{WWID: testDevice.WWID, Port: 10809, Device: testDevice.Path, Size: testDevice.Size}
	if len(exports) != 1 || exports[0] != want {
		t.Errorf("exports = %+v, want %+v", exports, []Export{want})
	}
	if _, ran := podman.ran("rm -f " + testContainer); !ran {
		t.Error("the stale container was not removed")
	}
	if _, ran := podman.contains("run -d"); !ran {
		t.Errorf("no replacement container was created; ran %v", podman.commands())
	}
}

// --- fixtures ---

func testConfig() Config {
	return Config{
		Image:     "localhost/nbd-container",
		BasePort:  10809,
		PublishIP: "0.0.0.0",
	}
}

// podmanStub is a podman put on PATH ahead of any real one. The runner drives
// podman entirely through its command line, so standing in for the command is
// what lets the reconcile logic be tested at all; there is nothing to inject.
type podmanStub struct {
	t   *testing.T
	log string
}

// podmanScript answers the handful of queries the runner makes. A container
// named in PODMAN_RUNNING inspects as having a published port; anything else
// inspects as having none, which is what a real stopped container reports and
// is the whole difference this stub exists to model.
const podmanScript = `#!/bin/sh
printf '%s\n' "$*" >> "$PODMAN_LOG"
case "$1" in
ps)
	case "$*" in
	*status=running*) printf '%s' "$PODMAN_RUNNING" ;;
	*) printf '%s' "$PODMAN_ALL" ;;
	esac
	;;
inspect)
	if [ "$2" = "--format" ]; then
		printf '%s\n' "${PODMAN_STATE:-running 0}"
		exit 0
	fi
	case " $PODMAN_RUNNING " in
	*" $2 "*) printf '[{"NetworkSettings":{"Ports":{"10809/tcp":[{"HostPort":"%s"}]}}}]' "$PODMAN_PORT" ;;
	*) printf '[{"NetworkSettings":{"Ports":{}}}]' ;;
	esac
	;;
esac
exit 0
`

func stubPodman(t *testing.T) *podmanStub {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "podman"), []byte(podmanScript), 0o755); err != nil {
		t.Fatalf("writing the podman stub: %v", err)
	}
	stub := &podmanStub{t: t, log: filepath.Join(dir, "commands")}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("PODMAN_LOG", stub.log)
	t.Setenv("PODMAN_RUNNING", "")
	t.Setenv("PODMAN_ALL", "")
	t.Setenv("PODMAN_PORT", "10809")
	t.Setenv("PODMAN_STATE", "running 0")
	return stub
}

// running names the containers podman reports as up. They are listed among all
// containers too, because that is where a real `podman ps -a` lists them.
func (r *podmanStub) running(names ...string) {
	r.t.Setenv("PODMAN_RUNNING", strings.Join(names, "\n"))
	r.t.Setenv("PODMAN_ALL", strings.Join(names, "\n"))
}

// all names the containers podman reports in any state.
func (r *podmanStub) all(names ...string) {
	r.t.Setenv("PODMAN_ALL", strings.Join(names, "\n"))
}

func (r *podmanStub) port(port string) {
	r.t.Setenv("PODMAN_PORT", port)
}

// commands is every podman invocation, in order, as one line of arguments each.
func (r *podmanStub) commands() []string {
	r.t.Helper()
	log, err := os.ReadFile(r.log)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		r.t.Fatalf("reading the podman log: %v", err)
	}
	return strings.Split(strings.TrimRight(string(log), "\n"), "\n")
}

// ran reports whether podman was invoked with exactly these arguments.
func (r *podmanStub) ran(command string) (string, bool) {
	r.t.Helper()
	for _, ran := range r.commands() {
		if ran == command {
			return ran, true
		}
	}
	return "", false
}

// contains reports whether podman was invoked with a command line carrying this
// fragment, for the arguments that are too long to write out.
func (r *podmanStub) contains(fragment string) (string, bool) {
	r.t.Helper()
	for _, ran := range r.commands() {
		if strings.Contains(ran, fragment) {
			return ran, true
		}
	}
	return "", false
}

func TestSanitize(t *testing.T) {
	tests := map[string]string{
		"eui.0025388411b1e8d8":                    "eui-0025388411b1e8d8",
		"0x5000c500a1b2c3d4":                      "0x5000c500a1b2c3d4",
		"SAMSUNG_MZVLB1T0HBLR-000L7_S4EMNX0R4321": "SAMSUNG-MZVLB1T0HBLR-000L7-S4EMNX0R4321",
		"/dev/sdb": "dev-sdb",
	}
	for in, want := range tests {
		if got := sanitize(in); got != want {
			t.Errorf("sanitize(%q) = %q, want %q", in, got, want)
		}
	}
}
