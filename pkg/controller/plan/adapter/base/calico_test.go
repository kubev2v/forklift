package base

import (
	"testing"

	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSetCalicoMAC(t *testing.T) {
	tests := []struct {
		name    string
		initial map[string]string
		ifname  string
		mac     string
		wantKey string
		wantVal string
	}{
		{
			name:    "NilAnnotationsLazyInits",
			initial: nil,
			ifname:  "net-0",
			mac:     "aa:bb:cc:dd:ee:ff",
			wantKey: "cni.projectcalico.org/net-0.hwAddr",
			wantVal: "aa:bb:cc:dd:ee:ff",
		},
		{
			name:    "ExistingAnnotationsPreserved",
			initial: map[string]string{"foo": "bar"},
			ifname:  "net-1",
			mac:     "11:22:33:44:55:66",
			wantKey: "cni.projectcalico.org/net-1.hwAddr",
			wantVal: "11:22:33:44:55:66",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &meta.ObjectMeta{Annotations: tt.initial}
			SetCalicoMAC(m, tt.ifname, tt.mac)
			if got := m.Annotations[tt.wantKey]; got != tt.wantVal {
				t.Errorf("annotations[%q] = %q, want %q", tt.wantKey, got, tt.wantVal)
			}
			if tt.initial != nil {
				if got := m.Annotations["foo"]; got != "bar" {
					t.Errorf("pre-existing annotation lost: %q", got)
				}
			}
		})
	}
}

func TestSetCalicoStaticIPs(t *testing.T) {
	tests := []struct {
		name       string
		initial    map[string]string
		ifname     string
		ips        []string
		wantKey    string
		wantVal    string
		wantMissed bool // when true, expect the key to NOT be present
	}{
		{
			name:    "SingleIP",
			ifname:  "net-0",
			ips:     []string{"10.0.0.5"},
			wantKey: "cni.projectcalico.org/net-0.ipAddrs",
			wantVal: `["10.0.0.5"]`,
		},
		{
			name:    "MultipleIPs",
			ifname:  "net-1",
			ips:     []string{"10.0.0.5", "10.0.0.6"},
			wantKey: "cni.projectcalico.org/net-1.ipAddrs",
			wantVal: `["10.0.0.5","10.0.0.6"]`,
		},
		{
			name:       "EmptySliceIsNoOp",
			ifname:     "net-0",
			ips:        nil,
			wantKey:    "cni.projectcalico.org/net-0.ipAddrs",
			wantMissed: true,
		},
		{
			name:    "ExistingAnnotationsPreserved",
			initial: map[string]string{"foo": "bar"},
			ifname:  "net-0",
			ips:     []string{"10.0.0.5"},
			wantKey: "cni.projectcalico.org/net-0.ipAddrs",
			wantVal: `["10.0.0.5"]`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &meta.ObjectMeta{Annotations: tt.initial}
			if err := SetCalicoStaticIPs(m, tt.ifname, tt.ips); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got, present := m.Annotations[tt.wantKey]
			if tt.wantMissed {
				if present {
					t.Errorf("annotations[%q] present = %q, want missing", tt.wantKey, got)
				}
				return
			}
			if got != tt.wantVal {
				t.Errorf("annotations[%q] = %q, want %q", tt.wantKey, got, tt.wantVal)
			}
			if tt.initial != nil {
				if v := m.Annotations["foo"]; v != "bar" {
					t.Errorf("pre-existing annotation lost: %q", v)
				}
			}
		})
	}
}

func TestSetCalicoPrimaryMAC(t *testing.T) {
	t.Run("NilAnnotationsLazyInits", func(t *testing.T) {
		m := &meta.ObjectMeta{}
		SetCalicoPrimaryMAC(m, "aa:bb:cc:dd:ee:ff")
		if got := m.Annotations[CalicoAnnPrimaryHwAddr]; got != "aa:bb:cc:dd:ee:ff" {
			t.Errorf("got %q, want %q", got, "aa:bb:cc:dd:ee:ff")
		}
	})
	t.Run("EmptyMacIsNoOp", func(t *testing.T) {
		m := &meta.ObjectMeta{}
		SetCalicoPrimaryMAC(m, "")
		if m.Annotations != nil {
			t.Errorf("annotations should be nil, got %v", m.Annotations)
		}
	})
	t.Run("ExistingAnnotationsPreserved", func(t *testing.T) {
		m := &meta.ObjectMeta{Annotations: map[string]string{"foo": "bar"}}
		SetCalicoPrimaryMAC(m, "11:22:33:44:55:66")
		if got := m.Annotations[CalicoAnnPrimaryHwAddr]; got != "11:22:33:44:55:66" {
			t.Errorf("got %q, want %q", got, "11:22:33:44:55:66")
		}
		if got := m.Annotations["foo"]; got != "bar" {
			t.Errorf("pre-existing annotation lost: %q", got)
		}
	})
}

