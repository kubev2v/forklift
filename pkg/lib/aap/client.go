package aap

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/lib/logging"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/settings"
)

// AAP controller API job status values (subset used for polling).
const (
	JobStatusSuccessful = "successful"
	JobStatusFailed     = "failed"
	JobStatusError      = "error"
	JobStatusCanceled   = "canceled"
	JobStatusPending    = "pending"
	JobStatusWaiting    = "waiting"
	JobStatusRunning    = "running"
)

// aapControllerV2APIPrefix is the default path prefix for the AAP/AWX controller API when
// automatic discovery (GET /api) fails or is unavailable.
const aapControllerV2APIPrefix = "/api/controller/v2"

const (
	defaultJobTemplatePageSize = 20
	maxJobTemplatePageSize     = 100
	// defaultMaxJobTemplatesList caps how many templates ListAllJobTemplates will aggregate from AAP.
	defaultMaxJobTemplatesList = 500
	listAllInternalPageSize    = 100
)

var log = logging.WithName("aap")

// Client handles communication with Ansible Automation Platform API.
type Client struct {
	URL    string
	Token  string
	client *http.Client

	// staticPathPrefix, if set (e.g. in tests via NewClient option), is copied into
	// apiPathPrefix once and GET /api is never called. Empty in production.
	staticPathPrefix string

	// apiPathPrefix is the path segment after the host, e.g.
	// "/api/controller/v2" — no trailing slash. Empty until resolvePathPrefix runs.
	apiPathPrefix string
}

// ClientOption configures [NewClient].
type ClientOption func(*Client)

// LaunchJobResponse represents the response from AAP job launch.
type LaunchJobResponse struct {
	ID     int    `json:"id"`
	Status string `json:"status"`
}

// JobStatusResponse represents the response from AAP job status query.
type JobStatusResponse struct {
	ID       int    `json:"id"`
	Status   string `json:"status"`
	Finished string `json:"finished"`
	Failed   bool   `json:"failed"`
}

// JobTemplateSummary is a subset of fields returned by the job_templates list API.
type JobTemplateSummary struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// JobTemplateListResponse is the paginated list returned by the controller API.
type JobTemplateListResponse struct {
	Count    int                  `json:"count"`
	Next     string               `json:"next"`
	Previous string               `json:"previous"`
	Results  []JobTemplateSummary `json:"results"`
}

// WithPathPrefix sets a fixed API path prefix and skips GET /api discovery. For tests; production
// should omit this option so the first HTTP call can discover the prefix from GET {baseURL}/api.
func WithPathPrefix(prefix string) ClientOption {
	return func(c *Client) {
		c.staticPathPrefix = prefix
	}
}

// resolvePathPrefix sets apiPathPrefix once: optional static ([WithPathPrefix]), else GET {baseURL}/api
// (Ansible/AWX root doc) and current_version, else the package default aapControllerV2APIPrefix.
func (c *Client) resolvePathPrefix(ctx context.Context) {
	if c.apiPathPrefix != "" {
		return
	}
	if c.staticPathPrefix != "" {
		c.apiPathPrefix = normalizePathPrefix(c.staticPathPrefix)
		if c.apiPathPrefix == "" {
			c.apiPathPrefix = aapControllerV2APIPrefix
		}
		return
	}

	prefix, err := c.fetchPathPrefixFromAPIDoc(ctx)
	chosen := aapControllerV2APIPrefix
	if err == nil && prefix != "" {
		chosen = prefix
	} else {
		if err != nil {
			log.Error(err, "AAP/AWX API prefix: using default", "default", aapControllerV2APIPrefix)
		}
	}

	c.apiPathPrefix = chosen
}

