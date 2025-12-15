package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kubev2v/forklift/pkg/lib/logging"
)

var log = logging.WithName("provider|api")

// SetLogger allows provider servers to set a custom logger name.
func SetLogger(name string) {
	log = logging.WithName(name)
}

// BadRequestError represents a 400 error.
type BadRequestError struct {
	Message string
}

func (e *BadRequestError) Error() string {
	return e.Message
}

// NotFoundError represents a 404 error.
type NotFoundError struct {
	Message string
}

func (e *NotFoundError) Error() string {
	return e.Message
}

// ConflictError represents a 409 error.
type ConflictError struct {
	Message string
}

func (e *ConflictError) Error() string {
	return e.Message
}

// ErrorHandler returns a gin middleware that handles errors.
func ErrorHandler() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Next()
		err := ctx.Errors.Last()
		if err == nil {
			return
		}
		log.Error(err, "request failed")
		switch err.Err.(type) {
		case *BadRequestError:
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case *NotFoundError:
			ctx.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case *ConflictError:
			ctx.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
	}
}
