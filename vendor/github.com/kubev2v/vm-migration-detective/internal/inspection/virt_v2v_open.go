package inspection

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
)

type V2VSession struct {
	NBDURL string
	cmd    *exec.Cmd
}

func OpenWithVirtV2V(
	ctx context.Context,
	vmMoref string,
	snapshotMoref string,
	vcenterURL string,
	username string,
	password string,
) (*V2VSession, error) {

	parsedURL, err := url.Parse(vcenterURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse vCenter URL: %w", err)
	}

	vcenterHost := parsedURL.Hostname()

	// Build vpx source URL using VMMoref
	// Format: vpx://user@host/?moref=vm-123&snapshot=snapshot-456&no_verify=1&password=...
	vpxURL := fmt.Sprintf(
		"vpx://%s@%s/?moref=%s&snapshot=%s&no_verify=1&password=%s",
		username,
		vcenterHost,
		vmMoref,
		snapshotMoref,
		password,
	)

	args := []string{
		"-it", "vddk",
		vpxURL,
		"-o", "nbd",
	}

	cmd := exec.CommandContext(ctx, "virt-v2v-open", args...)

	// Optional: pipe output to your logger / stdout for debugging
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start virt-v2v-open: %w", err)
	}

	// Default port used by virt-v2v-open
	nbdURL := "nbd://localhost:10809"

	return &V2VSession{
		NBDURL: nbdURL,
		cmd:    cmd,
	}, nil
}

func (s *V2VSession) Close() {
	if s != nil && s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
		_, _ = s.cmd.Process.Wait()
	}
}
