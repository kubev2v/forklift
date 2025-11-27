package dynamic

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// VersionResponse represents the version information from the external provider service.
type VersionResponse struct {
	Version struct {
		Major    string `json:"major"`
		Minor    string `json:"minor"`
		Build    string `json:"build"`
		Revision string `json:"revision"`
	} `json:"version"`
	Product *struct {
		Name   string `json:"name"`
		Vendor string `json:"vendor"`
	} `json:"product,omitempty"`
}

// Test performs a health check against the external provider service.
// It returns the HTTP status code and any error encountered.
// For dynamic providers, the service may not be available yet during initial
// validation, so we return success if the ServiceURL is not yet configured.
func (r *Collector) Test() (int, error) {
	// Test connection to external service
	if r.config == nil {
		return 0, fmt.Errorf("no config found")
	}

	// If ServiceURL is not yet set, the DynamicProviderServer hasn't been created yet.
	// Return success - the actual health check will happen once the service is deployed.
	if r.config.ServiceURL == "" {
		log.V(3).Info("ServiceURL not yet configured, skipping health check",
			"provider", r.provider.Name)
		return http.StatusOK, nil
	}

	healthURL := r.config.ServiceURL
	if r.config.HealthCheck.Path != "" {
		healthURL += r.config.HealthCheck.Path
	}

	resp, err := http.Get(healthURL)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return resp.StatusCode, fmt.Errorf("health check failed: %d", resp.StatusCode)
	}

	return resp.StatusCode, nil
}

// Version fetches version information from the external provider service.
// It calls the /version endpoint and returns major, minor, build, and revision.
// Version information is optional and this method will not fail if unavailable.
func (r *Collector) Version() (major, minor, build, revision string, err error) {
	if r.config == nil {
		return
	}

	// Call external service /version endpoint
	versionURL := r.config.ServiceURL + "/version"

	log.V(3).Info("Fetching version from external service",
		"provider", r.provider.Name,
		"url", versionURL)

	resp, err := http.Get(versionURL)
	if err != nil {
		log.V(3).Info("Failed to fetch version from external service",
			"url", versionURL,
			"error", err)
		// Version is optional - don't fail
		err = nil
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.V(3).Info("External service returned non-200 for version",
			"status", resp.StatusCode)
		// Version is optional - don't fail
		return
	}

	var versionResp VersionResponse
	err = json.NewDecoder(resp.Body).Decode(&versionResp)
	if err != nil {
		log.V(3).Info("Failed to decode version response",
			"error", err)
		// Don't fail on decode error
		err = nil
		return
	}

	major = versionResp.Version.Major
	minor = versionResp.Version.Minor
	build = versionResp.Version.Build
	revision = versionResp.Version.Revision

	log.V(2).Info("External service version retrieved",
		"provider", r.provider.Name,
		"version", fmt.Sprintf("%s.%s.%s-%s", major, minor, build, revision))

	return
}

// Follow is a placeholder method for compatibility with the collector interface.
// Dynamic providers do not support the Follow operation used by static providers
// for traversing object references in the source inventory.
func (r *Collector) Follow(moRef interface{}, p []string, dst interface{}) error {
	return fmt.Errorf("not implemented for dynamic providers")
}
