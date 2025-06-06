package catalog

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const Collection = "/status"

type Handler struct {
	Manager *Manager
}

func (r *Handler) AddRoutes(e *gin.Engine) {
	e.GET(Collection, r.List)
}

func (r *Handler) List(ctx *gin.Context) {
	statuses := r.Manager.GetStatuses()
	ctx.JSON(http.StatusOK, statuses)
}
