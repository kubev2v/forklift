// Package runner starts and reconciles one nbdkit container per block device using
// the podman CLI.
package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/kubev2v/forklift/pkg/nbd-container/blockdev"
)

const (
	// wwidLabel tags each container with the device WWID for idempotent re-runs.
	wwidLabel = "nbd.wwid"
	// deviceLabel records the host device path (informational).
	deviceLabel = "nbd.device"
	// portLabel records the host port nbdkit listens on. Host-network containers
	// publish no port bindings, so reuse reads this label instead.
	portLabel = "nbd.port"
	// containerPort is the fixed port nbdkit listens on inside every container.
	containerPort = 10809
)

// Export describes a running per-device NBD container, as announced to clients.
type Export struct {
	WWID   string `json:"wwid"`
	Port   int    `json:"port"`
	Device string `json:"device"`
	Size   uint64 `json:"size,omitempty"`
}

// Config controls how containers are launched.
type Config struct {
	Image     string // container image, e.g. localhost/nbd-container
	BasePort  int    // first host port to allocate
	PublishIP string // host IP to bind published ports to (e.g. 0.0.0.0)
	CertsDir  string // host TLS certs directory; empty disables NBD TLS
	TLS       bool   // require mutual TLS on nbdkit exports
}

// Runner shells out to podman.
type Runner struct {
	cfg Config
}

func New(cfg Config) *Runner { return &Runner{cfg: cfg} }

// Reconcile ensures a container is running for each device and returns the resulting
// exports. A failure for one device is logged by the caller via the returned error
// slice semantics: individual errors are wrapped and returned aggregated, but every
// device is attempted.
func (r *Runner) Reconcile(ctx context.Context, devices []blockdev.Device) ([]Export, error) {
	used, err := r.usedPorts(ctx)
	if err != nil {
		return nil, err
	}

	var exports []Export
	var errs []error
	nextPort := r.cfg.BasePort
	for _, d := range devices {
		if port, ok, err := r.existingPort(ctx, d.WWID); err != nil {
			errs = append(errs, fmt.Errorf("%s: checking existing container: %w", d.Path, err))
			continue
		} else if ok {
			exports = append(exports, Export{WWID: d.WWID, Port: port, Device: d.Path, Size: d.Size})
			continue
		}

		if err := r.removeContainers(ctx, d.WWID); err != nil {
			errs = append(errs, fmt.Errorf("%s: removing stale containers: %w", d.Path, err))
			continue
		}

		// Allocate the next free host port.
		for used[nextPort] {
			nextPort++
		}
		port := nextPort
		used[port] = true
		nextPort++

		if err := r.run(ctx, d, port); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", d.Path, err))
			continue
		}
		exports = append(exports, Export{WWID: d.WWID, Port: port, Device: d.Path, Size: d.Size})
	}

	if len(errs) > 0 {
		return exports, fmt.Errorf("some devices failed: %w", errors.Join(errs...))
	}
	return exports, nil
}

