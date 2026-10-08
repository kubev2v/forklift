package copyappliance

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/template"
	"time"

	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/nbd-container/announce"
)

//go:embed nbd-orchestrator.service.tmpl
var orchestratorUnitTemplate string

var orchestratorUnitText = template.Must(
	template.New(orchestratorUnit).Parse(orchestratorUnitTemplate))

// Orchestrator is the NBD supervisor on the appliance, reached over SSH.
type Orchestrator struct {
	context *ApplianceContext
	ssh     *SSHClient
}

// NewOrchestrator logs in. The caller owns the result and must Close it.
func NewOrchestrator(ctx context.Context, ac *ApplianceContext, timeout time.Duration) (*Orchestrator, bool, error) {
	client, ready, err := ac.SSHClient(ctx, timeout)
	if err != nil || !ready {
		return nil, ready, err
	}
	return &Orchestrator{context: ac, ssh: client}, true, nil
}

func (r *Orchestrator) Close() error {
	return r.ssh.Close()
}

// renderUnit builds the systemd unit for this appliance's loaded image.
func (r *Orchestrator) renderUnit() (string, error) {
	image := r.context.Appliance.Status.ExporterImage
	if image == "" {
		return "", liberr.New(
			"the appliance has no loaded image to supervise",
			"appliance", r.context.Appliance.Name)
	}
	var buf bytes.Buffer
	err := orchestratorUnitText.Execute(&buf, struct {
		Binary       string
		CertsDir     string
		Image        string
		AnnouncePort string
		BasePort     int
		TLS          bool
	}{
		Binary:       orchestratorBinary,
		CertsDir:     applianceCertsDir,
		Image:        image,
		AnnouncePort: applianceAnnouncePort,
		BasePort:     applianceBasePort,
		TLS:          r.context.NbdSsl,
	})
	if err != nil {
		return "", liberr.Wrap(err)
	}
	return buf.String(), nil
}

// Installed reports whether the appliance already has the binary, unit, and
// certs Install would push. Kept separate from Active so a crash-looping unit
// is not reinstalled (and its exports torn down) on every reconcile.
func (r *Orchestrator) Installed() (bool, error) {
	unit, err := r.renderUnit()
	if err != nil {
		return false, err
	}
	certs, err := r.context.ServerTLS()
	if err != nil {
		return false, err
	}
	manifest, err := r.manifest(unit, certs)
	if err != nil {
		return false, err
	}
	err = r.ssh.RunWithStdin("sha256sum --status -c -", strings.NewReader(manifest))
	if err == nil {
		return true, nil
	}
	if IsExitError(err) {
		return false, nil
	}
	return false, err
}

// manifest is a sha256sum -c check file for everything Install writes.
func (r *Orchestrator) manifest(unit string, certs map[string][]byte) (string, error) {
	binarySum, err := fileSum(orchestratorBinary)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s  %s\n", binarySum, orchestratorBinary)
	fmt.Fprintf(&b, "%x  %s\n", sha256.Sum256([]byte(unit)), orchestratorService)
	names := make([]string, 0, len(certs))
	for name := range certs {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Fprintf(&b, "%x  %s\n", sha256.Sum256(certs[name]), applianceCertsDir+"/"+name)
	}
	return b.String(), nil
}

// Install ships certs, binary, and unit, then enables and starts the service.
func (r *Orchestrator) Install() error {
	unit, err := r.renderUnit()
	if err != nil {
		return err
	}
	certs, err := r.context.ServerTLS()
	if err != nil {
		return err
	}
	if err := r.ssh.RunCommand("install -d -m 0700 " + applianceCertsDir); err != nil {
		return err
	}
	for _, name := range []string{announce.CACert, announce.ServerCert, announce.ServerKey} {
		if err := r.put(applianceCertsDir+"/"+name, bytes.NewReader(certs[name])); err != nil {
			return err
		}
	}
	if err := r.installBinary(); err != nil {
		return err
	}
	if err := r.put(orchestratorService, strings.NewReader(unit)); err != nil {
		return err
	}
	for _, cmd := range []string{
		"systemctl daemon-reload",
		"systemctl enable " + orchestratorUnit,
	} {
		if err := r.ssh.RunCommand(cmd); err != nil {
			return err
		}
	}
	return r.Restart()
}

func (r *Orchestrator) put(path string, in io.Reader) error {
	return r.ssh.RunWithStdin("cat > "+path, in)
}

func (r *Orchestrator) installBinary() error {
	f, err := os.Open(orchestratorBinary)
	if err != nil {
		return liberr.Wrap(err, "path", orchestratorBinary)
	}
	defer func() { _ = f.Close() }()
	return r.ssh.RunWithStdin(
		"cat > "+orchestratorStaging+" && chmod 0755 "+orchestratorStaging+" && mv -f "+orchestratorStaging+" "+orchestratorBinary,
		f)
}

// Active reports whether systemd says the unit is running. Not proof that
// exports work — use WaitForExports for that.
func (r *Orchestrator) Active() (bool, error) {
	err := r.ssh.RunCommand("systemctl is-active --quiet " + orchestratorUnit)
	if err == nil {
		return true, nil
	}
	if IsExitError(err) {
		return false, nil
	}
	return false, err
}

// Start brings the supervisor up without tearing down running exports.
func (r *Orchestrator) Start() error {
	return r.resetAndRun("start")
}

// Restart reloads the supervisor after hot-attaching disks.
func (r *Orchestrator) Restart() error {
	return r.resetAndRun("restart")
}

func (r *Orchestrator) resetAndRun(verb string) error {
	if err := r.ssh.RunCommand("systemctl reset-failed " + orchestratorUnit); err != nil {
		return err
	}
	return r.ssh.RunCommand("systemctl " + verb + " " + orchestratorUnit)
}

// Log returns a short journal tail for diagnosing a unit that will not come up.
func (r *Orchestrator) Log() string {
	session, err := r.ssh.Client.NewSession()
	if err != nil {
		return "(no journal)"
	}
	defer func() { _ = session.Close() }()
	out, _ := session.CombinedOutput("journalctl -u " + orchestratorUnit + " --no-pager -n 20")
	if tail := strings.TrimSpace(string(out)); tail != "" {
		return tail
	}
	return "(no journal)"
}

func fileSum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", liberr.Wrap(err, "path", path)
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", liberr.Wrap(err, "path", path)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
