// nutanix-http-populator downloads a Nutanix catalog image over HTTP and
// writes it into the target volume.
//
// Auth comes from the EnvFrom secret wired by the populator machinery:
//   - Basic Auth: user + password (Prism Element, or PC fallback to v3 /file)
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
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/kubev2v/forklift/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/klog/v2"
)

const (
	maxDownloadAttempts       = 8
	downloadRetryBase         = 2 * time.Second
	entityDownloadPathPrefix  = "/entity_download/"
	httpDialTimeout           = 30 * time.Second
	httpResponseHeaderTimeout = 60 * time.Second
	httpBodyIdleTimeout       = 5 * time.Minute
)

var errBodyIdleTimeout = errors.New("download body idle timeout")

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
		klog.Infof("Downloading %s -> %s (pvc-size=%d)", cfg.url, cfg.volumePath, cfg.pvcSize)
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

// download tries the configured URL first. If that fails and the URL is a PC
// entity_download Location, it rewrites to PE v3 /file and retries with Basic
// Auth — some AOS builds reject entity_download while /file still works.
func download(ctx context.Context, cfg *config, progress *prometheus.GaugeVec) error {
	err := downloadRetries(ctx, cfg, progress, cfg.url, false)
	if err == nil {
		return nil
	}
	fileURL, ok := entityDownloadToV3FileURL(cfg.url)
	if !ok {
		return err
	}
	klog.Warningf("entity_download failed (%v); falling back to %s with Basic Auth", err, fileURL)
	return downloadRetries(ctx, cfg, progress, fileURL, true)
}

func downloadRetries(ctx context.Context, cfg *config, progress *prometheus.GaugeVec, url string, forceBasic bool) error {
	var lastErr error
	for attempt := 1; attempt <= maxDownloadAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		if attempt > 1 {
			progress.WithLabelValues(cfg.ownerUID).Set(0)
			delay := time.Duration(attempt-1) * downloadRetryBase
			klog.Infof("Retrying download attempt %d/%d after %v (last error: %v)",
				attempt, maxDownloadAttempts, delay, lastErr)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}

		lastErr = downloadOnce(ctx, cfg, progress, url, forceBasic, attempt)
		if lastErr == nil {
			return nil
		}
		if !isRetryable(lastErr) {
			return lastErr
		}
		klog.Warningf("Transient download failure on attempt %d/%d: %v",
			attempt, maxDownloadAttempts, lastErr)
	}
	return fmt.Errorf("download failed after %d attempts: %w", maxDownloadAttempts, lastErr)
}

func applyDownloadAuth(req *http.Request, forceBasic bool) error {
	if req.URL == nil || !strings.EqualFold(req.URL.Scheme, "https") {
		scheme := ""
		if req.URL != nil {
			scheme = req.URL.Scheme
		}
		return fmt.Errorf("refusing to send Prism credentials over %q (HTTPS required)", scheme)
	}
	if !forceBasic {
		if cookie := os.Getenv("Cookie"); cookie != "" {
			req.Header.Set("Cookie", cookie)
			return nil
		}
		if header := os.Getenv("cookie"); strings.HasPrefix(header, "Cookie: ") {
			req.Header.Set("Cookie", strings.TrimPrefix(header, "Cookie: "))
			return nil
		}
	}
	user := os.Getenv("user")
	password := os.Getenv("password")
	if user != "" || password != "" {
		req.SetBasicAuth(user, password)
	}
	return nil
}

func downloadOnce(ctx context.Context, cfg *config, progress *prometheus.GaugeVec, rawURL string, forceBasic bool, attempt int) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	if err := applyDownloadAuth(req, forceBasic); err != nil {
		return err
	}

	client := &http.Client{
		Timeout: 0,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   httpDialTimeout,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			TLSClientConfig:       tlsConfigFromEnv(),
			TLSHandshakeTimeout:   httpDialTimeout,
			ResponseHeaderTimeout: httpResponseHeaderTimeout,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return errors.New("stopped after 10 redirects")
			}
			if req.URL == nil || !strings.EqualFold(req.URL.Scheme, "https") {
				return fmt.Errorf("refusing redirect to non-HTTPS URL")
			}
			return nil
		},
	}

	klog.Infof("Connecting to Prism (attempt %d/%d)...", attempt, maxDownloadAttempts)
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	klog.Infof("HTTP %d in %v (Content-Length=%s Transfer-Encoding=%s)",
		resp.StatusCode, time.Since(start).Round(time.Millisecond),
		resp.Header.Get("Content-Length"), resp.Header.Get("Transfer-Encoding"))
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("download failed: status=%d body=%q", resp.StatusCode, string(body))
	}

	file, err := openVolume(cfg.volumePath)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	if err := resetVolume(file); err != nil {
		return err
	}

	read, err := copyBodyToVolume(ctx, cfg, progress, &idleReader{r: resp.Body, idle: httpBodyIdleTimeout}, file, start)
	if err != nil {
		return err
	}
	if read == 0 {
		return fmt.Errorf("download returned empty body")
	}
	klog.Infof("Download complete: %d bytes in %v", read, time.Since(start).Round(time.Millisecond))
	return nil
}

