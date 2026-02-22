package health

// JSONLogEntry represents a structured log entry from the forklift controller.
// The forklift controller outputs JSON logs with fields like:
// {"level":"info","ts":"2026-02-05 10:45:52","logger":"plan|zw4bt","msg":"Reconcile started.","plan":{"name":"my-plan","namespace":"demo"}}
type JSONLogEntry struct {
	Level     string            `json:"level"`
	Ts        string            `json:"ts"`
	Logger    string            `json:"logger"`
	Msg       string            `json:"msg"`
	Plan      map[string]string `json:"plan,omitempty"`
	Provider  map[string]string `json:"provider,omitempty"`
	Map       map[string]string `json:"map,omitempty"`
	Migration map[string]string `json:"migration,omitempty"`
	VM        string            `json:"vm,omitempty"`
	VMName    string            `json:"vmName,omitempty"`
	VMID      string            `json:"vmID,omitempty"`
	ReQ       int               `json:"reQ,omitempty"`
}

// RawLogLine represents a log line that could not be parsed as JSON.
// Used to preserve malformed or non-JSON log lines in the output.
type RawLogLine struct {
	Raw string `json:"raw"`
}

// LogFilterParams holds parameters for filtering and formatting structured JSON logs.
type LogFilterParams struct {
	FilterPlan      string
	FilterProvider  string
	FilterVM        string
	FilterMigration string
	FilterLevel     string
	FilterLogger    string
	Grep            string
	IgnoreCase      bool
	LogFormat       string // "json", "text", or "pretty"
}
