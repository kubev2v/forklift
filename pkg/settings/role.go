package settings

import (
	"fmt"
	"os"
	"strings"

	liberr "github.com/kubev2v/forklift/pkg/lib/error"
)

// Environment variables & roles.
const (
	Roles         = "ROLE"
	InventoryRole = "inventory"
	MainRole      = "main"
)

// Role settings
type Role struct {
	// Enabled roles.
	Roles map[string]bool
}

// Load settings.
func (r *Role) Load() error {
	r.Roles = map[string]bool{}
	if s, found := os.LookupEnv(Roles); found {
		for _, role := range strings.Split(s, ",") {
			role = strings.ToLower(strings.TrimSpace(role))
			switch role {
			case MainRole, InventoryRole:
				r.Roles[role] = true
			default:
				list := strings.Join([]string{
					MainRole,
					InventoryRole},
					"|")
				return liberr.New(
					fmt.Sprintf(
						"%s must be (%s)",
						Roles,
						list))
			}
		}
	} else {
		r.Roles[InventoryRole] = true
		r.Roles[MainRole] = true
	}

	return nil
}

// Test has-role.
func (r *Role) Has(name string) bool {
	_, found := r.Roles[name]
	return found
}
