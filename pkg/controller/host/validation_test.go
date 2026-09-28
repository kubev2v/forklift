package host

import (
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
)

func TestSetOverallStatusConditions(t *testing.T) {
	cases := []struct {
		name           string
		overallStatus  string
		wantNotGreen   bool
		wantNotHealthy bool
		wantBlocker    bool
	}{
		{
			name:          "green",
			overallStatus: "green",
		},
		{
			name:          "yellow",
			overallStatus: "yellow",
			wantNotGreen:  true,
		},
		{
			name:          "gray",
			overallStatus: "gray",
			wantNotGreen:  true,
		},
		{
			name:           "red",
			overallStatus:  "red",
			wantNotGreen:   true,
			wantNotHealthy: true,
			wantBlocker:    true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			host := &api.Host{}
			setOverallStatusConditions(host, tc.overallStatus)

			if got := host.Status.HasCondition(HostStatusNotGreen); got != tc.wantNotGreen {
				t.Errorf("HostStatusNotGreen = %v, want %v", got, tc.wantNotGreen)
			}
			if got := host.Status.HasCondition(NotHealthy); got != tc.wantNotHealthy {
				t.Errorf("NotHealthy = %v, want %v", got, tc.wantNotHealthy)
			}
			if got := host.Status.HasBlockerCondition(); got != tc.wantBlocker {
				t.Errorf("HasBlockerCondition = %v, want %v", got, tc.wantBlocker)
			}

			if tc.wantNotGreen {
				cnd := host.Status.FindCondition(HostStatusNotGreen)
				if cnd == nil {
					t.Fatal("expected HostStatusNotGreen condition")
				}
				if cnd.Category != Warn {
					t.Errorf("HostStatusNotGreen category = %q, want %q", cnd.Category, Warn)
				}
			}
			if tc.wantNotHealthy {
				cnd := host.Status.FindCondition(NotHealthy)
				if cnd == nil {
					t.Fatal("expected NotHealthy condition")
				}
				if cnd.Category != Critical {
					t.Errorf("NotHealthy category = %q, want %q", cnd.Category, Critical)
				}
			}
		})
	}
}
