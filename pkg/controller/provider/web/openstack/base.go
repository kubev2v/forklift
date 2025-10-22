package openstack

import (
	"strings"

	pathlib "path"

	"github.com/gin-gonic/gin"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/openstack"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
	"github.com/kubev2v/forklift/pkg/lib/logging"
)

// Package logger.
var log = logging.WithName("web|openstack")

// Fields.
const (
	DetailParam = base.DetailParam
	NameParam   = base.NameParam
)

// Base handler.
type Handler struct {
	base.Handler
}

// Build list predicate.
func (h Handler) Predicate(ctx *gin.Context) (p libmodel.Predicate) {
	q := ctx.Request.URL.Query()
	name := q.Get(NameParam)
	if len(name) > 0 {
		path := strings.Split(name, "/")
		name := path[len(path)-1]
		p = libmodel.Eq(NameParam, name)
	}

	return
}

// Build list options.
func (h Handler) ListOptions(ctx *gin.Context) libmodel.ListOptions {
	detail := h.Detail
	if detail > 0 {
		detail = model.MaxDetail
	}
	return libmodel.ListOptions{
		Predicate: h.Predicate(ctx),
		Detail:    detail,
		Page:      &h.Page,
	}
}

// Path builder.
type PathBuilder struct {
	// Database.
	DB libmodel.DB
	// Cached resources.
	cache map[string]interface{}
}

// Build.
func (r *PathBuilder) Path(m model.Model) (path string) {
	var err error
	if r.cache == nil {
		r.cache = map[string]interface{}{}
	}
	switch m := m.(type) {
	case *model.Project:
		project := m
		path = project.Name
		if project.IsDomain {
			return
		}
		if project.ParentID != project.DomainID {
			var parentProject *model.Project
			parentProject, err = r.getProject(project.ParentID)
			path = pathlib.Join(r.Path(parentProject), path)
		}

	case *model.VM:
		vm := m
		var project *model.Project
		project, err = r.getProject(vm.TenantID)
		if err != nil {
			return
		}
		path = pathlib.Join(r.Path(project), vm.Name)
	}

	if err != nil {
		log.Error(
			err,
			"path builder failed.",
			"model",
			libmodel.Describe(m))
	}

	return
}

func (r *PathBuilder) getProject(projectID string) (project *model.Project, err error) {
	project, cached := r.cache[projectID].(*model.Project)
	if !cached {
		project = &model.Project{
			Base: model.Base{ID: projectID},
		}
		err = r.DB.Get(project)
		if err != nil {
			return
		}
		r.cache[projectID] = project
	}

	return
}
