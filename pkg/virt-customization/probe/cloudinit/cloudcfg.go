package cloudinit

import (
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v2"
)

// CloudCfgResult holds the fields extracted from cloud.cfg and cloud.cfg.d.
type CloudCfgResult struct {
	DatasourceList        []string
	NetworkConfigDisabled bool
}

// cloudCfgDoc is the partial YAML structure we extract from cloud.cfg.
type cloudCfgDoc struct {
	DatasourceList []string    `yaml:"datasource_list"`
	Network        interface{} `yaml:"network"`
}

// ParseCloudCfg parses concatenated cloud.cfg + cloud.cfg.d/*.cfg YAML
// documents and extracts the fields needed by migration plugins.
// Later documents override earlier ones (drop-in semantics).
func ParseCloudCfg(section string) (CloudCfgResult, error) {
	var result CloudCfgResult
	var errs []error

	for i, doc := range splitYAMLDocuments(section) {
		if strings.TrimSpace(doc) == "" {
			continue
		}
		var cfg cloudCfgDoc
		if err := yaml.Unmarshal([]byte(doc), &cfg); err != nil {
			errs = append(errs, fmt.Errorf("parsing cloud.cfg document %d: %w", i, err))
			continue
		}

		if len(cfg.DatasourceList) > 0 {
			result.DatasourceList = cfg.DatasourceList
		}

		if cfg.Network != nil {
			result.NetworkConfigDisabled = isNetworkDisabled(cfg.Network)
		}
	}

	return result, errors.Join(errs...)
}

// isNetworkDisabled checks if the network key is {config: disabled}.
// cloud-init supports: network: {config: disabled}
func isNetworkDisabled(network interface{}) bool {
	m, ok := network.(map[interface{}]interface{})
	if !ok {
		return false
	}
	val, ok := m["config"]
	if !ok {
		return false
	}
	s, ok := val.(string)
	return ok && s == "disabled"
}

// ParseDatasourceFile reads the /var/lib/cloud/instance/datasource file
// which contains a line like "DataSourceNoCloud [seed=/dev/sr0]".
// Returns the datasource name (e.g., "NoCloud").
func ParseDatasourceFile(content string) string {
	line := strings.TrimSpace(content)
	if line == "" {
		return ""
	}
	// Format: "DataSource<Name> [<details>]" or "DataSource<Name>"
	name := line
	if idx := strings.IndexByte(name, ' '); idx > 0 {
		name = name[:idx]
	}
	if idx := strings.IndexByte(name, '['); idx > 0 {
		name = name[:idx]
	}
	name = strings.TrimPrefix(name, "DataSource")
	return name
}

// splitYAMLDocuments splits concatenated YAML docs on "---" separators.
func splitYAMLDocuments(input string) []string {
	var docs []string
	var current strings.Builder

	for _, line := range strings.Split(input, "\n") {
		if strings.TrimSpace(line) == "---" {
			if current.Len() > 0 {
				docs = append(docs, current.String())
				current.Reset()
			}
			continue
		}
		current.WriteString(line)
		current.WriteByte('\n')
	}
	if current.Len() > 0 {
		docs = append(docs, current.String())
	}
	return docs
}
