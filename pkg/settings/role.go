package settings

import (
	"fmt"
	liberr "github.com/konveyor/controller/pkg/error"
	"os"
	"strings"
)

//
// Environment variables & roles.
const (
	Roles         = "ROLE"
	InventoryRole = "inventory"
	MtvRole       = "mtv"
)

//
// Role settings
type Role struct {
	// Enabled roles.
	Roles map[string]bool
}

//
// Load settings.
func (r *Role) Load() error {
	r.Roles = map[string]bool{}
	if s, found := os.LookupEnv(Roles); found {
		for _, role := range strings.Split(s, ",") {
			role = strings.ToLower(strings.TrimSpace(role))
			switch role {
			case MtvRole, InventoryRole:
				r.Roles[role] = true
			default:
				list := strings.Join([]string{
					MtvRole,
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
		r.Roles[MtvRole] = true
	}

	return nil
}

//
// Test has-role.
func (r *Role) Has(name string) bool {
	_, found := r.Roles[name]
	return found
}
