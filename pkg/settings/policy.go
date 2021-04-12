package settings

import (
	"os"
)

//
// Environment variables.
const (
	PolicyAgentURL            = "POLICY_AGENT_URL"
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
	// CA path
	CA string
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
	if s, found := os.LookupEnv(PolicyAgentCA); found {
		r.CA = s
	} else {
		r.CA = ServiceCAFile
	}
	r.Limit.Worker, err = getEnvLimit(PolicyAgentWorkerLimit, 25)
	if err != nil {
		return err
	}
	r.Limit.Backlog, err = getEnvLimit(PolicyAgentBacklogLimit, 250)
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
