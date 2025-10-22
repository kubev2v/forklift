package storage

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

type OntapSystem struct {
	Name    string `json:"name"`
	Version struct {
		Full string `json:"full"`
	}
}

func getOntapSystemInfo(apiURL, username, password string, skipSSLVerify bool) (OntapSystem, error) {
	fullURL, err := url.JoinPath(apiURL, "/api/cluster")
	if err != nil {
		return OntapSystem{}, fmt.Errorf("error constructing URL: %w", err)
	}

	req, err := http.NewRequest(http.MethodGet, fullURL, nil)
	if err != nil {
		return OntapSystem{}, fmt.Errorf("error creating HTTP request: %w", err)
	}

	req.SetBasicAuth(username, password)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: skipSSLVerify},
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return OntapSystem{}, fmt.Errorf("error sending HTTP request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return OntapSystem{}, fmt.Errorf("error reading response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return OntapSystem{}, fmt.Errorf("API request failed with status %s and body: %s", resp.Status, string(body))
	}

	fmt.Println(string(body))
	var systemInfo OntapSystem
	err = json.Unmarshal(body, &systemInfo)
	if err != nil {
		return OntapSystem{}, fmt.Errorf("error unmarshalling JSON: %w. Response body was: %s", err, string(body))
	}

	return systemInfo, nil
}
