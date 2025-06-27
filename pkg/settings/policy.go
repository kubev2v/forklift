package settings

import (
	"errors"
	"os"
)

// Environment variables.
const (
	PolicyAgentURL            = "POLICY_AGENT_URL"
	PolicyAgentCA             = "POLICY_AGENT_CA"
	PolicyAgentWorkerLimit    = "POLICY_AGENT_WORKER_LIMIT"
	PolicyAgentSearchInterval = "POLICY_AGENT_SEARCH_INTERVAL"
)

// Policy agent settings.
type PolicyAgent struct {
	// URL.
	URL string
	// TLS
	TLS struct {
		// CA path
		CA string
	}
	// Search interval (seconds).
	SearchInterval int
	// Limits.
	Limit struct {
		// Number of workers.
		Worker int
	}
}

// Load settings.
func (r *PolicyAgent) Load() (err error) {
	if s, found := os.LookupEnv(PolicyAgentURL); found {
		r.URL = s
	}
	// TLS
	if s, found := os.LookupEnv(PolicyAgentCA); found {
		r.TLS.CA = s
	} else if _, err := os.Stat(ServiceCAFile); !errors.Is(err, os.ErrNotExist) {
		r.TLS.CA = ServiceCAFile
	}
	r.Limit.Worker, err = getPositiveEnvLimit(PolicyAgentWorkerLimit, 10)
	if err != nil {
		return err
	}
	r.SearchInterval, err = getPositiveEnvLimit(PolicyAgentSearchInterval, 600)
	if err != nil {
		return err
	}

	return
}

// Enabled.
func (r *PolicyAgent) Enabled() bool {
	return r.URL != ""
}
