package nutanix

import "testing"

func TestV3ImageNameFilter(t *testing.T) {
	name := "forklift-migration-abc-vm-1-disk-1"
	if filter := V3ImageNameFilter(name); filter != "name=="+name {
		t.Fatalf("expected unquoted filter, got %q", filter)
	}
}

func TestEscapeFIQLLiteral(t *testing.T) {
	if got := escapeFIQLLiteral("name with spaces"); got != "'name with spaces'" {
		t.Fatalf("expected quoted literal, got %q", got)
	}
	if got := escapeFIQLLiteral("O'Brien"); got != "'O''Brien'" {
		t.Fatalf("expected escaped single quotes, got %q", got)
	}
}

func TestCoalesce(t *testing.T) {
	if result := Coalesce("", "value-b", "value-c"); result != "value-b" {
		t.Errorf("expected first non-empty value 'value-b', got %q", result)
	}
	if result := Coalesce(""); result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestParseNumericString(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected int64
		expectOK bool
	}{
		{"numeric string", "12345", 12345, true},
		{"non-numeric string", "not-a-number", 0, false},
		{"int", 42, 42, true},
		{"int64", int64(99), 99, true},
		{"float64", float64(7), 7, true},
		{"unsupported type", true, 0, false},
		{"missing value", nil, 0, false},
		{"zero string", "0", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := ParseNumericString(tt.input)
			if result != tt.expected || ok != tt.expectOK {
				t.Errorf("expected (%d, %v), got (%d, %v)", tt.expected, tt.expectOK, result, ok)
			}
		})
	}
}

func TestNutanixBool(t *testing.T) {
	tests := []struct {
		name     string
		value    any
		expected bool
	}{
		{"native true", true, true},
		{"native false", false, false},
		{"enabled string", "ENABLED", true},
		{"disabled string", "DISABLED", false},
		{"true string", "true", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NutanixBool(tt.value); got != tt.expected {
				t.Fatalf("expected %v, got %v", tt.expected, got)
			}
		})
	}
}
