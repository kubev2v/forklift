package nutanix

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/nutanix"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
)

// Routes.
const (
	WorkloadCollection = "workloads"
	WorkloadsRoot      = ProviderRoot + "/" + WorkloadCollection
	WorkloadRoot       = WorkloadsRoot + "/:" + VMParam
)

// Workload handler.
type WorkloadHandler struct {
	Handler
}

// Add routes to the `gin` router.
func (h *WorkloadHandler) AddRoutes(e *gin.Engine) {
	e.GET(WorkloadRoot, h.Get)
}

// List resources in a REST collection.
func (h WorkloadHandler) List(ctx *gin.Context) {
}

// Get a specific REST resource.
func (h WorkloadHandler) Get(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}
	m := &model.VM{
		Base: model.Base{
			ID: ctx.Param(VMParam),
		},
	}
	db := h.Collector.DB()
	err = db.Get(m)
	if errors.Is(err, model.NotFound) {
		ctx.Status(http.StatusNotFound)
		return
	}
	defer func() {
		if err != nil {
			log.Trace(
				err,
				"url",
				ctx.Request.URL)
			ctx.Status(http.StatusInternalServerError)
		}
	}()
	if err != nil {
		return
	}
	r := Workload{}
	r.With(m)
	pb := PathBuilder{DB: db}
	r.Path = pb.Path(m)
	r.Link(h.Provider)

	ctx.JSON(http.StatusOK, r)
}

// Workload resource.
//
// A superset of the full VM resource used by the policy agent,
// migrator, and hooks for validation and templating purposes.
type Workload struct {
	VM
}

// IsWindows reports whether the VM's guest OS appears to be Windows.
// AHV does not expose an enumerated guest OS type; the OS name is
// free text supplied either at VM creation or by Nutanix Guest Tools.
func (r *Workload) IsWindows() bool {
	combined := strings.ToLower(r.GuestOSID + " " + r.GuestOSVersion)
	return strings.Contains(combined, "win")
}

// Build self link (URI).
func (r *Workload) Link(p *api.Provider) {
	r.SelfLink = base.Link(
		WorkloadRoot,
		base.Params{
			base.ProviderParam: string(p.UID),
			VMParam:            r.ID,
		})
}
