package settings

import (
	"fmt"
	"os"
	"strconv"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
)

// Global
var Settings = ControllerSettings{}

const (
	OpenShift        = "OPENSHIFT"
	Development      = "DEVELOPMENT"
	OpenShiftVersion = "OPENSHIFT_VERSION"
)

// Settings
type ControllerSettings struct {
	// Roles.
	Role
	// Metrics settings.
	Metrics
	// Inventory settings.
	Inventory
	// Migration settings.
	Migration
	// Policy agent settings.
	PolicyAgent
	// Logging settings.
	Logging
	// Profiler settings.
	Profiler
	// Feature gates.
	Features
	OpenShift   bool
	Development bool
}

// Load settings.
func (r *ControllerSettings) Load() error {
	err := r.Role.Load()
	if err != nil {
		return err
	}
	err = r.Metrics.Load()
	if err != nil {
		return err
	}
	err = r.Inventory.Load()
	if err != nil {
		return err
	}
	err = r.Migration.Load()
	if err != nil {
		return err
	}
	err = r.PolicyAgent.Load()
	if err != nil {
		return err
	}
	err = r.Logging.Load()
	if err != nil {
		return err
	}
	err = r.Profiler.Load()
	if err != nil {
		return err
	}
	err = r.Features.Load()
	if err != nil {
		return err
	}
	r.OpenShift = getEnvBool(OpenShift, false)
	r.Development = getEnvBool(Development, false)
	return nil
}

// Get positive integer limit from the environment
// using the specified variable name and default.
func getPositiveEnvLimit(name string, def int) (int, error) {
	return getEnvLimit(name, def, 1)
}

// Get non-negative integer limit from the environment
// using the specified variable name and default.
func getNonNegativeEnvLimit(name string, def int) (int, error) {
	return getEnvLimit(name, def, 0)
}

// Get an integer limit from the environment
// using the specified variable name and default.
func getEnvLimit(name string, def, minimum int) (int, error) {
	limit := 0
	if s, found := os.LookupEnv(name); found {
		n, err := strconv.Atoi(s)
		if err != nil {
			return 0, liberr.New(name + " must be an integer")
		}
		if n < minimum {
			return 0, liberr.New(fmt.Sprintf(name+" must be >= %d", minimum))
		}
		limit = n
	} else {
		limit = def
	}

	return limit, nil
}

// Get boolean.
func getEnvBool(name string, def bool) bool {
	boolean := def
	if s, found := os.LookupEnv(name); found {
		parsed, err := strconv.ParseBool(s)
		if err == nil {
			boolean = parsed
		}
	}

	return boolean
}

// GetVDDKImage gets the VDDK image from provider spec settings with fall back to global settings.
func GetVDDKImage(providerSpecSettings map[string]string) string {
	vddkImage := providerSpecSettings[api.VDDK]
	if vddkImage == "" && Settings.Migration.VddkImage != "" {
		vddkImage = Settings.Migration.VddkImage
	}

	return vddkImage
}