func copyBodyToVolume(
	ctx context.Context,
	cfg *config,
	progress *prometheus.GaugeVec,
	body io.Reader,
	file *os.File,
	start time.Time,
) (int64, error) {
	done := make(chan struct{})
	var read int64
	go reportDownloadProgress(cfg, progress, &read, done)

	buf := make([]byte, 1<<20)
	firstByte := true
	for {
		if err := ctx.Err(); err != nil {
			close(done)
			return read, err
		}
		n, rerr := body.Read(buf)
		if n > 0 {
			if firstByte {
				klog.Infof("First byte after %v", time.Since(start).Round(time.Millisecond))
				firstByte = false
			}
			if _, werr := file.Write(buf[:n]); werr != nil {
				close(done)
				return read, werr
			}
			read += int64(n)
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			close(done)
			return read, rerr
		}
	}
	close(done)
	return read, nil
}

func reportDownloadProgress(cfg *config, progress *prometheus.GaugeVec, read *int64, done <-chan struct{}) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			if cfg.pvcSize <= 0 {
				continue
			}
			pct := (float64(*read) / float64(cfg.pvcSize)) * 100
			if pct > 100 {
				pct = 100
			}
			progress.WithLabelValues(cfg.ownerUID).Set(pct)
			klog.Infof("Progress: %.0f%% (%d/%d bytes)", pct, *read, cfg.pvcSize)
		}
	}
}

// entityDownloadToV3FileURL rewrites a PE entity_download Location to the
// documented v3 image /file URL on the same host.
func entityDownloadToV3FileURL(raw string) (string, bool) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", false
	}
	if !strings.HasPrefix(u.Path, entityDownloadPathPrefix) {
		return "", false
	}
	imageUUID := strings.TrimPrefix(u.Path, entityDownloadPathPrefix)
	if imageUUID == "" || strings.Contains(imageUUID, "/") {
		return "", false
	}
	u.Path = "/api/nutanix/v3/images/" + imageUUID + "/file"
	u.RawPath = ""
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), true
}

func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, errBodyIdleTimeout) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	s := err.Error()
	return strings.Contains(s, "status=429") ||
		strings.Contains(s, "status=503") ||
		strings.Contains(s, "connection reset") ||
		strings.Contains(s, "broken pipe") ||
		strings.Contains(s, "Server closed idle connection") ||
		strings.Contains(s, "use of closed network connection") ||
		strings.Contains(s, "timeout awaiting response headers")
}

// idleReader fails a Read that makes no progress within idle.
type idleReader struct {
	r    io.Reader
	idle time.Duration
}

func (i *idleReader) Read(p []byte) (int, error) {
	type result struct {
		n   int
		err error
	}
	ch := make(chan result, 1)
	go func() {
		n, err := i.r.Read(p)
		ch <- result{n, err}
	}()
	timer := time.NewTimer(i.idle)
	defer timer.Stop()
	select {
	case res := <-ch:
		return res.n, res.err
	case <-timer.C:
		return 0, errBodyIdleTimeout
	}
}

func openVolume(volumePath string) (*os.File, error) {
	flags := os.O_RDWR
	if strings.HasSuffix(volumePath, "disk.img") {
		flags |= os.O_CREATE
	}
	return os.OpenFile(volumePath, flags, 0650)
}

// resetVolume rewinds the target so a retry overwrites from offset 0 instead of appending.
func resetVolume(file *os.File) error {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if err := file.Truncate(0); err != nil {
		// Block devices often reject Truncate; seeking to 0 is enough there.
		klog.V(2).Infof("Truncate skipped: %v", err)
	}
	return nil
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
