package health

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// HasFilters returns true if any JSON-specific filters are set.
func (p LogFilterParams) HasFilters() bool {
	return p.FilterPlan != "" ||
		p.FilterProvider != "" ||
		p.FilterVM != "" ||
		p.FilterMigration != "" ||
		p.FilterLevel != "" ||
		p.FilterLogger != ""
}

// ProcessLogs applies grep filtering, JSON detection, structured filtering, and formatting
// to raw log output. It returns the formatted result and an output format hint ("json", "text",
// or "pretty"). If the logs are not JSON, raw text is returned as-is.
func ProcessLogs(rawLogs string, params LogFilterParams) (interface{}, string, error) {
	output := rawLogs

	if params.Grep != "" {
		filtered, err := FilterByPattern(output, params.Grep, params.IgnoreCase)
		if err != nil {
			return nil, "", fmt.Errorf("grep filter error: %w", err)
		}
		output = filtered
	}

	if !LooksLikeJSONLogs(output) {
		return output, "text", nil
	}

	format := params.LogFormat
	switch format {
	case "json", "text", "pretty":
	case "":
		format = "pretty"
	default:
		return nil, "", fmt.Errorf("invalid format %q, valid formats: json, text, pretty", format)
	}

	normalized := params
	normalized.LogFormat = format
	result, err := FilterAndFormatJSONLogs(output, normalized)
	if err != nil {
		return nil, "", fmt.Errorf("JSON log processing error: %w", err)
	}

	return result, format, nil
}

// FilterByPattern filters log lines by a regex pattern.
// If pattern is empty, returns the original logs unchanged.
// If ignoreCase is true, the pattern matching is case-insensitive.
func FilterByPattern(logs string, pattern string, ignoreCase bool) (string, error) {
	if pattern == "" {
		return logs, nil
	}

	prefix := ""
	if ignoreCase {
		prefix = "(?i)"
	}

	re, err := regexp.Compile(prefix + pattern)
	if err != nil {
		return "", fmt.Errorf("invalid grep pattern: %w", err)
	}

	var filtered []string
	lines := strings.Split(logs, "\n")
	for _, line := range lines {
		if re.MatchString(line) {
			filtered = append(filtered, line)
		}
	}

	return strings.Join(filtered, "\n"), nil
}

// LooksLikeJSONLogs checks if the logs appear to be in JSON format by examining up to 5
// non-empty lines. It handles the kubectl --timestamps prefix
// (e.g., "2026-02-05T10:45:52.123Z {"level":"info",...}").
// Returns true as soon as any scanned line contains valid JSON with expected log fields
// (level, msg). Returns false if none of the scanned lines yield a valid JSON entry.
func LooksLikeJSONLogs(logs string) bool {
	lines := strings.Split(logs, "\n")

	const maxLinesToCheck = 5
	checkedLines := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		checkedLines++
		if checkedLines > maxLinesToCheck {
			break
		}

		idx := strings.Index(trimmed, "{")
		if idx < 0 {
			continue
		}
		jsonPart := trimmed[idx:]

		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(jsonPart), &entry); err != nil {
			continue
		}

		_, hasLevel := entry["level"]
		_, hasMsg := entry["msg"]

		if hasLevel && hasMsg {
			return true
		}
	}

	return false
}

// MatchesFilters checks if a log entry matches all specified filters.
func MatchesFilters(entry JSONLogEntry, p LogFilterParams) bool {
	if p.FilterLevel != "" && !strings.EqualFold(entry.Level, p.FilterLevel) {
		return false
	}
	if p.FilterLogger != "" {
		loggerType := strings.Split(entry.Logger, "|")[0]
		if !strings.EqualFold(loggerType, p.FilterLogger) {
			return false
		}
	}
	if p.FilterPlan != "" {
		planName := ""
		if entry.Plan != nil {
			planName = entry.Plan["name"]
		}
		if !strings.EqualFold(planName, p.FilterPlan) {
			return false
		}
	}
	if p.FilterProvider != "" {
		providerName := ""
		if entry.Provider != nil {
			providerName = entry.Provider["name"]
		}
		if !strings.EqualFold(providerName, p.FilterProvider) {
			return false
		}
	}
	if p.FilterVM != "" {
		vmMatch := strings.EqualFold(entry.VM, p.FilterVM) ||
			strings.EqualFold(entry.VMName, p.FilterVM) ||
			strings.EqualFold(entry.VMID, p.FilterVM)
		if !vmMatch {
			return false
		}
	}
	if p.FilterMigration != "" {
		loggerParts := strings.Split(entry.Logger, "|")
		loggerType := loggerParts[0]
		if !strings.EqualFold(loggerType, "migration") {
			return false
		}
		migrationName := ""
		if entry.Migration != nil {
			migrationName = entry.Migration["name"]
		}
		if migrationName == "" && len(loggerParts) > 1 {
			migrationName = loggerParts[1]
		}
		if !strings.EqualFold(migrationName, p.FilterMigration) {
			return false
		}
	}
	return true
}