func (c *Client) fetchPathPrefixFromAPIDoc(ctx context.Context) (string, error) {
	u := strings.TrimRight(c.URL, "/") + "/api/"
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return "", liberr.Wrap(err, "AAP/AWX: create GET /api request")
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return "", liberr.Wrap(err, "AAP/AWX: GET /api")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GET /api: status %d", resp.StatusCode)
	}
	var body struct {
		CurrentVersion string `json:"current_version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", liberr.Wrap(err, "AAP/AWX: decode /api")
	}
	return normalizePathPrefix(body.CurrentVersion), nil
}

// normalizePathPrefix turns AAP/AWX current_version (path with optional trailing /, or rarely a full
// URL) into a path with no trailing slash, e.g. "/api/v2".
func normalizePathPrefix(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	if u, err := url.Parse(v); err == nil && u.Path != "" {
		v = u.Path
	} else if !strings.HasPrefix(v, "/") {
		v = "/" + v
	}
	v = strings.TrimRight(v, "/")
	if v == "" {
		return ""
	}
	if !strings.HasPrefix(v, "/") {
		v = "/" + v
	}
	return v
}

func (c *Client) resourceURL(ctx context.Context, path string) (string, error) {
	c.resolvePathPrefix(ctx)
	pref := c.apiPathPrefix
	if pref == "" {
		pref = aapControllerV2APIPrefix
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return strings.TrimRight(c.URL, "/") + pref + path, nil
}

// NewClient creates an AAP API client with the given HTTP timeout.
func NewClient(aapURL, token string, httpTimeout time.Duration, opts ...ClientOption) *Client {
	if httpTimeout <= 0 {
		httpTimeout = 30 * time.Second
	}
	c := &Client{
		URL:   aapURL,
		Token: token,
		client: &http.Client{
			Timeout: httpTimeout,
		},
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// LaunchJob launches an AAP job template with the given extra variables.
func (c *Client) LaunchJob(ctx context.Context, jobTemplateID int, extraVars map[string]string) (int, error) {
	u, err := c.resourceURL(ctx, fmt.Sprintf("/job_templates/%d/launch/", jobTemplateID))
	if err != nil {
		return 0, err
	}

	reqBody := map[string]interface{}{
		"extra_vars": extraVars,
	}
	bodyJSON, err := json.Marshal(reqBody)
	if err != nil {
		return 0, liberr.Wrap(err, "failed to marshal request body")
	}

	req, err := http.NewRequestWithContext(ctx, "POST", u, bytes.NewBuffer(bodyJSON))
	if err != nil {
		return 0, liberr.Wrap(err, "failed to create request")
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.Token))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return 0, liberr.Wrap(err, "failed to launch AAP job")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("AAP returned status %d: %s", resp.StatusCode, string(body))
	}

	var result LaunchJobResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, liberr.Wrap(err, "failed to decode response")
	}

	return result.ID, nil
}

// GetJobStatus retrieves the current status of an AAP job.
func (c *Client) GetJobStatus(ctx context.Context, jobID int) (*JobStatusResponse, error) {
	u, err := c.resourceURL(ctx, fmt.Sprintf("/jobs/%d/", jobID))
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, liberr.Wrap(err, "failed to create request")
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.Token))

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, liberr.Wrap(err, "failed to get AAP job status")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("AAP returned status %d: %s", resp.StatusCode, string(body))
	}

	var result JobStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, liberr.Wrap(err, "failed to decode response")
	}

	return &result, nil
}

// ListJobTemplates returns a page of job templates from the AAP controller API.
func (c *Client) ListJobTemplates(ctx context.Context, page, pageSize int) (*JobTemplateListResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = defaultJobTemplatePageSize
	}
	if pageSize > maxJobTemplatePageSize {
		pageSize = maxJobTemplatePageSize
	}
	uStr, err := c.resourceURL(ctx, "/job_templates/")
	if err != nil {
		return nil, err
	}
	u, err := url.Parse(uStr)
	if err != nil {
		return nil, liberr.Wrap(err, "invalid AAP job_templates URL")
	}
	q := u.Query()
	q.Set("page", strconv.Itoa(page))
	q.Set("page_size", strconv.Itoa(pageSize))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, liberr.Wrap(err, "failed to create request")
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.Token))

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, liberr.Wrap(err, "failed to list AAP job templates")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("AAP returned status %d: %s", resp.StatusCode, string(body))
	}
	var out JobTemplateListResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, liberr.Wrap(err, "failed to decode job template list")
	}
	return &out, nil
}

// ListAllJobTemplates walks AAP pagination and returns a flat list, stopping when there are no more
// pages or when maxJobs templates have been collected. If maxJobs <= 0, defaultMaxJobTemplatesList is used.
func (c *Client) ListAllJobTemplates(ctx context.Context, maxJobs int) ([]JobTemplateSummary, error) {
	if maxJobs <= 0 {
		maxJobs = defaultMaxJobTemplatesList
	}
	var all []JobTemplateSummary
	page := 1
	for len(all) < maxJobs {
		resp, err := c.ListJobTemplates(ctx, page, listAllInternalPageSize)
		if err != nil {
			return nil, err
		}
		for _, t := range resp.Results {
			if len(all) >= maxJobs {
				break
			}
			all = append(all, t)
		}
		if resp.Next == "" || len(resp.Results) == 0 {
			break
		}
		page++
	}
	return all, nil
}

// WaitForJobCompletion polls the AAP job status until it completes, fails, or hits a timeout.
func (c *Client) WaitForJobCompletion(ctx context.Context, jobID int, timeout time.Duration, unlimited bool) error {
	if !unlimited {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	done, err := c.jobPollTick(ctx, jobID)
	if err != nil {
		return err
	}
	if done {
		return nil
	}

	for {
		select {
		case <-ctx.Done():
			if !unlimited && errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return fmt.Errorf("AAP job %d timed out after %s", jobID, timeout)
			}
			return ctx.Err()
		case <-ticker.C:
		}
		done, err := c.jobPollTick(ctx, jobID)
		if err != nil {
			return err
		}
		if done {
			return nil
		}
	}
}

func (c *Client) jobPollTick(ctx context.Context, jobID int) (done bool, err error) {
	status, err := c.GetJobStatus(ctx, jobID)
	if err != nil {
		return false, liberr.Wrap(err, fmt.Sprintf("checking AAP job %d status", jobID))
	}

	log.Info("AAP job status", "jobID", jobID, "status", status.Status)

	switch status.Status {
	case JobStatusSuccessful:
		return true, nil
	case JobStatusFailed, JobStatusError, JobStatusCanceled:
		return true, fmt.Errorf("AAP job %d failed with status: %s", jobID, status.Status)
	case JobStatusPending, JobStatusWaiting, JobStatusRunning:
		return false, nil
	default:
		log.Info("Unknown AAP job status, continuing to poll", "status", status.Status)
		return false, nil
	}
}

// HookAAPRunnableFromMigrationSettings reports whether an AAP hook can run using per-hook
// connection (spec.aap.url + tokenSecret) or cluster defaults in settings.Settings.Migration.
// It delegates to [api.HookAAPRunnable]; that logic cannot call [settings] from the apis
// package (import cycle: v1beta1 <-> settings).
func HookAAPRunnableFromMigrationSettings(hook *api.Hook) bool {
	m := settings.Settings.Migration
	return api.HookAAPRunnable(hook, m.AAPURL, m.AAPTokenSecretName)
}
