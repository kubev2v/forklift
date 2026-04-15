package services

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// AAPJobTemplatesPath is the forklift-services path that proxies to the AAP Controller API job_templates list.
	AAPJobTemplatesPath = "/aap/job-templates"
	headerAAPToken      = "X-AAP-Token"
	aapProxyTimeout     = 60 * time.Second
	aapJobTemplatesPath = "/api/controller/v2/job_templates/"
)

// aapProxyHTTPClient is the client used for upstream AAP requests (overridable in tests).
var aapProxyHTTPClient = &http.Client{Timeout: aapProxyTimeout}

func serveAAPJobTemplates(resp http.ResponseWriter, req *http.Request, _ client.Client) {
	if req.Method != http.MethodGet {
		http.Error(resp, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rawBase := req.URL.Query().Get("url")
	if rawBase == "" {
		http.Error(resp, "required query parameter: url", http.StatusBadRequest)
		return
	}

	aapBase, err := url.Parse(rawBase)
	if err != nil || aapBase.Scheme == "" || aapBase.Host == "" {
		log.Info("invalid AAP url query parameter", "url", rawBase)
		http.Error(resp, "invalid url query parameter", http.StatusBadRequest)
		return
	}

	if err := validateAAPUpstreamURL(aapBase); err != nil {
		log.Info("rejected AAP upstream url", "url", rawBase, "reason", err.Error())
		http.Error(resp, err.Error(), http.StatusBadRequest)
		return
	}

	token := req.Header.Get(headerAAPToken)
	if strings.TrimSpace(token) == "" {
		http.Error(resp, fmt.Sprintf("required header: %s", headerAAPToken), http.StatusBadRequest)
		return
	}

	q := req.URL.Query()
	q.Del("url")
	upstreamQuery := q.Encode()

	target, err := aapJobTemplatesTargetURL(aapBase, upstreamQuery)
	if err != nil {
		log.Error(err, "failed to build AAP target URL")
		http.Error(resp, "invalid url query parameter", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(req.Context(), aapProxyTimeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		log.Error(err, "failed to build AAP request", "target", target)
		http.Error(resp, err.Error(), http.StatusInternalServerError)
		return
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)

	out, err := aapProxyHTTPClient.Do(httpReq)
	if err != nil {
		log.Error(err, "AAP job_templates request failed", "target", target)
		http.Error(resp, err.Error(), http.StatusBadGateway)
		return
	}
	defer out.Body.Close()

	if ct := out.Header.Get("Content-Type"); ct != "" {
		resp.Header().Set("Content-Type", ct)
	}
	resp.WriteHeader(out.StatusCode)
	if _, err := io.Copy(resp, out.Body); err != nil {
		log.Error(err, "failed to write AAP response body")
	}
}

// validateAAPUpstreamURL rejects schemes and hosts that would turn this service into an open proxy (SSRF).
func validateAAPUpstreamURL(u *url.URL) error {
	if !strings.EqualFold(u.Scheme, "https") {
		return fmt.Errorf("url scheme must be https")
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("url host is required")
	}
	if isBlockedAAPHostname(host) {
		return fmt.Errorf("url host is not allowed")
	}
	if ip := net.ParseIP(host); ip != nil {
		if isBlockedAAPIP(ip) {
			return fmt.Errorf("url host is not allowed")
		}
	}
	return nil
}

func isBlockedAAPHostname(host string) bool {
	h := strings.ToLower(strings.TrimSpace(host))
	switch h {
	case "localhost":
		return true
	}
	if strings.HasSuffix(h, ".svc.cluster.local") || strings.HasSuffix(h, ".svc") {
		return true
	}
	switch h {
	case "metadata.google.internal", "kubernetes.default.svc.cluster.local":
		return true
	}
	return false
}

func isBlockedAAPIP(ip net.IP) bool {
	// Block loopback, unspecified, and multicast. Do not block RFC1918: on-prem AAP
	// commonly uses private IPs reachable only from the cluster.
	if ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalMulticast() {
		return true
	}
	// Link-local unicast (e.g. fe80::/10); allow RFC1918 private networks for customer AAP.
	if ip.IsLinkLocalUnicast() {
		return true
	}
	// Cloud instance metadata
	if ip.Equal(net.IPv4(169, 254, 169, 254)) {
		return true
	}
	return false
}

// aapJobTemplatesTargetURL builds the AAP Controller list URL from a parsed base using url.URL (not string concatenation).
func aapJobTemplatesTargetURL(base *url.URL, upstreamQuery string) (string, error) {
	if base == nil {
		return "", fmt.Errorf("base url is nil")
	}
	t := *base
	t.Scheme = "https"
	t.Fragment = ""
	t.RawQuery = upstreamQuery
	p := strings.TrimSuffix(t.Path, "/")
	if p == "" {
		t.Path = aapJobTemplatesPath
	} else {
		t.Path = p + aapJobTemplatesPath
	}
	return t.String(), nil
}
