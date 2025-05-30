package base

import (
	"github.com/gin-gonic/gin"
)

func SetForkliftError(ctx *gin.Context, err error) {
	if err != nil {
		ctx.Header("forklift-error-message", err.Error())
		_ = ctx.Error(err)
	}
}
