/*
Copyright 2019 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package ansible

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	// DefaultTimeout for HTTP requests to Ansible.
	DefaultTimeout = 30 * time.Second
)

// Client calls an Ansible API to fetch playbook content.
type Client struct {
	baseURL    string
	apiPath    string // e.g. "/api/v1" or "/api/v2" for AWX/AAP
	httpClient *http.Client
}

// New returns an Ansible API client. baseURL should not include a trailing slash.
// apiPath is the API path prefix (e.g. "/api/v1" or "/api/v2" for AWX/AAP); if empty, "/api/v1" is used.
func New(baseURL, apiPath string) *Client {
	if apiPath == "" {
		apiPath = "/api/v1"
	}
	return &Client{
		baseURL: baseURL,
		apiPath: strings.TrimSuffix(apiPath, "/"),
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
		},
	}
}

// GetPlaybook fetches the playbook for the given user and step.
// Endpoint: GET {baseURL}{apiPath}/users/{userID}/playbook?step={step}
// If token is non-empty, adds Authorization: Bearer {token}.
// Returns the raw response body (playbook YAML or other format as returned by Ansible).
func (c *Client) GetPlaybook(ctx context.Context, userID, step, token string) ([]byte, error) {
	path := fmt.Sprintf("%s/users/%s/playbook", c.apiPath, url.PathEscape(userID))
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid Ansible base URL: %w", err)
	}
	u.Path = path
	if step != "" {
		u.RawQuery = url.Values{"step": {step}}.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Accept", "text/yaml, application/yaml, application/json, */*")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request to Ansible failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Ansible returned %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read Ansible response: %w", err)
	}
	return body, nil
}
