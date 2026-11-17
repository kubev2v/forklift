// nutanix-http-populator downloads a Nutanix catalog image over HTTP and
// writes it into the target volume.
//
// Auth comes from the EnvFrom secret wired by the populator machinery:
//   - Basic Auth: user + password (Prism Element)
//   - Cookie header: Cookie env (Prism Central entity_download)
//
// TLS: cacert env (mapped from ca.crt) and insecureSkipVerify.
//
// Concurrent downloads against one Prism are limited with k8s Leases (same
// slot pattern as the vSphere copy-offload populator).
package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/kubev2v/forklift/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/klog/v2"
)

type config struct {
	volumePath           string
	url                  string
	pvcSize              int64
	crName               string
	crNS                 string
	ownerUID             string
	secretName           string
	leaseKey             string
	leaseNamespace       string
	leaseDurationSeconds int
	downloadSlots        int
}

func main() {
	cfg := &config{}
	flag.StringVar(&cfg.volumePath, "volume-path", "", "Path to populate (block device or disk.img)")
	flag.StringVar(&cfg.url, "url", "", "HTTP(S) download URL for the catalog image")
	flag.Int64Var(&cfg.pvcSize, "pvc-size", 0, "Size of the source disk (bytes)")
	flag.StringVar(&cfg.crName, "cr-name", "", "Custom Resource name")
	flag.StringVar(&cfg.crNS, "cr-namespace", "", "Custom Resource namespace")
	flag.StringVar(&cfg.ownerUID, "owner-uid", "", "Owner UID (target PVC UID), used as the progress label")
	flag.StringVar(&cfg.secretName, "secret-name", "", "Secret name (credentials via EnvFrom)")
	flag.StringVar(&cfg.leaseKey, "lease-key", "", "DNS label identifying the Prism provider for download leases")
	flag.StringVar(&cfg.leaseNamespace, "lease-namespace", "openshift-mtv", "Namespace for download lease objects")
	flag.IntVar(&cfg.leaseDurationSeconds, "lease-duration-seconds", 30, "Lease duration in seconds (renewed while downloading)")
	flag.IntVar(&cfg.downloadSlots, "download-slots", 2, "Max concurrent downloads for this lease key")
	flag.Parse()

	if cfg.volumePath == "" || cfg.url == "" {
		klog.Fatal("volume-path and url are required")
	}
	if cfg.pvcSize <= 0 {
		klog.Fatal("pvc-size must be greater than 0")
	}

	certsDir, err := os.MkdirTemp("", "certsdir")
	if err != nil {
		klog.Fatal(err)
	}
	metrics.StartPrometheusEndpoint(certsDir)

	progress := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "nutanix_http_populator_progress",
			Help: "Percentage of the Nutanix disk downloaded over HTTP",
		},
		[]string{"ownerUID"},
	)
	if err := prometheus.Register(progress); err != nil {
		klog.Error("progress gauge not registered: ", err)
	}

	ctx := context.Background()
	run := func(ctx context.Context) error {
		klog.Infof("Downloading %s -> %s", cfg.url, cfg.volumePath)
		return download(ctx, cfg, progress)
	}

	if cfg.leaseKey != "" {
		locker, err := newDownloadLeaseLocker(cfg.downloadSlots, cfg.leaseNamespace, cfg.leaseDurationSeconds)
		if err != nil {
			klog.Fatal(err)
		}
		klog.Infof("Waiting for download lease key=%s slots=%d ns=%s", cfg.leaseKey, cfg.downloadSlots, cfg.leaseNamespace)
		err = locker.withLock(ctx, cfg.leaseKey, run)
		if err != nil {
			klog.Fatal(err)
		}
	} else {
		if err := run(ctx); err != nil {
			klog.Fatal(err)
		}
	}
	progress.WithLabelValues(cfg.ownerUID).Set(100)
	klog.Info("Finished populating the volume. Progress: 100%")
}

func download(ctx context.Context, cfg *config, progress *prometheus.GaugeVec) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.url, nil)
	if err != nil {
		return err
	}
	if cookie := os.Getenv("Cookie"); cookie != "" {
		req.Header.Set("Cookie", cookie)
	} else if header := os.Getenv("cookie"); strings.HasPrefix(header, "Cookie: ") {
		req.Header.Set("Cookie", strings.TrimPrefix(header, "Cookie: "))
	} else {
		user := os.Getenv("user")
		password := os.Getenv("password")
		if user != "" || password != "" {
			req.SetBasicAuth(user, password)
		}
	}

	client := &http.Client{
		Timeout: 0,
		Transport: &http.Transport{
			TLSClientConfig: tlsConfigFromEnv(),
			Proxy:           http.ProxyFromEnvironment,
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("download failed: status=%d body=%q", resp.StatusCode, string(body))
	}

	file, err := openVolume(cfg.volumePath)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	done := make(chan struct{})
	var read int64
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				if cfg.pvcSize > 0 {
					pct := (float64(read) / float64(cfg.pvcSize)) * 100
					if pct > 100 {
						pct = 100
					}
					progress.WithLabelValues(cfg.ownerUID).Set(pct)
					klog.Infof("Progress: %.0f%%", pct)
				}
			}
		}
	}()

	buf := make([]byte, 1<<20)
	for {
		if err := ctx.Err(); err != nil {
			close(done)
			return err
		}
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := file.Write(buf[:n]); werr != nil {
				close(done)
				return werr
			}
			read += int64(n)
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			close(done)
			return rerr
		}
	}
	close(done)
	return nil
}

func openVolume(volumePath string) (*os.File, error) {
	flags := os.O_RDWR
	if strings.HasSuffix(volumePath, "disk.img") {
		flags |= os.O_CREATE
	}
	return os.OpenFile(volumePath, flags, 0650)
}

func tlsConfigFromEnv() *tls.Config {
	cfg := &tls.Config{MinVersion: tls.VersionTLS12}
	if os.Getenv("insecureSkipVerify") == "true" {
		cfg.InsecureSkipVerify = true // #nosec G402 - honoring the provider's insecureSkipVerify setting
	}
	if ca := os.Getenv("cacert"); ca != "" {
		pool := x509.NewCertPool()
		if pool.AppendCertsFromPEM([]byte(ca)) {
			cfg.RootCAs = pool
		}
	}
	return cfg
}