// Shutdown force-removes every container this tool manages (identified by the wwid
// label), including the crash-looping remnants of any failed start. It is called when
// the supervisor exits so no nbd exports outlive it. Removing (rather than stopping)
// also keeps the reconcile model clean: a later run recreates and re-verifies each
// container instead of reusing a stale one by WWID.
func (r *Runner) Shutdown(ctx context.Context) error {
	out, err := exec.CommandContext(ctx, "podman", "ps", "-aq",
		"--filter", "label="+wwidLabel).Output()
	if err != nil {
		return fmt.Errorf("listing nbd containers: %w", err)
	}
	ids := strings.Fields(string(out))
	if len(ids) == 0 {
		return nil
	}
	args := append([]string{"rm", "-f"}, ids...)
	if out, err := exec.CommandContext(ctx, "podman", args...).CombinedOutput(); err != nil {
		return fmt.Errorf("removing nbd containers: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// run launches a detached nbdkit container for one device and confirms it stays up.
func (r *Runner) run(ctx context.Context, d blockdev.Device, hostPort int) error {
	name := "nbd-" + sanitize(d.WWID)
	args := []string{
		"run", "-d", "--restart=always",
		"--network", "host",
		"--privileged",
		"--cgroups=disabled",
		"--security-opt", "label=disable",
		"--name", name,
		"--label", wwidLabel + "=" + d.WWID,
		"--label", deviceLabel + "=" + d.Path,
		"--label", portLabel + "=" + strconv.Itoa(hostPort),
		"--device", fmt.Sprintf("%s:/dev/nbd-export:r", d.Path),
	}
	if r.cfg.TLS && r.cfg.CertsDir != "" {
		args = append(args, "-v", r.cfg.CertsDir+":/etc/pki/nbd:ro")
	}
	args = append(args,
		"--entrypoint", "nbdkit",
		r.cfg.Image,
		"--foreground", "--readonly",
		"--port", strconv.Itoa(hostPort),
	)
	if r.cfg.TLS {
		args = append(args, "--tls=require", "--tls-certificates=/etc/pki/nbd", "--tls-verify-peer")
	}
	args = append(args, "file", "/dev/nbd-export")
	if out, err := exec.CommandContext(ctx, "podman", args...).CombinedOutput(); err != nil {
		return fmt.Errorf("podman run: %w: %s", err, strings.TrimSpace(string(out)))
	}

	// `podman run -d` succeeds as soon as the container is created, even if the process
	// inside exits immediately (e.g. nbdkit failing on a missing device). Confirm the
	// container is actually up before reporting the export as ready, and tear down a
	// failed one so it doesn't linger crash-looping (restart=always) and get falsely
	// reused by WWID on the next reconcile.
	if err := r.verifyUp(ctx, name); err != nil {
		_ = exec.CommandContext(context.Background(), "podman", "rm", "-f", name).Run()
		return err
	}
	return nil
}

// verifyUp waits briefly for the container to settle, then fails if it is not running.
// A container that crashes on startup flaps under restart=always, so a nonzero restart
// count is a reliable "it started but is failing" signal even if a single inspect
// happens to catch it mid-restart in the "running" state.
func (r *Runner) verifyUp(ctx context.Context, name string) error {
	const settle = 750 * time.Millisecond
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(settle):
	}

	out, err := exec.CommandContext(ctx, "podman", "inspect",
		"--format", "{{.State.Status}} {{.RestartCount}}", name).Output()
	if err != nil {
		return fmt.Errorf("podman inspect %s: %w", name, err)
	}
	fields := strings.Fields(string(out))
	if len(fields) != 2 {
		return fmt.Errorf("unexpected inspect output: %q", strings.TrimSpace(string(out)))
	}
	status := fields[0]
	restarts, err := strconv.Atoi(fields[1])
	if err != nil {
		return fmt.Errorf("parsing restart count %q: %w", fields[1], err)
	}
	if status != "running" || restarts > 0 {
		return fmt.Errorf("container %s failed to start (status=%s, restarts=%d): %s",
			name, status, restarts, r.lastLog(ctx, name))
	}
	return nil
}

// lastLog returns the tail of a container's logs for inclusion in error messages.
func (r *Runner) lastLog(ctx context.Context, name string) string {
	out, err := exec.CommandContext(ctx, "podman", "logs", "--tail", "3", name).CombinedOutput()
	if err != nil || len(bytes.TrimSpace(out)) == 0 {
		return "(no container logs)"
	}
	return strings.TrimSpace(string(out))
}

// existingPort returns the host port of a healthy running container for wwid.
func (r *Runner) existingPort(ctx context.Context, wwid string) (int, bool, error) {
	running, err := r.containers(ctx, wwid, "running")
	if err != nil {
		return 0, false, err
	}
	if len(running) == 0 {
		return 0, false, nil
	}
	name := running[0]
	if err := r.verifyUp(ctx, name); err == nil {
		port, err := r.publishedPort(ctx, name)
		if err != nil {
			return 0, false, err
		}
		return port, true, nil
	}
	return 0, false, nil
}

func (r *Runner) removeContainers(ctx context.Context, wwid string) error {
	names, err := r.containers(ctx, wwid, "")
	if err != nil {
		return err
	}
	for _, name := range names {
		if out, err := exec.CommandContext(ctx, "podman", "rm", "-f", name).CombinedOutput(); err != nil {
			return fmt.Errorf("removing container %s: %w: %s",
				name, err, strings.TrimSpace(string(out)))
		}
	}
	return nil
}

// containers lists the names of this tool's containers for one WWID. An empty state means
// every state, as `podman ps -a` reports them.
func (r *Runner) containers(ctx context.Context, wwid, state string) ([]string, error) {
	args := []string{"ps", "-a",
		"--filter", "label=" + wwidLabel + "=" + wwid,
		"--format", "{{.Names}}"}
	if state != "" {
		args = append(args, "--filter", "status="+state)
	}
	out, err := exec.CommandContext(ctx, "podman", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("podman ps: %w", err)
	}
	// Container names never contain whitespace, so the lines split cleanly.
	return strings.Fields(string(out)), nil
}

// inspectPortBindings models the subset of `podman inspect` we need.
type inspectPortBindings struct {
	NetworkSettings struct {
		Ports map[string][]struct {
			HostPort string `json:"HostPort"`
		} `json:"Ports"`
	} `json:"NetworkSettings"`
}

// publishedPort reads the host port mapped to the container's nbdkit port.
func (r *Runner) publishedPort(ctx context.Context, name string) (int, error) {
	out, err := exec.CommandContext(ctx, "podman", "inspect", name).Output()
	if err != nil {
		return 0, fmt.Errorf("podman inspect %s: %w", name, err)
	}
	var inspected []inspectPortBindings
	if err := json.NewDecoder(bytes.NewReader(out)).Decode(&inspected); err != nil {
		return 0, fmt.Errorf("decoding inspect for %s: %w", name, err)
	}
	if len(inspected) == 0 {
		return 0, fmt.Errorf("no inspect data for %s", name)
	}
	key := fmt.Sprintf("%d/tcp", containerPort)
	bindings := inspected[0].NetworkSettings.Ports[key]
	if len(bindings) > 0 {
		return strconv.Atoi(bindings[0].HostPort)
	}
	return r.labeledPort(ctx, name)
}

// labeledPort reads the host port from the container label written at create time.
func (r *Runner) labeledPort(ctx context.Context, name string) (int, error) {
	out, err := exec.CommandContext(ctx, "podman", "inspect", name,
		"--format", "{{index .Config.Labels \""+portLabel+"\"}}").Output()
	if err != nil {
		return 0, fmt.Errorf("podman inspect %s: %w", name, err)
	}
	portStr := strings.TrimSpace(string(out))
	if portStr == "" {
		return 0, fmt.Errorf("container %s has no %s label", name, portLabel)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 0, fmt.Errorf("parsing %s label on %s: %w", portLabel, name, err)
	}
	return port, nil
}

// usedPorts returns the set of host ports already published by nbd containers, so we
// don't try to reuse them for newly discovered devices.
func (r *Runner) usedPorts(ctx context.Context) (map[int]bool, error) {
	out, err := exec.CommandContext(ctx, "podman", "ps", "-a",
		"--filter", "label="+wwidLabel,
		"--format", "{{.Names}}").Output()
	if err != nil {
		return nil, fmt.Errorf("podman ps: %w", err)
	}
	used := make(map[int]bool)
	for _, name := range strings.Fields(string(out)) {
		if port, err := r.publishedPort(ctx, name); err == nil {
			used[port] = true
		}
	}
	return used, nil
}

var nonAlnum = regexp.MustCompile(`[^a-zA-Z0-9]+`)

func sanitize(s string) string {
	return strings.Trim(nonAlnum.ReplaceAllString(s, "-"), "-")
}
