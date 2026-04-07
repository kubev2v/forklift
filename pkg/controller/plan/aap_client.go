package plan

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// AAP controller API job status values (subset used for polling).
const (
	aapJobStatusSuccessful = "successful"
	aapJobStatusFailed     = "failed"
	aapJobStatusError      = "error"
	aapJobStatusCanceled   = "canceled"
	aapJobStatusPending    = "pending"
	aapJobStatusWaiting    = "waiting"
	aapJobStatusRunning    = "running"
)

// AAPClient handles communication with Ansible Automation Platform API
type AAPClient struct {
	// AAP instance URL
	URL string
	// AAP API token
	Token string
	// HTTP client for making requests
	client *http.Client
}

// NewAAPClient creates a new AAP API client
func NewAAPClient(url, token string) *AAPClient {
	return &AAPClient{
		URL:   url,
		Token: token,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// LaunchJobResponse represents the response from AAP job launch
type LaunchJobResponse struct {
	ID     int    `json:"id"`
	Status string `json:"status"`
}

// JobStatusResponse represents the response from AAP job status query
type JobStatusResponse struct {
	ID       int    `json:"id"`
	Status   string `json:"status"`
	Finished string `json:"finished"`
	Failed   bool   `json:"failed"`
}

// LaunchJob launches an AAP job template with the given extra variables
func (c *AAPClient) LaunchJob(ctx context.Context, jobTemplateID int, extraVars map[string]string) (int, error) {
	url := fmt.Sprintf("%s/api/controller/v2/job_templates/%d/launch/", c.URL, jobTemplateID)

	reqBody := map[string]interface{}{
		"extra_vars": extraVars,
	}
	bodyJSON, err := json.Marshal(reqBody)
	if err != nil {
		return 0, liberr.Wrap(err, "failed to marshal request body")
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(bodyJSON))
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

// GetJobStatus retrieves the current status of an AAP job
func (c *AAPClient) GetJobStatus(ctx context.Context, jobID int) (*JobStatusResponse, error) {
	url := fmt.Sprintf("%s/api/controller/v2/jobs/%d/", c.URL, jobID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
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

// WaitForJobCompletion polls the AAP job status until it completes, fails, or hits a timeout.
// If unlimited is true, timeout is ignored and polling runs until a terminal status or ctx is cancelled.
func (c *AAPClient) WaitForJobCompletion(ctx context.Context, jobID int, timeout time.Duration, unlimited bool) error {
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

// jobPollTick returns (true, nil) on success, (true, err) on terminal failure, (false, nil) to keep polling.
func (c *AAPClient) jobPollTick(ctx context.Context, jobID int) (done bool, err error) {
	status, err := c.GetJobStatus(ctx, jobID)
	if err != nil {
		return false, liberr.Wrap(err, fmt.Sprintf("checking AAP job %d status", jobID))
	}

	log.Info("AAP job status", "jobID", jobID, "status", status.Status)

	switch status.Status {
	case aapJobStatusSuccessful:
		return true, nil
	case aapJobStatusFailed, aapJobStatusError, aapJobStatusCanceled:
		return true, fmt.Errorf("AAP job %d failed with status: %s", jobID, status.Status)
	case aapJobStatusPending, aapJobStatusWaiting, aapJobStatusRunning:
		return false, nil
	default:
		log.Info("Unknown AAP job status, continuing to poll", "status", status.Status)
		return false, nil
	}
}

// GetAAPTokenFromSecret reads the AAP API token from a Kubernetes Secret referenced by ref.
// The Secret is always loaded from defaultNamespace (the migration plan namespace).
// If ref.Namespace is set, it must equal defaultNamespace.
func GetAAPTokenFromSecret(ctx context.Context, k8sClient client.Client, defaultNamespace string, ref *core.ObjectReference) (string, error) {
	if ref == nil || strings.TrimSpace(ref.Name) == "" {
		return "", fmt.Errorf("tokenSecret must be set with a non-empty name")
	}
	if strings.TrimSpace(ref.Namespace) != "" && ref.Namespace != defaultNamespace {
		return "", fmt.Errorf(
			"tokenSecret namespace %q must be empty or match the plan namespace %q",
			ref.Namespace, defaultNamespace)
	}
	ns := defaultNamespace
	secret := &core.Secret{}
	err := k8sClient.Get(
		ctx,
		types.NamespacedName{
			Namespace: ns,
			Name:      ref.Name,
		},
		secret,
	)
	if err != nil {
		return "", liberr.Wrap(err, fmt.Sprintf("failed to get secret %s/%s", ns, ref.Name))
	}

	tokenBytes, ok := secret.Data["token"]
	if !ok {
		return "", fmt.Errorf("secret %s/%s does not contain 'token' key", ns, ref.Name)
	}

	return string(tokenBytes), nil
}
