package ocp

import plancontext "github.com/konveyor/forklift-controller/pkg/controller/plan/context"

type Builder struct {
	*plancontext.Context
	macConflictsMap map[string]string
}
