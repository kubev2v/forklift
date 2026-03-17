package storage

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

type SystemVersion struct {
	Id            int    `json:"id"`
	VersionString string `json:"versionString"`
	Name          string `json:"name"`
}

type SystemInfo struct {
	SystemVersion string `json:"systemVersion"`
	Model         string `json:"model"`
}

func getPrimera3ParSystemInfo(apiURL, username, password string, skipSSLVerify bool) (SystemInfo, error) {
	sessionKey, err := getSessionKey(apiURL, username, password, skipSSLVerify)
	if err != nil {
		return SystemInfo{}, err
	}

	fullURL, err := url.JoinPath(apiURL, "/api/v1/system")
	if err != nil {
		return SystemInfo{}, fmt.Errorf("error constructing URL: %w", err)
	}

	req, err := http.NewRequest(http.MethodGet, fullURL, nil)
	if err != nil {
		return SystemInfo{}, fmt.Errorf("error creating HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-HP3PAR-WSAPI-SessionKey", sessionKey)

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: skipSSLVerify},
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return SystemInfo{}, fmt.Errorf("error sending HTTP request: %w", err)
	}
	defer resp.Body.Close() // Ensure the body is closed after we're done.

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return SystemInfo{}, fmt.Errorf("error reading response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return SystemInfo{}, fmt.Errorf("API request failed with status %s and body: %s", resp.Status, string(body))
	}

	var systemInfo SystemInfo
	err = json.Unmarshal(body, &systemInfo)
	if err != nil {
		return SystemInfo{}, fmt.Errorf("error unmarshalling JSON: %w.  Response body was: %s", err, string(body))
	}
	return systemInfo, nil
}

func getSessionKey(hostname, username, password string, skipSSLVerify bool) (string, error) {
	url := fmt.Sprintf("%s/api/v1/credentials", hostname)

	requestBody := map[string]string{
		"user":     username,
		"password": password,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to encode JSON: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: skipSSLVerify},
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		var errorResp struct {
			Code int    `json:"code"`
			Desc string `json:"desc"`
		}

		if err := json.Unmarshal(bodyBytes, &errorResp); err == nil {
			return "", fmt.Errorf("authentication failed: %s (code %d)", errorResp.Desc, errorResp.Code)
		}
		return "", fmt.Errorf("authentication failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var response map[string]string
	if err := json.Unmarshal(bodyBytes, &response); err != nil {
		return "", fmt.Errorf("failed to parse session key response: %w", err)
	}

	if sessionKey, ok := response["key"]; ok {
		return sessionKey, nil
	}

	return "", fmt.Errorf("failed to retrieve session key, response: %s", string(bodyBytes))
}
