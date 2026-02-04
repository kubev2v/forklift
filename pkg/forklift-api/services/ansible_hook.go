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

package services

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/forklift-api/ansible"
	core "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// HooksFromAnsiblePath is the path for the "create hook from Ansible playbook" endpoint.
	HooksFromAnsiblePath = "/api/v1/hooks/from-ansible"

	// AnsibleConfigMapName is the ConfigMap that holds Ansible base URL.
	AnsibleConfigMapName = "ansible-config"
	// AnsibleConfigKeyBaseURL is the ConfigMap key for the Ansible API base URL.
	AnsibleConfigKeyBaseURL = "ansibleBaseUrl"
	// AnsibleConfigKeyTokenURL is the ConfigMap key for the OAuth2 token endpoint (optional). If set with client_id/client_secret in Secret, a token is obtained via client credentials grant.
	AnsibleConfigKeyTokenURL = "ansibleTokenUrl"
	// AnsibleConfigKeyApiPath is the ConfigMap key for the Ansible API path prefix (optional). Use "/api/v2" for AWX/AAP; default is "/api/v1".
	AnsibleConfigKeyApiPath = "ansibleApiPath"

	// AnsibleSecretName is the Secret that holds optional Ansible API token or OAuth2 credentials.
	AnsibleSecretName = "ansible-credentials"
	// AnsibleSecretKeyToken is the Secret key for a pre-obtained Bearer token (used when OAuth2 token URL is not configured).
	AnsibleSecretKeyToken = "token"
	// AnsibleSecretKeyClientID is the Secret key for OAuth2 client_id (used with ansibleTokenUrl).
	AnsibleSecretKeyClientID = "client_id"
	// AnsibleSecretKeyClientSecret is the Secret key for OAuth2 client_secret (used with ansibleTokenUrl).
	AnsibleSecretKeyClientSecret = "client_secret"

	// DefaultHookImage is the default hook-runner image.
	DefaultHookImage = "quay.io/kubev2v/hook-runner:latest"

	// MaxHookNameLength is the max length for a Kubernetes resource name.
	MaxHookNameLength = 63
)

var (
	// dnsLabelRe matches valid DNS-1123 label characters (lowercase alphanumeric, hyphen).
	dnsLabelRe = regexp.MustCompile(`[^a-z0-9-]+`)
)

// FromAnsibleRequest is the JSON body for creating a hook from Ansible.
type FromAnsibleRequest struct {
	Namespace     string `json:"namespace"`
	PlanName      string `json:"planName"`
	VMID          string `json:"vmId"`
	Step          string `json:"step"` // PreHook or PostHook
	AnsibleUserID string `json:"ansibleUserId"`
	PlaybookID    string `json:"playbookId,omitempty"` // optional
}

// FromAnsibleResponse is the JSON response with the created Hook ref.
type FromAnsibleResponse struct {
	HookNamespace string `json:"hookNamespace"`
	HookName      string `json:"hookName"`
}

// oauth2TokenResponse is the standard OAuth2 token endpoint response (RFC 6749).
type oauth2TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

// getOAuth2Token obtains an access token via OAuth2 client credentials grant.
func getOAuth2Token(ctx context.Context, tokenURL, clientID, clientSecret string) (string, error) {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("OAuth2 token endpoint returned %d", resp.StatusCode)
	}
	var tok oauth2TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		return "", err
	}
	if tok.AccessToken == "" {
		return "", fmt.Errorf("OAuth2 response missing access_token")
	}
	return tok.AccessToken, nil
}

