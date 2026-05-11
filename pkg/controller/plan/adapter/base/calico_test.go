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
