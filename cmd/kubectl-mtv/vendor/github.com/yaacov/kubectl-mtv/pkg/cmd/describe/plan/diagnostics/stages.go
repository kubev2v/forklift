package diagnostics

import (
	"regexp"
	"strings"
)

var (
	// Phase markers: [ 0.0] Setting up the source ...
	// 1-2 decimal digits distinguish from kernel dmesg (3+ digits).
	v2vPhaseRe = regexp.MustCompile(`^\[\s+(\d+\.\d{1,2})\]\s+(.*)`)

	// Progress percentage from monitoring lines: "completed 50 %"
	v2vMonitorProgressRe = regexp.MustCompile(`completed\s+(\d+)\s*%`)
)

var stageMapping = []struct {
	keyword string
	stage   string
}{
	{"Setting up the source", "source-setup"},
	{"Opening the source", "source-open"},
	{"Inspecting the source", "inspect"},
	{"Mapping filesystem", "map-fs"},
	{"Creating an overlay", "overlay"},
	{"Setting up the destination", "dest-setup"},
	{"Copying disk", "disk-copy"},
	{"Creating output metadata", "metadata"},
	{"Finishing off", "finish"},
}

// matchV2VStage returns the stage name if the message contains a known phase keyword.
func matchV2VStage(message string) string {
	for _, m := range stageMapping {
		if strings.Contains(message, m.keyword) {
			return m.stage
		}
	}
	return ""
}

// detectV2VStage performs a forward pass over log lines, identifying the last
// reached virt-v2v stage and the last known progress percentage.
// Returns empty strings if the log does not contain virt-v2v phase markers.
func detectV2VStage(lines []string) (stage string, progressPct string) {
	for _, line := range lines {
		if m := v2vPhaseRe.FindStringSubmatch(line); m != nil {
			if s := matchV2VStage(m[2]); s != "" {
				stage = s
				if s == "source-setup" {
					progressPct = ""
				}
			}
		}

		if strings.HasPrefix(line, "virt-v2v monitoring:") {
			if pm := v2vMonitorProgressRe.FindStringSubmatch(line); pm != nil {
				progressPct = pm[1]
			}
		}
	}
	return stage, progressPct
}
