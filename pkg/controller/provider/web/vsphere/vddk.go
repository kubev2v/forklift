package vsphere

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/base"
	"github.com/konveyor/forklift-controller/pkg/settings"
	buildv1 "github.com/openshift/api/build/v1"
	buildclientset "github.com/openshift/client-go/build/clientset/versioned"
	imagev1client "github.com/openshift/client-go/image/clientset/versioned/typed/image/v1"
	"io"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	VddkRoot = "vddk" // Route
)

var (
	buildConfigName  = "vddk"
	registryImageTag = "vddk:latest"
	vddkTarFileName  = "vddk-%s.tar.gz"
	uploadDir        = "/tmp/uploads"
)

// VddkHandler provides endpoints for VDDK image management.
type VddkHandler struct {
	base.Handler
}

// AddRoutes registers the VDDK-specific HTTP routes on the given Gin engine.
func (h *VddkHandler) AddRoutes(e *gin.Engine) {
	e.POST(VddkRoot+"/build-image", h.BuildImage)
	e.GET(VddkRoot+"/image-url", h.ImageUrl)
	e.GET(VddkRoot+"/download-tar", h.DownloadVddkTar)
}

// BuildImage receives a VDDK tar file, writes it to disk,
// and triggers an OpenShift BuildConfig to build and push the image.
func (h *VddkHandler) BuildImage(ctx *gin.Context) {
	status, err := h.Handler.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}

	file, err := ctx.FormFile("file")
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "No file provided",
		})
		return
	}

	fileName := fmt.Sprintf(vddkTarFileName, uuid.New().String())
	if err := saveFile(filepath.Join(uploadDir, fileName), file); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to save file",
		})
		return
	}

	buildName, err := triggerBuildConfig(fileName)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusAccepted,
		gin.H{
			"status":     "success",
			"message":    "VDDK build started; check your registry in OpenShift",
			"build-name": buildName,
		},
	)
}

// ImageUrl handles HTTP requests to fetch the VDDK image URL.
// it returns a 200 JSON response containing the image reference. On error,
// it writes a JSON error with the appropriate HTTP status.
func (h *VddkHandler) ImageUrl(ctx *gin.Context) {
	status, err := h.Handler.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}

	if h.Handler.WatchRequest {
		h.watchImageURL(ctx)
		return
	}

	url, exists, err := imageReference(ctx.Request.Context(), registryImageTag)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": fmt.Sprintf("Error checking image reference: %v", err),
		})
		return
	}

	if !exists {
		ctx.JSON(http.StatusNotFound, gin.H{
			"status":  "error",
			"message": fmt.Sprintf("Image: %s not found", registryImageTag),
		})
		return
	}

	ctx.JSON(http.StatusOK,
		gin.H{
			"status":   "success",
			"message":  fmt.Sprintf("Image: %s exists", registryImageTag),
			"imageUrl": url,
		},
	)
}

// DownloadVddkTar streams the uploaded VDDK tar back to the client.
func (h *VddkHandler) DownloadVddkTar(ctx *gin.Context) {
	status, err := h.Handler.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}

	filename := ctx.Query("filename")
	if filename == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "No filename provided",
		})
		return
	}

	filePath := filepath.Join(uploadDir, filename)

	if _, err := os.Stat(filePath); err != nil {
		if os.IsNotExist(err) {
			ctx.JSON(http.StatusNotFound, gin.H{
				"status":  "error",
				"message": "VDDK tar not found",
			})
		} else {
			ctx.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": fmt.Sprintf("Failed to stat file: %v", err),
			})
		}
		return
	}

	ctx.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	ctx.Header("Content-Type", "application/octet-stream")

	ctx.File(filePath)
}

// saveFile writes the uploaded multipart file to the given path, creating
// the upload directory if needed. It removes all the old files older than 1 hour.
func saveFile(filePath string, file *multipart.FileHeader) error {
	src, err := file.Open()
	if err != nil {
		return fmt.Errorf("could not process uploaded file: %v", err)
	}
	defer src.Close()

	if err := os.MkdirAll(uploadDir, 0600); err != nil {
		return fmt.Errorf("could not prepare upload directory: %v", err)
	}

	dst, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("error: %v, Could not save file on disk: %s. ", err, filePath)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("error copy to the local file: %v", err)
	}

	go cleanOldFiles(uploadDir, time.Hour) // background cleanup: remove files older than 1 hour

	return nil
}

// triggerBuildConfig triggers the OpenShift BuildConfig to build and push the VDDK image.
func triggerBuildConfig(targetTarFile string) (string, error) {
	buildClient, err := NewBuildClient()
	if err != nil {
		return "", err
	}

	buildRequest := &buildv1.BuildRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name: buildConfigName,
		},
		DockerStrategyOptions: &buildv1.DockerStrategyOptions{
			BuildArgs: []corev1.EnvVar{{
				Name:  "VDDK_FILE",
				Value: targetTarFile,
			}},
		},
	}

	buildObj, err := buildClient.BuildV1().
		BuildConfigs(settings.Settings.Namespace).
		Instantiate(context.TODO(), buildConfigName, buildRequest, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("start build: %w", err)
	}

	return buildObj.Name, nil
}

