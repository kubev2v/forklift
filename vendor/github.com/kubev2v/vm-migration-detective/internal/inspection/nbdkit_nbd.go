package inspection

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/kubev2v/vm-migration-detective/internal/cmdbuilder"
	"github.com/sirupsen/logrus"
)

// ResolveNBDURLsForInspector returns URLs virt-inspector can open.
// nbds:// is proxied via nbdkit-nbd (libguestfs has no nbds protocol).
// TLS hostname is fixed to "nbd-server" (copy-appliance cert CN; clients use guest IP).
func ResolveNBDURLsForInspector(ctx context.Context, nbdURLs []string, tlsCertificates string, logger *logrus.Logger) ([]string, func(), error) {
	var sessions []*NBDKitSession
	closer := func() {
		for _, s := range sessions {
			s.Close()
		}
	}

	out := make([]string, 0, len(nbdURLs))
	for _, raw := range nbdURLs {
		if !strings.HasPrefix(raw, "nbds://") && !strings.HasPrefix(raw, "nbds+") {
			out = append(out, raw)
			continue
		}
		if tlsCertificates == "" {
			closer()
			return nil, func() {}, fmt.Errorf("nbds:// URL requires TLSCertificates directory: %s", raw)
		}
		session, err := openNBDProxy(ctx, raw, tlsCertificates, logger)
		if err != nil {
			closer()
			return nil, func() {}, err
		}
		sessions = append(sessions, session)
		out = append(out, session.NBDURL)
	}
	return out, closer, nil
}

func openNBDProxy(ctx context.Context, remoteURL, tlsCertificates string, logger *logrus.Logger) (*NBDKitSession, error) {
	socketPath := filepath.Join("/tmp", fmt.Sprintf("nbdkit-nbd-%s.sock", uuid.New().String()))
	proxyURI := remoteURL
	if u, err := url.Parse(remoteURL); err == nil {
		q := u.Query()
		q.Del("tls-certificates")
		q.Set("tls-hostname", "nbd-server")
		u.RawQuery = q.Encode()
		proxyURI = u.String()
	}

	cmd := cmdbuilder.New().
		WithLogger(logger).
		Flag("-U", socketPath).
		Add("--foreground").
		Add("--exit-with-parent").
		Add("-r").
		Add("nbd").
		Add("uri="+proxyURI).
		Add("tls-certificates="+tlsCertificates).
		Command(ctx, "nbdkit")

	var stderrBuf, stdoutBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start nbdkit nbd proxy: %w", err)
	}

	session := &NBDKitSession{
		NBDURL:     fmt.Sprintf("nbd+unix:///?socket=%s", socketPath),
		socketPath: socketPath,
		cmd:        cmd,
		logger:     logger,
		stderrBuf:  &stderrBuf,
		stdoutBuf:  &stdoutBuf,
	}
	if err := session.WaitForReady(30 * time.Second); err != nil {
		session.Close()
		return nil, fmt.Errorf("nbdkit nbd proxy not ready for %s: %w", remoteURL, err)
	}
	return session, nil
}
