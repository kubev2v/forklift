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