// FilterAndFormatJSONLogs parses JSON logs, applies filters, and formats output.
// It returns the processed logs based on the specified format:
//   - "json": Array of mixed JSONLogEntry and RawLogLine (for malformed lines)
//   - "text": Original raw JSONL lines (filtered)
//   - "pretty": Human-readable formatted output
func FilterAndFormatJSONLogs(logs string, p LogFilterParams) (interface{}, error) {
	lines := strings.Split(strings.TrimSpace(logs), "\n")
	if len(lines) == 0 || (len(lines) == 1 && lines[0] == "") {
		return []interface{}{}, nil
	}

	logLines := make([]interface{}, 0)
	var filteredLines []string
	hasFilters := p.HasFilters()

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		jsonPart := line
		timestampPrefix := ""
		if idx := strings.Index(line, "{"); idx > 0 {
			timestampPrefix = line[:idx]
			jsonPart = line[idx:]
		}

		var entry JSONLogEntry
		if err := json.Unmarshal([]byte(jsonPart), &entry); err != nil {
			if !hasFilters {
				logLines = append(logLines, RawLogLine{Raw: line})
				filteredLines = append(filteredLines, line)
			}
			continue
		}

		if hasFilters && !MatchesFilters(entry, p) {
			continue
		}

		logLines = append(logLines, entry)
		filteredLines = append(filteredLines, timestampPrefix+jsonPart)
	}

	format := p.LogFormat
	if format == "" {
		format = "json"
	}

	switch format {
	case "json":
		return logLines, nil
	case "text":
		return strings.Join(filteredLines, "\n"), nil
	case "pretty":
		return FormatPrettyLogs(logLines), nil
	default:
		return logLines, nil
	}
}

// FormatPrettyLogs formats log entries in a human-readable format.
// It handles both JSONLogEntry (parsed) and RawLogLine (malformed) types.
func FormatPrettyLogs(logLines []interface{}) string {
	var lines []string
	for _, item := range logLines {
		switch v := item.(type) {
		case RawLogLine:
			lines = append(lines, v.Raw)
		case JSONLogEntry:
			levelUpper := strings.ToUpper(v.Level)
			ctx := ""

			if v.Plan != nil && v.Plan["name"] != "" {
				ctx = fmt.Sprintf(" plan=%s", v.Plan["name"])
				if ns := v.Plan["namespace"]; ns != "" {
					ctx += fmt.Sprintf("/%s", ns)
				}
			} else if v.Provider != nil && v.Provider["name"] != "" {
				ctx = fmt.Sprintf(" provider=%s", v.Provider["name"])
				if ns := v.Provider["namespace"]; ns != "" {
					ctx += fmt.Sprintf("/%s", ns)
				}
			} else if v.Map != nil && v.Map["name"] != "" {
				ctx = fmt.Sprintf(" map=%s", v.Map["name"])
				if ns := v.Map["namespace"]; ns != "" {
					ctx += fmt.Sprintf("/%s", ns)
				}
			}

			if v.VM != "" {
				ctx += fmt.Sprintf(" vm=%s", v.VM)
			} else if v.VMName != "" {
				ctx += fmt.Sprintf(" vm=%s", v.VMName)
			}

			line := fmt.Sprintf("[%s] %s %s: %s%s", levelUpper, v.Ts, v.Logger, v.Msg, ctx)
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}
