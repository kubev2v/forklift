package staticip

import (
	"strings"
	"testing"
)

const testTemplate = `$networkConfigs = @(
{{- range $i, $cfg := . }}
    @{
        MAC = '{{ lower $cfg.MAC }}'
        IPs = @{{ formatIPs $cfg.IPs }}
        PrefixLength = {{ (index $cfg.IPs 0).PrefixLength }}
        DNS = @{{ formatDNS $cfg }}
    }{{ if ne (add $i 1) (len $) }},{{ end }}
{{- end }}
)`

func TestRenderComplementaryTemplate(t *testing.T) {
	t.Parallel()
	configs := []IPConfig{
		{
			MAC: "AA-BB-CC-DD-EE-FF",
			IPs: []IPEntry{
				{IP: "10.0.0.2", Gateway: "10.0.0.254", PrefixLength: "24", DNS: []string{"8.8.8.8", "8.8.4.4"}},
			},
		},
	}
	result, err := RenderComplementaryTemplate(configs, []byte(testTemplate))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := string(result)
	if !strings.Contains(output, "aa-bb-cc-dd-ee-ff") {
		t.Error("expected lowercased MAC in output")
	}
	if !strings.Contains(output, "'10.0.0.2'") {
		t.Error("expected IP 10.0.0.2 in output")
	}
	if !strings.Contains(output, "PrefixLength = 24") {
		t.Error("expected PrefixLength 24 in output")
	}
	if !strings.Contains(output, "'8.8.8.8'") {
		t.Error("expected DNS 8.8.8.8 in output")
	}
}

func TestRenderComplementaryTemplate_SingleQuoteEscaping(t *testing.T) {
	t.Parallel()
	configs := []IPConfig{
		{
			MAC: "AA-BB-CC-DD-EE-FF",
			IPs: []IPEntry{
				{IP: "10.0.0.2'evil", Gateway: "10.0.0.254", PrefixLength: "24", DNS: []string{"8.8.8.8'bad", "8.8.4.4"}},
			},
		},
	}
	result, err := RenderComplementaryTemplate(configs, []byte(testTemplate))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := string(result)
	if !strings.Contains(output, "'10.0.0.2''evil'") {
		t.Errorf("expected escaped IP with doubled single-quotes, got: %s", output)
	}
	if !strings.Contains(output, "'8.8.8.8''bad'") {
		t.Errorf("expected escaped DNS with doubled single-quotes, got: %s", output)
	}
}

func TestRenderComplementaryTemplate_Empty(t *testing.T) {
	t.Parallel()
	result, err := RenderComplementaryTemplate(nil, []byte(testTemplate))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := string(result)
	if !strings.Contains(output, "$networkConfigs = @(") {
		t.Error("expected template preamble in output")
	}
}
