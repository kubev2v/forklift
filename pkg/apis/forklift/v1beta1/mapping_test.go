package v1beta1

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestDestinationNetwork_RoundTrip_NoCalico(t *testing.T) {
	// Entries without the calico field should round-trip without a "calico"
	// key appearing in the JSON.
	for _, typ := range []string{"pod", "multus", "ignored"} {
		in := DestinationNetwork{Type: typ, Namespace: "ns", Name: "n"}
		raw, err := json.Marshal(in)
		if err != nil {
			t.Fatalf("marshal %q: %v", typ, err)
		}
		if strings.Contains(string(raw), "calico") {
			t.Errorf("type=%q JSON includes calico key: %s", typ, raw)
		}
		var out DestinationNetwork
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatalf("unmarshal %q: %v", typ, err)
		}
		if !reflect.DeepEqual(out, in) {
			t.Errorf("round-trip mismatch for %q: got %+v, want %+v", typ, out, in)
		}
	}
}

func TestDestinationNetwork_RoundTrip_CalicoEmpty(t *testing.T) {
	// The empty calico block is the minimal opt-in — its presence must
	// survive a round trip (nil vs empty-struct distinction is load-bearing).
	in := DestinationNetwork{Type: "pod", Calico: &CalicoDestination{}}
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(raw), `"calico":{}`) {
		t.Errorf("JSON missing empty calico block: %s", raw)
	}
	var out DestinationNetwork
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Calico == nil {
		t.Fatalf("calico block lost in round trip: %s", raw)
	}
	if !reflect.DeepEqual(out, in) {
		t.Errorf("round-trip mismatch: got %+v, want %+v", out, in)
	}
}

func TestDestinationNetwork_RoundTrip_CalicoWithNetwork(t *testing.T) {
	in := DestinationNetwork{Type: "pod", Calico: &CalicoDestination{Network: "vlan100"}}
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(raw), `"calico":{"network":"vlan100"}`) {
		t.Errorf("JSON missing calico.network: %s", raw)
	}
	var out DestinationNetwork
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(out, in) {
		t.Errorf("round-trip mismatch: got %+v, want %+v", out, in)
	}
}

func TestDestinationNetwork_RoundTrip_CalicoWithNetworkAndVlan(t *testing.T) {
	in := DestinationNetwork{Type: "pod", Calico: &CalicoDestination{Network: "vlan100", Vlan: 100}}
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(raw), `"network":"vlan100"`) {
		t.Errorf("JSON missing calico.network: %s", raw)
	}
	if !strings.Contains(string(raw), `"vlan":100`) {
		t.Errorf("JSON missing calico.vlan: %s", raw)
	}
	var out DestinationNetwork
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(out, in) {
		t.Errorf("round-trip mismatch: got %+v, want %+v", out, in)
	}
}

func TestCalicoDestination_OmitsZeroVlan(t *testing.T) {
	// Zero (implicit) vlan must serialize as omitted so users who set 0 in
	// YAML don't trip the kubebuilder Maximum validator on follow-up updates.
	in := DestinationNetwork{Type: "pod", Calico: &CalicoDestination{Network: "prod", Vlan: 0}}
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(raw), `"vlan":`) {
		t.Errorf("Vlan=0 should be omitted, got: %s", raw)
	}
}
