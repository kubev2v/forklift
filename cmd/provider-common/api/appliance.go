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
	"github.com/kubev2v/forklift/cmd/provider-common/auth"
	"github.com/kubev2v/forklift/cmd/provider-common/ovf"
)

const (
	AppliancesRoute = "/appliances"
	ApplianceRoute  = AppliancesRoute + "/:" + Filename
	Filename        = "filename"
	DirectoryPrefix = "appliance-"
	ApplianceField  = "file"
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
	StoragePath  string
	AuthRequired bool
	Auth         *auth.ProviderAuth
	// FileExtension is the expected file extension (.ova or .ovf)
	FileExtension string
}

// AddRoutes adds appliance management routes to a gin router.
func (h ApplianceHandler) AddRoutes(e *gin.Engine) {
	router := e.Group("/")
	router.GET(AppliancesRoute, h.List)
	router.POST(AppliancesRoute, h.Upload)
	// leave the delete endpoint disabled for now
	// router.DELETE(ApplianceRoute, h.Delete)
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
		ctx.AbortWithStatus(http.StatusForbidden)
		return
	}
	entries, err := os.ReadDir(h.StoragePath)
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	appliances := make([]*ApplianceInfo, 0)
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), DirectoryPrefix) {
			continue
		}
		dirPath := pathlib.Join(h.StoragePath, entry.Name())
		dirEntries, dErr := os.ReadDir(dirPath)
		if dErr != nil {
			log.Error(dErr, "couldn't read directory", "dir", dirPath)
			continue
		}
		for _, dirEntry := range dirEntries {
			if dirEntry.IsDir() {
				continue
			}
			if !strings.HasSuffix(strings.ToLower(dirEntry.Name()), h.FileExtension) {
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
// @summary Accepts upload of an appliance to the catalog.
// @description Accepts upload of an appliance to the catalog.
// @tags appliances
// @success 200 {object} ApplianceInfo
// @router /appliances [post]
func (h ApplianceHandler) Upload(ctx *gin.Context) {
	if !h.permitted(ctx) {
		ctx.AbortWithStatus(http.StatusForbidden)
		return
	}
	err := h.writable()
	if err != nil {
		err = &BadRequestError{err.Error()}
		_ = ctx.Error(err)
		return
	}
	input, err := ctx.FormFile(ApplianceField)
	if err != nil {
		err = &BadRequestError{err.Error()}
		_ = ctx.Error(err)
		return
	}
	filename := pathlib.Base(input.Filename)
	if !strings.HasSuffix(strings.ToLower(filename), h.FileExtension) {
		err = &BadRequestError{fmt.Sprintf("filename must end with %s extension", h.FileExtension)}
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
		_ = os.RemoveAll(pathlib.Dir(path))
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
// @summary Deletes an appliance from the catalog.
// @description Deletes an appliance from the catalog.
// @tags appliances
// @success 204
// @router /appliances/{filename} [delete]
// @param filename path string true "Filename of appliance in catalog"
func (h ApplianceHandler) Delete(ctx *gin.Context) {
	if !h.permitted(ctx) {
		ctx.AbortWithStatus(http.StatusForbidden)
		return
	}
	filename := pathlib.Base(ctx.Param(Filename))
	if !strings.HasSuffix(strings.ToLower(filename), h.FileExtension) {
		err := &BadRequestError{fmt.Sprintf("filename must end with %s extension", h.FileExtension)}
		_ = ctx.Error(err)
		return
	}
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
	var envelope *ovf.Envelope
	var err error

	path := h.fullPath(info.Name())

	// Use the correct function based on file extension
	if h.FileExtension == ovf.ExtOVA {
		envelope, err = ovf.ExtractEnvelope(path) // For .ova (tar archive)
	} else {
		envelope, err = ovf.ReadEnvelope(path) // For .ovf (XML file)
	}
	if err != nil {
		appliance.Error = err.Error()
		return
	}
	appliance.Source = ovf.GuessSource(*envelope)
	for _, vs := range envelope.VirtualSystem {
		appliance.VirtualSystems = append(appliance.VirtualSystems, Ref{Name: vs.Name, ID: vs.ID})
	}
	return
}

func (h ApplianceHandler) fullPath(filename string) string {
	return pathlib.Join(
		h.StoragePath,
		fmt.Sprintf("%s%s", DirectoryPrefix, string2hash(filename)),
		filename)
}

func (h ApplianceHandler) writable() error {
	check := pathlib.Join(h.StoragePath, ".writeable")
	f, err := os.OpenFile(check, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil
		}
		return err
	}
	_ = f.Close()
	_ = os.Remove(check)
	return nil
}

func string2hash(s string) string {
	h := sha256.New()
	_, _ = h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}
