package dynamic

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/kubev2v/forklift/pkg/lib/logging"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var log = logging.WithName("dynamic-provider-registry")

// Global registry instance
var Registry = &ProviderRegistry{
	configs: make(map[string]*ProviderConfig),
}

type ProviderRegistry struct {
	mu      sync.RWMutex
	configs map[string]*ProviderConfig
	client  client.Client
}

type ProviderConfig struct {
	ProviderType    string
	DisplayName     string
	Description     string
	ServiceURL      string
	Routes          []RouteConfig
	HealthCheck     HealthCheckConfig
	RefreshInterval int32 // Seconds, 0 = no polling
}

type RouteConfig struct {
	Path    string
	Methods []string
}

type HealthCheckConfig struct {
	Path            string
	IntervalSeconds int
}

func (r *ProviderRegistry) Initialize(k8sClient client.Client) {
	r.client = k8sClient
}

// RegisterType registers a new dynamic provider type (from DynamicProvider CR)
func (r *ProviderRegistry) RegisterType(providerType, displayName, description string, refreshInterval int32) {
	r.mu.Lock()
	defer r.mu.Unlock()

	providerConfig := &ProviderConfig{
		ProviderType:    providerType,
		DisplayName:     displayName,
		Description:     description,
		RefreshInterval: refreshInterval,
		HealthCheck: HealthCheckConfig{
			Path:            "/test_connection",
			IntervalSeconds: 30,
		},
	}

	r.configs[providerType] = providerConfig

	log.Info("Registered dynamic provider type",
		"type", providerType,
		"refreshInterval", refreshInterval)
}

// UpdateServiceURL updates the service URL for a provider (from DynamicProviderServer)
func (r *ProviderRegistry) UpdateServiceURL(providerType, serviceURL string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	config, exists := r.configs[providerType]
	if !exists {
		return fmt.Errorf("provider type %s not registered", providerType)
	}

	// Test connectivity
	healthURL := serviceURL + "/test_connection"
	resp, err := http.Get(healthURL)
	if err != nil || resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed for %s: %v", providerType, err)
	}
	resp.Body.Close()

	config.ServiceURL = serviceURL

	log.Info("Updated service URL for dynamic provider",
		"type", providerType,
		"url", serviceURL)

	return nil
}

// Unregister a dynamic provider
func (r *ProviderRegistry) Unregister(providerType string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.configs, providerType)
	log.Info("Unregistered dynamic provider", "type", providerType)
}

// Get provider config
func (r *ProviderRegistry) Get(providerType string) (*ProviderConfig, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	config, exists := r.configs[providerType]
	return config, exists
}

// Check if provider is dynamic
func (r *ProviderRegistry) IsDynamic(providerType string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, exists := r.configs[providerType]
	return exists
}

// List all dynamic providers
func (r *ProviderRegistry) List() []*ProviderConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()

	configs := make([]*ProviderConfig, 0, len(r.configs))
	for _, config := range r.configs {
		configs = append(configs, config)
	}
	return configs
}

// GetTypes returns a list of all registered dynamic provider type names
func (r *ProviderRegistry) GetTypes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	types := make([]string, 0, len(r.configs))
	for providerType := range r.configs {
		types = append(types, providerType)
	}
	return types
}
