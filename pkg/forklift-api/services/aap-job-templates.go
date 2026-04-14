package services

import (
	"context"
	"fmt"
	"io"
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
)

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
	if aapBase.Scheme != "http" && aapBase.Scheme != "https" {
		http.Error(resp, "url scheme must be http or https", http.StatusBadRequest)
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

	base := strings.TrimRight(aapBase.String(), "/")
	target := base + "/api/controller/v2/job_templates/"
	if upstreamQuery != "" {
		target += "?" + upstreamQuery
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

	hc := &http.Client{Timeout: aapProxyTimeout}
	out, err := hc.Do(httpReq)
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
