package api

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	pathlib "path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kubev2v/forklift/cmd/ova-provider-server/auth"
	"github.com/kubev2v/forklift/cmd/ova-provider-server/ova"
	"github.com/kubev2v/forklift/cmd/ova-provider-server/settings"
)

var Settings = &settings.Settings

const (
	AppliancesRoute = "/appliances"
	ApplianceRoute  = AppliancesRoute + "/:" + Filename
	Filename        = "filename"
	DirectoryPrefix = "appliance-"
	ApplianceField  = "appliance"
)

// ApplianceInfo JSON resource
type ApplianceInfo struct {
	File           string     `json:"file"`
	Size           int64      `json:"size,omitempty"`
	Modified       *time.Time `json:"modified,omitempty"`
	Error          string     `json:"error,omitempty"`
	Source         string     `json:"source,omitempty"`
	VirtualSystems []Ref      `json:"virtualSystems"`
}

func (r *ApplianceInfo) OK() bool {
	return r.Error == ""
}

type Ref struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

// ApplianceHandler serves appliance management routes.
type ApplianceHandler struct {
	OVAStoragePath string
	AuthRequired   bool
	Auth           *auth.ProviderAuth
}

// AddRoutes adds appliance management routes to a gin router.
func (h ApplianceHandler) AddRoutes(e *gin.Engine) {
	router := e.Group("/")
	router.GET(AppliancesRoute, h.List)
	router.POST(AppliancesRoute, h.Upload)
	router.DELETE(ApplianceRoute, h.Delete)
}

// List godoc
// @summary Lists the appliances that are present in the catalog.
// @description Lists the appliances that are present in the catalog.
// @tags appliances
// @produce json
// @success 200 {array} ApplianceInfo
// @router /appliances [get]
func (h ApplianceHandler) List(ctx *gin.Context) {
	if !h.permitted(ctx) {
		ctx.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	entries, err := os.ReadDir(h.OVAStoragePath)
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	appliances := make([]*ApplianceInfo, 0)
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), DirectoryPrefix) {
			continue
		}
		dirPath := pathlib.Join(h.OVAStoragePath, entry.Name())
		dirEntries, dErr := os.ReadDir(dirPath)
		if dErr != nil {
			_ = ctx.Error(dErr)
			return
		}
		for _, dirEntry := range dirEntries {
			if dirEntry.IsDir() {
				continue
			}
			if !strings.HasSuffix(strings.ToLower(dirEntry.Name()), ova.ExtOVA) {
				continue
			}
			info, fErr := dirEntry.Info()
			if fErr != nil {
				continue
			}
			appliance := h.applianceInfo(info)
			appliances = append(appliances, appliance)
		}
	}
	ctx.JSON(http.StatusOK, appliances)
}

// Upload godoc
// @summary Accepts upload of an OVA to the catalog.
// @description Accepts upload of an OVA to the catalog.
// @tags appliances
// @success 200 {object} ApplianceInfo
// @router /appliances [post]
func (h ApplianceHandler) Upload(ctx *gin.Context) {
	if !h.permitted(ctx) {
		ctx.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	input, err := ctx.FormFile(ApplianceField)
	if err != nil {
		err = &BadRequestError{err.Error()}
		_ = ctx.Error(err)
		return
	}
	filename := pathlib.Base(input.Filename)
	if !strings.HasSuffix(strings.ToLower(filename), ova.ExtOVA) {
		err = &BadRequestError{"filename must end with .ova extension"}
		_ = ctx.Error(err)
		return
	}
	path := h.fullPath(filename)
	_, err = os.Stat(path)
	if err == nil {
		err = &ConflictError{"a file by that name already exists"}
		_ = ctx.Error(err)
		return
	} else {
		if errors.Is(err, os.ErrNotExist) {
			err = nil
		} else {
			_ = ctx.Error(err)
			return
		}
	}
	src, err := input.Open()
	if err != nil {
		err = &BadRequestError{err.Error()}
		_ = ctx.Error(err)
		return
	}
	defer func() {
		_ = src.Close()
	}()
	err = os.MkdirAll(pathlib.Dir(path), 0750)
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	err = h.upload(src, path)
	if err != nil {
		log.Error(err, "failed uploading file")
		_ = ctx.Error(err)
		return
	}
	// remove a file that doesn't appear to contain
	// a valid appliance and return an error.
	appliance, err := h.validate(path)
	if err != nil {
		log.Error(err, "failed statting file")
		_ = os.RemoveAll(pathlib.Dir(path))
		_ = ctx.Error(err)
		return
	}
	if !appliance.OK() {
		_ = os.RemoveAll(pathlib.Dir(path))
		_ = ctx.Error(&BadRequestError{appliance.Error})
		return
	}
	ctx.JSON(http.StatusOK, appliance)
}

func (h ApplianceHandler) upload(src io.Reader, path string) (err error) {
	dst, err := os.Create(path)
	if err != nil {
		return
	}
	defer func() {
		_ = dst.Close()
	}()
	_, err = io.Copy(dst, src)
	if err != nil {
		return
	}
	err = os.Chmod(path, 0640)
	if err != nil {
		return
	}
	return
}

func (h ApplianceHandler) validate(path string) (appliance *ApplianceInfo, err error) {
	info, err := os.Stat(path)
	if err != nil {
		return
	}
	appliance = h.applianceInfo(info)
	return
}

// Delete godoc
// @summary Deletes an OVA from the catalog.
// @description Deletes an OVA from the catalog.
// @tags appliances
// @success 204
// @router /appliances/{filename} [delete]
// @param filename path string true "Filename of OVA in catalog"
func (h ApplianceHandler) Delete(ctx *gin.Context) {
	if !h.permitted(ctx) {
		ctx.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	filename := pathlib.Base(ctx.Param(Filename))
	path := h.fullPath(filename)
	_, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			ctx.Status(http.StatusNoContent)
			return
		} else {
			_ = ctx.Error(err)
			return
		}
	}
	err = os.RemoveAll(pathlib.Dir(path))
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

func (h ApplianceHandler) permitted(ctx *gin.Context) bool {
	if !h.AuthRequired {
		return true
	}
	return h.Auth.Permit(ctx)
}

func (h ApplianceHandler) applianceInfo(info os.FileInfo) (appliance *ApplianceInfo) {
	modTime := info.ModTime()
	appliance = &ApplianceInfo{
		File:     info.Name(),
		Modified: &modTime,
		Size:     info.Size(),
	}
	envelope, err := ova.ExtractEnvelope(h.fullPath(info.Name()))
	if err != nil {
		appliance.Error = err.Error()
		return
	}
	appliance.Source = ova.GuessSource(*envelope)
	for _, vs := range envelope.VirtualSystem {
		appliance.VirtualSystems = append(appliance.VirtualSystems, Ref{Name: vs.Name, ID: vs.ID})
	}
	return
}

func (h ApplianceHandler) fullPath(filename string) string {
	return pathlib.Join(
		h.OVAStoragePath,
		fmt.Sprintf("%s%s", DirectoryPrefix, string2hash(filename)),
		filename)
}

func string2hash(s string) string {
	h := sha256.New()
	_, _ = h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}