func TestSetCalicoPrimaryStaticIPs(t *testing.T) {
	t.Run("SingleIPLazyInit", func(t *testing.T) {
		m := &meta.ObjectMeta{}
		if err := SetCalicoPrimaryStaticIPs(m, []string{"10.0.0.5"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := m.Annotations[CalicoAnnPrimaryIPs]; got != `["10.0.0.5"]` {
			t.Errorf("got %q, want %q", got, `["10.0.0.5"]`)
		}
	})
	t.Run("MultipleIPs", func(t *testing.T) {
		m := &meta.ObjectMeta{}
		if err := SetCalicoPrimaryStaticIPs(m, []string{"10.0.0.5", "10.0.0.6"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := m.Annotations[CalicoAnnPrimaryIPs]; got != `["10.0.0.5","10.0.0.6"]` {
			t.Errorf("got %q, want %q", got, `["10.0.0.5","10.0.0.6"]`)
		}
	})
	t.Run("EmptySliceIsNoOp", func(t *testing.T) {
		m := &meta.ObjectMeta{}
		if err := SetCalicoPrimaryStaticIPs(m, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, present := m.Annotations[CalicoAnnPrimaryIPs]; present {
			t.Errorf("annotation should not be present")
		}
	})
}

func TestSetCalicoPrimaryNetwork(t *testing.T) {
	t.Run("PopulatedLazyInit", func(t *testing.T) {
		m := &meta.ObjectMeta{}
		SetCalicoPrimaryNetwork(m, "vlan100")
		if got := m.Annotations[CalicoAnnPrimaryNetwork]; got != "vlan100" {
			t.Errorf("got %q, want %q", got, "vlan100")
		}
	})
	t.Run("EmptyIsNoOp", func(t *testing.T) {
		m := &meta.ObjectMeta{}
		SetCalicoPrimaryNetwork(m, "")
		if m.Annotations != nil {
			t.Errorf("annotations should be nil, got %v", m.Annotations)
		}
	})
}

func TestSetCalicoPrimaryVlan(t *testing.T) {
	t.Run("PopulatedLazyInit", func(t *testing.T) {
		m := &meta.ObjectMeta{}
		SetCalicoPrimaryVlan(m, 200)
		if got := m.Annotations[CalicoAnnPrimaryVlan]; got != "200" {
			t.Errorf("got %q, want %q", got, "200")
		}
	})
	t.Run("ZeroIsNoOp", func(t *testing.T) {
		m := &meta.ObjectMeta{}
		SetCalicoPrimaryVlan(m, 0)
		if m.Annotations != nil {
			t.Errorf("annotations should be nil, got %v", m.Annotations)
		}
	})
}

func TestStampCalicoPrimary(t *testing.T) {
	t.Run("AllFieldsPopulated", func(t *testing.T) {
		m := &meta.ObjectMeta{}
		err := StampCalicoPrimary(m, CalicoPrimaryParams{
			MAC: "aa:bb:cc:dd:ee:ff", IPs: []string{"10.0.0.5"}, Network: "vlan100", Vlan: 100,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if m.Annotations[CalicoAnnPrimaryHwAddr] != "aa:bb:cc:dd:ee:ff" {
			t.Errorf("hwAddr missing")
		}
		if m.Annotations[CalicoAnnPrimaryIPs] != `["10.0.0.5"]` {
			t.Errorf("ipAddrs missing")
		}
		if m.Annotations[CalicoAnnPrimaryNetwork] != "vlan100" {
			t.Errorf("networks missing")
		}
		if m.Annotations[CalicoAnnPrimaryVlan] != "100" {
			t.Errorf("vlan missing")
		}
	})
	t.Run("ZeroValuesAllNoOp", func(t *testing.T) {
		m := &meta.ObjectMeta{}
		if err := StampCalicoPrimary(m, CalicoPrimaryParams{}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if m.Annotations != nil {
			t.Errorf("annotations should be nil with all-zero params, got %v", m.Annotations)
		}
	})
	t.Run("MACOnly", func(t *testing.T) {
		m := &meta.ObjectMeta{}
		if err := StampCalicoPrimary(m, CalicoPrimaryParams{MAC: "aa:bb:cc:dd:ee:ff"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, has := m.Annotations[CalicoAnnPrimaryIPs]; has {
			t.Errorf("ipAddrs should be absent")
		}
		if _, has := m.Annotations[CalicoAnnPrimaryNetwork]; has {
			t.Errorf("networks should be absent")
		}
		if m.Annotations[CalicoAnnPrimaryHwAddr] != "aa:bb:cc:dd:ee:ff" {
			t.Errorf("hwAddr missing")
		}
	})
}