// imageReference returns (image url, true, nil) if the given ImageStreamTag exists,
// ("", false, nil) if it does not, or ("", false, error) on any other failure.
func imageReference(ctx context.Context, registryImageTag string) (string, bool, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return "", false, fmt.Errorf("error load cluster config: %v", err)
	}

	imgClient, err := imagev1client.NewForConfig(cfg)
	if err != nil {
		return "", false, fmt.Errorf("error create image client: %v", err)
	}

	ist, err := imgClient.ImageStreamTags(settings.Settings.Namespace).Get(ctx, registryImageTag, metav1.GetOptions{})
	if kerrors.IsNotFound(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("error: %w. could not get ImageStreamTag: %s", err, registryImageTag)
	}
	return ist.Image.DockerImageReference, true, nil
}

// watchImageURL upgrades the HTTP connection to a WebSocket and streams
// build progress (and final image URL) for the given build-name query parameter.
func (h *VddkHandler) watchImageURL(ctx *gin.Context) {
	buildName := ctx.Query("build-name")
	if buildName == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"status": gin.H{
				"phase":   "Failed",
				"message": "build-name parameter is missing",
			},
		})
		return
	}

	upGrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}

	bcClient, err := NewBuildClient()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"status": gin.H{
				"phase":   "Failed",
				"message": err,
			},
		})
		return
	}

	conn, err := upGrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"status": gin.H{
				"phase":   "Failed",
				"message": fmt.Sprintf("error upgrading connection to websocket: %v", err),
			},
		})
		return
	}
	defer func() {
		_ = conn.WriteMessage(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
		)
		_ = conn.Close()
	}()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Request.Context().Done():
			return
		case <-ticker.C:
			buildObj, err := bcClient.BuildV1().
				Builds(settings.Settings.Namespace).
				Get(ctx.Request.Context(), buildName, metav1.GetOptions{})
			if err != nil {
				_ = conn.WriteJSON(gin.H{
					"status": gin.H{
						"phase":   "Failed",
						"message": err.Error(),
					},
				})
				return
			}

			if isTerminal(buildObj.Status.Phase) {
				if buildObj.Status.Phase != buildv1.BuildPhaseComplete {
					_ = conn.WriteJSON(gin.H{
						"status": gin.H{
							"phase":   "Failed",
							"message": "build failed or cancelled",
						},
					})
					return
				}
				sendImageResult(ctx.Request.Context(), conn, registryImageTag)
				return
			}

			_ = conn.WriteJSON(gin.H{
				"status": gin.H{
					"phase":   buildObj.Status.Phase,
					"message": buildObj.Status.Message,
				},
			})

		}
	}
}

// sendImageResult fetches the final image URL and writes the appropriate JSON frame.
func sendImageResult(ctx context.Context, conn *websocket.Conn, tag string) {
	url, exist, err := imageReference(ctx, tag)
	if err != nil {
		_ = conn.WriteJSON(gin.H{
			"status": gin.H{
				"phase":   "Failed",
				"message": err.Error(),
			},
		})
		return
	}
	if exist {
		_ = conn.WriteJSON(gin.H{
			"status": gin.H{
				"phase":   buildv1.BuildPhaseComplete,
				"message": fmt.Sprintf("Image: %s exists", tag),
			},
			"imageUrl": url,
		})
		return
	}
	_ = conn.WriteJSON(gin.H{
		"status": gin.H{
			"phase":   "Failed",
			"message": "Not found",
		},
	})
}

// isTerminal returns true if the BuildPhase is one of Complete, Failed, Error, or Cancelled.
func isTerminal(phase buildv1.BuildPhase) bool {
	switch phase {
	case buildv1.BuildPhaseComplete, buildv1.BuildPhaseFailed,
		buildv1.BuildPhaseError, buildv1.BuildPhaseCancelled:
		return true
	}
	return false
}

// NewBuildClient loads the in-cluster Kubernetes configuration and
// returns an OpenShift Build API clientset based on that config.
func NewBuildClient() (*buildclientset.Clientset, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("could not load in-cluster config: %w", err)
	}

	buildClient, err := buildclientset.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("could not create build client: %w", err)
	}

	return buildClient, nil
}

// cleanOldFiles Removes the old files older than period within the given directory
func cleanOldFiles(dir string, period time.Duration) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			return err
		}

		if info.ModTime().Before(time.Now().Add(-period)) {
			if err := os.Remove(path); err != nil {
				return err
			}
		}
	}

	return nil
}
