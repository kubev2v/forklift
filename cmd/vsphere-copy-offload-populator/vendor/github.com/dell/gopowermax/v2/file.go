package pmax

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	types "github.com/dell/gopowermax/v2/types/v100"
	log "github.com/sirupsen/logrus"
)

// constants to be used in APIs
const (
	XFile          = "file/"
	XFileSystem    = "/file_system"
	XNFSExport     = "/nfs_export"
	XNASServer     = "/nas_server"
	XFileInterface = "/file_interface"
)

// GetFileSystemList get file system list on a symID
func (c *Client) GetFileSystemList(ctx context.Context, symID string, query types.QueryParams) (*types.FileSystemIterator, error) {
	defer c.TimeSpent("GetFileSystemList", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	URL := c.urlPrefix() + XFile + SymmetrixX + symID + XFileSystem
	if len(query) > 0 {
		URL = fmt.Sprintf("%s?", URL)
		for key, value := range query {
			URL = fmt.Sprintf("%s%s=%s&", URL, key, value)
		}
		URL = URL[:len(URL)-1]
	}
	resp, err := c.api.DoAndGetResponseBody(ctx, http.MethodGet, URL, c.getDefaultHeaders(), nil)
	if err != nil {
		log.Error("GetFileSystemList failed: " + err.Error())
		return nil, err
	}

	if err = c.checkResponse(resp); err != nil {
		return nil, err
	}

	fileSystemIter := new(types.FileSystemIterator)
	if err := json.NewDecoder(resp.Body).Decode(fileSystemIter); err != nil {
		return nil, err
	}
	err = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	return fileSystemIter, nil
}

// GetFileSystemByID get file system  on a symID
func (c *Client) GetFileSystemByID(ctx context.Context, symID, fsID string) (*types.FileSystem, error) {
	defer c.TimeSpent("GetFileSystemByID", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()

	URL := c.urlPrefix() + XFile + SymmetrixX + symID + XFileSystem + "/" + fsID
	resp, err := c.api.DoAndGetResponseBody(ctx, http.MethodGet, URL, c.getDefaultHeaders(), nil)
	if err != nil {
		log.Error("GetFileSystemByID failed: " + err.Error())
		return nil, err
	}

	if err = c.checkResponse(resp); err != nil {
		return nil, err
	}

	fileSystem := new(types.FileSystem)
	if err := json.NewDecoder(resp.Body).Decode(fileSystem); err != nil {
		return nil, err
	}
	err = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	return fileSystem, nil
}

// CreateFileSystem creates a file system
func (c *Client) CreateFileSystem(ctx context.Context, symID, name, nasServer, serviceLevel string, sizeInMiB int64) (*types.FileSystem, error) {
	defer c.TimeSpent("CreateFileSystem", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}
	createFSPayload := types.CreateFileSystem{
		Name:         name,
		SizeTotal:    sizeInMiB,
		NasServer:    nasServer,
		ServiceLevel: serviceLevel,
	}
	Debug = true
	ifDebugLogPayload(createFSPayload)
	URL := c.urlPrefix() + XFile + SymmetrixX + symID + XFileSystem

	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	resp, err := c.api.DoAndGetResponseBody(
		ctx, http.MethodPost, URL, c.getDefaultHeaders(), createFSPayload)
	if err = c.checkResponse(resp); err != nil {
		return nil, err
	}

	fileSystem := &types.FileSystem{}
	decoder := json.NewDecoder(resp.Body)
	if err = decoder.Decode(fileSystem); err != nil {
		return nil, err
	}
	log.Infof("Successfully created file system for %s", fileSystem.Name)
	err = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	return fileSystem, nil
}

// ModifyFileSystem modifies the given file system
func (c *Client) ModifyFileSystem(ctx context.Context, symID, fsID string, payload types.ModifyFileSystem) (*types.FileSystem, error) {
	defer c.TimeSpent("ModifyFileSystem", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}
	ifDebugLogPayload(payload)
	URL := c.urlPrefix() + XFile + SymmetrixX + symID + XFileSystem + "/" + fsID
	fields := map[string]interface{}{
		http.MethodPut: URL,
		"fsID":         fsID,
		"payload":      payload,
	}
	log.WithFields(fields).Info("Modifying FileSystem")
	updatedFileSystem := &types.FileSystem{}
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	err := c.api.Put(
		ctx, URL, c.getDefaultHeaders(), payload, updatedFileSystem)
	if err != nil {
		log.WithFields(fields).Error("Error in ModifyFileSystem: " + err.Error())
		return nil, err
	}
	log.Infof("Successfully updated file system: %s", updatedFileSystem.Name)
	return updatedFileSystem, nil
}

// DeleteFileSystem deletes a file system
func (c *Client) DeleteFileSystem(ctx context.Context, symID, fsID string) error {
	defer c.TimeSpent("DeleteFileSystem", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return err
	}
	URL := c.urlPrefix() + XFile + SymmetrixX + symID + XFileSystem + "/" + fsID
	fields := map[string]interface{}{
		http.MethodDelete: URL,
		"FileSystemID":    fsID,
	}
	log.WithFields(fields).Info("Deleting FileSystem")
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	err := c.api.Delete(ctx, URL, c.getDefaultHeaders(), nil)
	if err != nil {
		log.WithFields(fields).Error("Error in Deleting FileSystem: " + err.Error())
	} else {
		log.Infof("Successfully deleted FileSystem: %s", fsID)
	}
	return err
}

// GetNFSExportList get NFS export list on a symID
func (c *Client) GetNFSExportList(ctx context.Context, symID string, query types.QueryParams) (*types.NFSExportIterator, error) {
	defer c.TimeSpent("GetNFSExportList", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	URL := c.urlPrefix() + XFile + SymmetrixX + symID + XNFSExport
	if len(query) > 0 {
		URL = fmt.Sprintf("%s?", URL)
		for key, value := range query {
			URL = fmt.Sprintf("%s%s=%s&", URL, key, value)
		}
		URL = URL[:len(URL)-1]
	}
	resp, err := c.api.DoAndGetResponseBody(ctx, http.MethodGet, URL, c.getDefaultHeaders(), nil)
	if err != nil {
		log.Error("GetNFSExportList failed: " + err.Error())
		return nil, err
	}

	if err = c.checkResponse(resp); err != nil {
		return nil, err
	}

	nfsExportIter := new(types.NFSExportIterator)
	if err := json.NewDecoder(resp.Body).Decode(nfsExportIter); err != nil {
		return nil, err
	}
	err = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	return nfsExportIter, nil
}

// GetNFSExportByID get file system  on a symID
func (c *Client) GetNFSExportByID(ctx context.Context, symID, nfsExportID string) (*types.NFSExport, error) {
	defer c.TimeSpent("GetNFSExportByID", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()

	URL := c.urlPrefix() + XFile + SymmetrixX + symID + XNFSExport + "/" + nfsExportID
	resp, err := c.api.DoAndGetResponseBody(ctx, http.MethodGet, URL, c.getDefaultHeaders(), nil)
	if err != nil {
		log.Error("GetNFSExportByID failed: " + err.Error())
		return nil, err
	}

	if err = c.checkResponse(resp); err != nil {
		return nil, err
	}

	nfsExport := new(types.NFSExport)
	if err := json.NewDecoder(resp.Body).Decode(nfsExport); err != nil {
		return nil, err
	}
	err = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	return nfsExport, nil
}

// CreateNFSExport creates a NFSExport
func (c *Client) CreateNFSExport(ctx context.Context, symID string, createNFSExportPayload types.CreateNFSExport) (*types.NFSExport, error) {
	defer c.TimeSpent("CreateNFSExport", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}
	Debug = true
	ifDebugLogPayload(createNFSExportPayload)
	URL := c.urlPrefix() + XFile + SymmetrixX + symID + XNFSExport

	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	resp, err := c.api.DoAndGetResponseBody(
		ctx, http.MethodPost, URL, c.getDefaultHeaders(), createNFSExportPayload)
	if err = c.checkResponse(resp); err != nil {
		return nil, err
	}

	nfsExport := &types.NFSExport{}
	decoder := json.NewDecoder(resp.Body)
	if err = decoder.Decode(nfsExport); err != nil {
		return nil, err
	}
	log.Infof("Successfully created nfs export for %s", nfsExport.Name)
	err = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	return nfsExport, nil
}

// ModifyNFSExport updates a NFS export
func (c *Client) ModifyNFSExport(ctx context.Context, symID, nfsExportID string, payload types.ModifyNFSExport) (*types.NFSExport, error) {
	defer c.TimeSpent("ModifyNFSExport", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}
	ifDebugLogPayload(payload)
	URL := c.urlPrefix() + XFile + SymmetrixX + symID + XNFSExport + "/" + nfsExportID
	fields := map[string]interface{}{
		http.MethodPut: URL,
		"nfsExportID":  nfsExportID,
		"payload":      payload,
	}
	log.WithFields(fields).Info("Modifying NFS Export")
	updatedNFSExport := &types.NFSExport{}
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	err := c.api.Put(
		ctx, URL, c.getDefaultHeaders(), payload, updatedNFSExport)
	if err != nil {
		log.WithFields(fields).Error("Error in ModifyNFSExport: " + err.Error())
		return nil, err
	}
	log.Infof("Successfully modified NFS export: %s", updatedNFSExport.Name)
	return updatedNFSExport, nil
}

// DeleteNFSExport deletes a nfs export
func (c *Client) DeleteNFSExport(ctx context.Context, symID, nfsExportID string) error {
	defer c.TimeSpent("DeleteNFSExport", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return err
	}
	URL := c.urlPrefix() + XFile + SymmetrixX + symID + XNFSExport + "/" + nfsExportID
	fields := map[string]interface{}{
		http.MethodDelete: URL,
		"nfsExportID":     nfsExportID,
	}
	log.WithFields(fields).Info("Deleting NFSExport")
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	err := c.api.Delete(ctx, URL, c.getDefaultHeaders(), nil)
	if err != nil {
		log.WithFields(fields).Error("Error in Deleting NFSExport: " + err.Error())
	} else {
		log.Infof("Successfully deleted NFSExport: %s", nfsExportID)
	}
	return err
}

// GetNASServerList get NAS Server list on a symID
func (c *Client) GetNASServerList(ctx context.Context, symID string, query types.QueryParams) (*types.NASServerIterator, error) {
	defer c.TimeSpent("GetNASServerList", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	URL := c.urlPrefix() + XFile + SymmetrixX + symID + XNASServer
	if len(query) > 0 {
		URL = fmt.Sprintf("%s?", URL)
		for key, value := range query {
			URL = fmt.Sprintf("%s%s=%s&", URL, key, value)
		}
		URL = URL[:len(URL)-1]
	}
	resp, err := c.api.DoAndGetResponseBody(ctx, http.MethodGet, URL, c.getDefaultHeaders(), nil)
	if err != nil {
		log.Error("GetNASServerList failed: " + err.Error())
		return nil, err
	}

	if err = c.checkResponse(resp); err != nil {
		return nil, err
	}

	nasServerList := new(types.NASServerIterator)
	if err := json.NewDecoder(resp.Body).Decode(nasServerList); err != nil {
		return nil, err
	}
	err = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	return nasServerList, nil
}

// GetNASServerByID fetch specific NAS server on a symID
func (c *Client) GetNASServerByID(ctx context.Context, symID, nasID string) (*types.NASServer, error) {
	defer c.TimeSpent("GetNASServerByID", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()

	URL := c.urlPrefix() + XFile + SymmetrixX + symID + XNASServer + "/" + nasID
	resp, err := c.api.DoAndGetResponseBody(ctx, http.MethodGet, URL, c.getDefaultHeaders(), nil)
	if err != nil {
		log.Error("GetNASServerByID failed: " + err.Error())
		return nil, err
	}

	if err = c.checkResponse(resp); err != nil {
		return nil, err
	}

	nasServer := new(types.NASServer)
	if err := json.NewDecoder(resp.Body).Decode(nasServer); err != nil {
		return nil, err
	}
	err = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	return nasServer, nil
}

// ModifyNASServer updates a NAS Server
func (c *Client) ModifyNASServer(ctx context.Context, symID, nasID string, payload types.ModifyNASServer) (*types.NASServer, error) {
	defer c.TimeSpent("ModifyNASServer", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}
	ifDebugLogPayload(payload)
	URL := c.urlPrefix() + XFile + SymmetrixX + symID + XNASServer + "/" + nasID
	fields := map[string]interface{}{
		http.MethodPut: URL,
		"nasID":        nasID,
		"payload":      payload,
	}
	log.WithFields(fields).Info("Modifying NAS Server")
	updatedNASServer := &types.NASServer{}
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	err := c.api.Put(
		ctx, URL, c.getDefaultHeaders(), payload, updatedNASServer)
	if err != nil {
		log.WithFields(fields).Error("Error in ModifyNASServer: " + err.Error())
		return nil, err
	}
	log.Infof("Successfully modified NFS export: %s", updatedNASServer.Name)
	return updatedNASServer, nil
}

// DeleteNASServer deletes a nas server
func (c *Client) DeleteNASServer(ctx context.Context, symID, nasID string) error {
	defer c.TimeSpent("DeleteNASServer", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return err
	}
	URL := c.urlPrefix() + XFile + SymmetrixX + symID + XNASServer + "/" + nasID
	fields := map[string]interface{}{
		http.MethodDelete: URL,
		"nasID":           nasID,
	}
	log.WithFields(fields).Info("Deleting NAS Server")
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	err := c.api.Delete(ctx, URL, c.getDefaultHeaders(), nil)
	if err != nil {
		log.WithFields(fields).Error("Error in Deleting NAS Server: " + err.Error())
	} else {
		log.Infof("Successfully deleted NAS Server: %s", nasID)
	}
	return err
}

// GetFileInterfaceByID get file system  on a symID
func (c *Client) GetFileInterfaceByID(ctx context.Context, symID, interfaceID string) (*types.FileInterface, error) {
	defer c.TimeSpent("GetNFSExportByID", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()

	URL := c.urlPrefix() + XFile + SymmetrixX + symID + XFileInterface + "/" + interfaceID
	resp, err := c.api.DoAndGetResponseBody(ctx, http.MethodGet, URL, c.getDefaultHeaders(), nil)
	if err != nil {
		log.Error("GetFileInterfaceByID failed: " + err.Error())
		return nil, err
	}

	if err = c.checkResponse(resp); err != nil {
		return nil, err
	}

	fileInterface := new(types.FileInterface)
	if err := json.NewDecoder(resp.Body).Decode(fileInterface); err != nil {
		return nil, err
	}
	err = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	return fileInterface, nil
}