func serveHooksFromAnsible(w http.ResponseWriter, r *http.Request, k8sClient crclient.Client) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req FromAnsibleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.Namespace == "" || req.PlanName == "" || req.VMID == "" || req.Step == "" || req.AnsibleUserID == "" {
		http.Error(w, "Missing required field: namespace, planName, vmId, step, ansibleUserId", http.StatusBadRequest)
		return
	}

	if req.Step != "PreHook" && req.Step != "PostHook" {
		http.Error(w, "step must be PreHook or PostHook", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Read Ansible base URL from ConfigMap.
	cm := &core.ConfigMap{}
	err := k8sClient.Get(ctx, crclient.ObjectKey{Namespace: req.Namespace, Name: AnsibleConfigMapName}, cm)
	if err != nil {
		if k8serr.IsNotFound(err) {
			http.Error(w, "Ansible config not found: ConfigMap "+AnsibleConfigMapName+" in namespace "+req.Namespace, http.StatusPreconditionFailed)
			return
		}
		log.Error(err, "Failed to get Ansible ConfigMap")
		http.Error(w, "Failed to read Ansible config", http.StatusInternalServerError)
		return
	}

	baseURL, ok := cm.Data[AnsibleConfigKeyBaseURL]
	if !ok || baseURL == "" {
		http.Error(w, "ConfigMap "+AnsibleConfigMapName+" missing key "+AnsibleConfigKeyBaseURL, http.StatusPreconditionFailed)
		return
	}

	baseURL = strings.TrimRight(baseURL, "/")

	apiPath := strings.TrimSpace(cm.Data[AnsibleConfigKeyApiPath])
	if apiPath == "" {
		apiPath = "/api/v1"
	}

	// Optional: read token or OAuth2 credentials from Secret.
	token := ""
	secret := &core.Secret{}
	err = k8sClient.Get(ctx, crclient.ObjectKey{Namespace: req.Namespace, Name: AnsibleSecretName}, secret)
	if err == nil && secret.Data != nil {
		tokenURL := strings.TrimSpace(cm.Data[AnsibleConfigKeyTokenURL])
		clientID := ""
		clientSecret := ""
		if b, ok := secret.Data[AnsibleSecretKeyClientID]; ok {
			clientID = string(b)
		}
		if b, ok := secret.Data[AnsibleSecretKeyClientSecret]; ok {
			clientSecret = string(b)
		}
		if tokenURL != "" && clientID != "" && clientSecret != "" {
			// OAuth2 client credentials: obtain token from token URL.
			token, err = getOAuth2Token(ctx, tokenURL, clientID, clientSecret)
			if err != nil {
				log.Error(err, "Failed to obtain OAuth2 token for Ansible")
				http.Error(w, "Failed to obtain Ansible credentials: "+err.Error(), http.StatusPreconditionFailed)
				return
			}
		} else if b, ok := secret.Data[AnsibleSecretKeyToken]; ok {
			token = string(b)
		}
	}
	// Ignore NotFound; token is optional.

	// Fetch playbook from Ansible.
	log.Info("Fetching playbook from Ansible", "userId", req.AnsibleUserID, "step", req.Step, "plan", req.PlanName, "vmId", req.VMID)
	ac := ansible.New(baseURL, apiPath)
	stepParam := req.Step
	if req.PlaybookID != "" {
		stepParam = req.PlaybookID
	}
	playbookYAML, err := ac.GetPlaybook(ctx, req.AnsibleUserID, stepParam, token)
	if err != nil {
		log.Error(err, "Failed to fetch playbook from Ansible", "userId", req.AnsibleUserID, "step", req.Step)
		http.Error(w, "Failed to fetch playbook from Ansible: "+err.Error(), http.StatusBadRequest)
		return
	}

	playbookBase64 := base64.StdEncoding.EncodeToString(playbookYAML)

	// Deterministic Hook name: planName-vmId-step-ansible (sanitized).
	hookName := sanitizeHookName(req.PlanName + "-" + req.VMID + "-" + req.Step + "-ansible")

	hook := &api.Hook{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: req.Namespace,
			Name:      hookName,
		},
		Spec: api.HookSpec{
			Image:    DefaultHookImage,
			Playbook: playbookBase64,
		},
	}

	err = k8sClient.Create(ctx, hook)
	if err != nil {
		if k8serr.IsAlreadyExists(err) {
			// Idempotent: return existing Hook ref.
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(FromAnsibleResponse{
				HookNamespace: req.Namespace,
				HookName:      hookName,
			})
			return
		}
		log.Error(err, "Failed to create Hook CR")
		http.Error(w, "Failed to create Hook: "+err.Error(), http.StatusInternalServerError)
		return
	}

	log.Info("Created Hook from Ansible", "namespace", req.Namespace, "name", hookName, "step", req.Step)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(FromAnsibleResponse{
		HookNamespace: req.Namespace,
		HookName:      hookName,
	})
}

// sanitizeHookName returns a valid DNS-1123 label (lowercase, alphanumeric, hyphen), max 63 chars.
func sanitizeHookName(s string) string {
	s = strings.ToLower(s)
	s = dnsLabelRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	if len(s) > MaxHookNameLength {
		s = s[:MaxHookNameLength]
		s = strings.TrimRight(s, "-")
	}
	if s == "" {
		s = "hook-from-ansible"
	}
	return s
}
