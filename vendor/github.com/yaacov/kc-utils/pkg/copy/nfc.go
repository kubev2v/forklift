package copy

import (
	"context"
	"fmt"
	"io"

	"github.com/vmware/govmomi/vim25/soap"
)

// downloadURL opens an ESXi NFC export HTTPS stream via the govmomi client that
// acquired the lease. This is node transport: ESXi host thumbprints from the NFC
// lease are registered during lease.Wait(), not CopyInput.fingerprint (vCenter).
func (l *Lease) downloadURL(ctx context.Context, rawURL string) (io.ReadCloser, error) {
	u, err := l.client.ParseURL(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse NFC URL: %w", err)
	}
	rc, _, err := l.client.Download(ctx, u, &soap.DefaultDownload)
	if err != nil {
		return nil, err
	}
	return rc, nil
}
