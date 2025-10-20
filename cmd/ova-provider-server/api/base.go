package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kubev2v/forklift/pkg/lib/logging"
)

var log = logging.WithName("ova|api")

type BadRequestError struct {
	Reason string
}

func (r *BadRequestError) Error() string { return r.Reason }

type ConflictError struct {
	Reason string
}

func (r *ConflictError) Error() string { return r.Reason }

// ErrorHandler renders error conditions from lower handlers.
func ErrorHandler() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Next()
		if len(ctx.Errors) == 0 {
			return
		}
		err := ctx.Errors.Last()
		if errors.Is(err.Err, &BadRequestError{}) {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if errors.Is(err.Err, &ConflictError{}) {
			ctx.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
