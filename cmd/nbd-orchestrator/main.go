// Command nbd-orchestrator discovers non-root block devices on the host, starts one
// nbdkit container per device (read-only), and serves a mutual-TLS HTTPS endpoint
// announcing the exported disks. NBD itself is plain TCP unless -tls is set.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/kubev2v/forklift/pkg/nbd-container/announce"
	"github.com/kubev2v/forklift/pkg/nbd-container/blockdev"
	"github.com/kubev2v/forklift/pkg/nbd-container/runner"
)

func main() {
	certsDir := flag.String("certs-dir", "/etc/pki/nbd",
		"directory with "+announce.CACert+", "+announce.ServerCert+", "+announce.ServerKey)
	image := flag.String("image", "localhost/nbd-container", "nbdkit container image")
	listen := flag.String("listen", ":8443", "address for the HTTPS announce server")
	basePort := flag.Int("base-port", 10809, "first host port to allocate for exports")
	publishIP := flag.String("publish-ip", "0.0.0.0", "host IP to bind published NBD ports to")
	tls := flag.Bool("tls", false, "require mutual TLS on nbdkit NBD exports")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	if err := run(logger, *certsDir, *image, *listen, *basePort, *publishIP, *tls); err != nil {
		logger.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger, certsDir, image, listen string, basePort int, publishIP string, tls bool) error {
	// Resolve the cert dir to an absolute path for the announce server and nbdkit mounts.
	absCertsDir, err := filepath.Abs(certsDir)
	if err != nil {
		return fmt.Errorf("resolving certs dir %q: %w", certsDir, err)
	}
	certsDir = absCertsDir

	// Fail fast if the shared certificates are missing.
	for _, f := range []string{announce.CACert, announce.ServerCert, announce.ServerKey} {
		if _, err := os.Stat(filepath.Join(certsDir, f)); err != nil {
			return err
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// 1. Discover exportable block devices.
	devices, err := blockdev.Discover(ctx)
	if err != nil {
		return err
	}
	logger.Info("discovered devices", "count", len(devices), "tls", tls)

	// 2. Start (or reuse) one container per device.
	r := runner.New(runner.Config{
		Image:     image,
		BasePort:  basePort,
		PublishIP: publishIP,
		CertsDir:  certsDir,
		TLS:       tls,
	})
	exports, err := r.Reconcile(ctx, devices)
	if err != nil {
		// Non-fatal: some containers may still have started; log and serve the rest.
		logger.Warn("some containers failed to start", "err", err)
	}
	for _, e := range exports {
		logger.Info("export ready", "wwid", e.WWID, "device", e.Device, "port", e.Port)
	}

	// 3. Serve the mutual-TLS announce endpoint.
	srv, err := announce.New(listen, certsDir, exports)
	if err != nil {
		return err
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("announce server listening", "addr", listen)
		errCh <- srv.ListenAndServe()
	}()

	var runErr error
	select {
	case <-ctx.Done():
		logger.Info("shutting down")
	case runErr = <-errCh:
		logger.Error("announce server stopped", "err", runErr)
	}

	// Best-effort graceful teardown. ctx is already cancelled on signal, so use a fresh
	// context. The nbd containers are owned by this supervisor: remove them so no NBD
	// exports outlive the process that announced them.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Warn("announce server shutdown", "err", err)
	}
	logger.Info("removing nbd containers")
	if err := r.Shutdown(shutdownCtx); err != nil {
		logger.Warn("failed to remove some nbd containers", "err", err)
	}
	return runErr
}
