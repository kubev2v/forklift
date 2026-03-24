package plan

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"sigs.k8s.io/controller-runtime/pkg/client"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
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

// WaitForJobCompletion polls the AAP job status until it completes or times out
func (c *AAPClient) WaitForJobCompletion(ctx context.Context, jobID int, timeout time.Duration) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	timeoutTimer := time.NewTimer(timeout)
	defer timeoutTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeoutTimer.C:
			return fmt.Errorf("AAP job %d timed out after %s", jobID, timeout)
		case <-ticker.C:
			status, err := c.GetJobStatus(ctx, jobID)
			if err != nil {
				Log.Error(err, "Failed to check AAP job status", "jobID", jobID)
				// Continue polling even if there's an error
				continue
			}

			Log.Info("AAP job status", "jobID", jobID, "status", status.Status)

			switch status.Status {
			case "successful":
				return nil
			case "failed", "error", "canceled":
				return fmt.Errorf("AAP job %d failed with status: %s", jobID, status.Status)
			case "pending", "waiting", "running":
				// Continue polling
				continue
			default:
				Log.Info("Unknown AAP job status, continuing to poll", "status", status.Status)
			}
		}
	}
}

// GetAAPToken retrieves the AAP token from a Kubernetes Secret
func GetAAPToken(ctx context.Context, k8sClient client.Client, namespace, secretName string) (string, error) {
	secret := &core.Secret{}
	err := k8sClient.Get(
		ctx,
		types.NamespacedName{
			Namespace: namespace,
			Name:      secretName,
		},
		secret,
	)
	if err != nil {
		return "", liberr.Wrap(err, fmt.Sprintf("failed to get secret %s/%s", namespace, secretName))
	}

	tokenBytes, ok := secret.Data["token"]
	if !ok {
		return "", fmt.Errorf("secret %s/%s does not contain 'token' key", namespace, secretName)
	}

	return string(tokenBytes), nil
}
