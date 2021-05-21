package settings

import (
	"os"
)

//
// Environment variables.
const (
	PolicyAgentURL            = "POLICY_AGENT_URL"
	PolicyTLSEnabled          = "POLICY_TLS_ENABLED"
	PolicyAgentCA             = "POLICY_AGENT_CA"
	PolicyAgentWorkerLimit    = "POLICY_AGENT_WORKER_LIMIT"
	PolicyAgentBacklogLimit   = "POLICY_AGENT_BACKLOG_LIMIT"
	PolicyAgentSearchInterval = "POLICY_AGENT_SEARCH_INTERVAL"
)

//
// Policy agent settings.
type PolicyAgent struct {
	// URL.
	URL string
	// TLS
	TLS struct {
		// Enabled.
		Enabled bool
		// CA path
		CA string
	}
	// Search interval (seconds).
	SearchInterval int
	// Limits.
	Limit struct {
		// Number of workers.
		Worker int
		// Backlog depth.
		Backlog int
	}
}

//
// Load settings.
func (r *PolicyAgent) Load() (err error) {
	if s, found := os.LookupEnv(PolicyAgentURL); found {
		r.URL = s
	}
	// TLS
	r.TLS.Enabled = getEnvBool(TLSEnabled, false)
	if s, found := os.LookupEnv(PolicyAgentCA); found {
		r.TLS.CA = s
	} else {
		r.TLS.CA = ServiceCAFile
	}
	r.Limit.Worker, err = getEnvLimit(PolicyAgentWorkerLimit, 10)
	if err != nil {
		return err
	}
	r.Limit.Backlog, err = getEnvLimit(PolicyAgentBacklogLimit, 10000)
	if err != nil {
		return err
	}
	r.SearchInterval, err = getEnvLimit(PolicyAgentSearchInterval, 600)
	if err != nil {
		return err
	}

	return
}

//
// Enabled.
func (r *PolicyAgent) Enabled() bool {
	return r.URL != ""
}
