package forklift_controller

import (
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
)

func TestModeForType(t *testing.T) {
	cases := []struct {
		name string
		typ  api.MigrationType
		want string
	}{
		{"warm", api.MigrationWarm, Warm},
		{"live", api.MigrationLive, Live},
		{"conversion", api.MigrationOnlyConversion, Conversion},
		{"cold", api.MigrationCold, Cold},
		{"empty defaults to cold (legacy plans)", "", Cold},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := modeForType(c.typ)
			if got != c.want {
				t.Fatalf("modeForType(%q) = %q, want %q", c.typ, got, c.want)
			}
		})
	}
}
