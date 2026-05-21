package staticip

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
)

// RenderComplementaryTemplate produces the 110_complementary_ips.ps1 script by
// injecting configs into the Go template in tmplContent.
func RenderComplementaryTemplate(configs []IPConfig, tmplContent []byte) ([]byte, error) {
	funcMap := template.FuncMap{
		"lower":     strings.ToLower,
		"add":       addInts,
		"len":       lengthOf,
		"formatIPs": formatIPs,
		"formatDNS": formatDNS,
	}

	tmpl, err := template.New("complementaryIPs").Funcs(funcMap).Parse(string(tmplContent))
	if err != nil {
		return nil, fmt.Errorf("parsing template complementaryIPs: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, configs); err != nil {
		return nil, fmt.Errorf("executing template complementaryIPs: %w", err)
	}
	return buf.Bytes(), nil
}

// addInts: used in templates as {{ add $i 1 }} for 1-based indexing and last-element checks.
func addInts(a, b int) int { return a + b }

// lengthOf: templates cannot call built-in len on custom slices; accepts []IPEntry or []IPConfig.
func lengthOf(v interface{}) int {
	switch s := v.(type) {
	case []IPEntry:
		return len(s)
	case []IPConfig:
		return len(s)
	default:
		panic(fmt.Sprintf("unsupported template input type in lengthOf: %T", v))
	}
}

// escapePowerShellLiteral escapes single quotes inside a PowerShell
// single-quoted string by doubling them (the only escape in PS literals).
func escapePowerShellLiteral(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

// formatIPs: []IPEntry{{IP:"10.0.0.1"},{IP:"10.0.0.2"}} → "(\n'10.0.0.1',\n'10.0.0.2'\n)"
func formatIPs(ips []IPEntry) string {
	var b strings.Builder
	b.WriteString("(\n")
	for i, ip := range ips {
		b.WriteString("'")
		b.WriteString(escapePowerShellLiteral(ip.IP))
		b.WriteString("'")
		if i < len(ips)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString(")")
	return b.String()
}

// formatDNS: extracts DNS from cfg.IPs[0] (shared across all IPs on a NIC) → "(\n'8.8.8.8',\n'8.8.4.4'\n)"
func formatDNS(cfg IPConfig) string {
	if len(cfg.IPs) > 0 {
		dns := cfg.IPs[0].DNS
		var b strings.Builder
		b.WriteString("(\n")
		for i, d := range dns {
			b.WriteString("'")
			b.WriteString(escapePowerShellLiteral(d))
			b.WriteString("'")
			if i < len(dns)-1 {
				b.WriteString(",")
			}
			b.WriteString("\n")
		}
		b.WriteString(")")
		return b.String()
	}
	return "()"
}
