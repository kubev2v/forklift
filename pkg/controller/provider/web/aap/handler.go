package aap

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	"github.com/kubev2v/forklift/pkg/lib/aap"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	"github.com/kubev2v/forklift/pkg/settings"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var log = logging.WithName("web|aap")

const jobTemplatesPath = "/aap/job-templates"

// Default cap for how many job templates to aggregate from AAP (override with query "max").
const (
	defaultJobTemplatesMax  = 500
	absoluteJobTemplatesMax = 2000
)

// Handler serves AAP-related inventory API endpoints (job template listing for the UI).
type Handler struct {
	Client client.Client
}

// AddRoutes implements libweb.RequestHandler.
func (h *Handler) AddRoutes(e *gin.Engine) {
	e.GET(jobTemplatesPath, h.ListJobTemplates)
}

// ListJobTemplates returns all configured AAP job templates (flat list). Pagination is handled inside pkg/lib/aap.
func (h *Handler) ListJobTemplates(ctx *gin.Context) {
	invNS := settings.Settings.Inventory.Namespace
	if invNS == "" {
		log.Error(nil, "inventory namespace not set for AAP handler")
		ctx.Status(http.StatusInternalServerError)
		return
	}

	if base.Settings.AuthRequired {
		nsForAuth := ctx.Request.URL.Query().Get(base.NsParam)
		if nsForAuth == "" {
			nsForAuth = invNS
		}
		orig := ctx.Request.URL.RawQuery
		q := ctx.Request.URL.Query()
		if q.Get(base.NsParam) == "" {
			q.Set(base.NsParam, nsForAuth)
			ctx.Request.URL.RawQuery = q.Encode()
		}
		defer func() { ctx.Request.URL.RawQuery = orig }()
		if status, aerr := base.DefaultAuth.Permit(ctx, &api.Provider{}); status != http.StatusOK {
			ctx.Status(status)
			base.SetForkliftError(ctx, aerr)
			return
		}
	}

	m := settings.Settings.Migration
	if m.AAPURL == "" || m.AAPTokenSecretName == "" {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{
			"message": "AAP is not configured (set ForkliftController aap_url and aap_token_secret_name).",
		})
		return
	}
	ns := invNS
	token, err := aap.GetTokenFromSecretName(ctx.Request.Context(), h.Client, ns, m.AAPTokenSecretName)
	if err != nil {
		log.Error(err, "failed to read AAP token Secret")
		ctx.JSON(http.StatusBadGateway, gin.H{"message": "failed to read AAP token Secret"})
		return
	}

	maxJobs := defaultJobTemplatesMax
	if q := ctx.Query("max"); q != "" {
		n, err := strconv.Atoi(q)
		if err != nil || n < 1 || n > absoluteJobTemplatesMax {
			ctx.JSON(http.StatusBadRequest, gin.H{
				"message": "max must be an integer between 1 and 2000",
			})
			return
		}
		maxJobs = n
	}

	httpTimeout := 30 * time.Second
	if m.AAPTimeoutSeconds > 0 {
		httpTimeout = time.Duration(m.AAPTimeoutSeconds) * time.Second
	}
	cl := aap.NewClient(m.AAPURL, token, httpTimeout)
	results, err := cl.ListAllJobTemplates(ctx.Request.Context(), maxJobs)
	if err != nil {
		log.Error(err, "AAP list job templates failed")
		ctx.JSON(http.StatusBadGateway, gin.H{"message": "failed to list AAP job templates"})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{
		"count":   len(results),
		"results": results,
	})
}
