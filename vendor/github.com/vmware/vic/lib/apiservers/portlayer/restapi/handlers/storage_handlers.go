// Copyright 2016-2017 VMware, Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package handlers

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"

	log "github.com/Sirupsen/logrus"
	"github.com/go-openapi/runtime/middleware"

	"github.com/vmware/vic/lib/apiservers/portlayer/models"
	"github.com/vmware/vic/lib/apiservers/portlayer/restapi/operations"
	"github.com/vmware/vic/lib/apiservers/portlayer/restapi/operations/storage"
	"github.com/vmware/vic/lib/archive"
	epl "github.com/vmware/vic/lib/portlayer/exec"
	spl "github.com/vmware/vic/lib/portlayer/storage"
	"github.com/vmware/vic/lib/portlayer/storage/container"
	"github.com/vmware/vic/lib/portlayer/storage/image"
	vsimage "github.com/vmware/vic/lib/portlayer/storage/image/vsphere"
	"github.com/vmware/vic/lib/portlayer/storage/volume"
	"github.com/vmware/vic/lib/portlayer/storage/volume/nfs"
	vsvolume "github.com/vmware/vic/lib/portlayer/storage/volume/vsphere"
	"github.com/vmware/vic/lib/portlayer/storage/vsphere"
	"github.com/vmware/vic/lib/portlayer/util"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/datastore"
)

// StorageHandlersImpl is the receiver for all of the storage handler methods
type StorageHandlersImpl struct {
	imageCache  *image.NameLookupCache
	volumeCache *volume.VolumeLookupCache
}

const (
	nfsScheme = "nfs"
	dsScheme  = "ds"

	uidQueryKey = "uid"
	gidQueryKey = "gid"
)

// Configure assigns functions to all the storage api handlers
func (h *StorageHandlersImpl) Configure(api *operations.PortLayerAPI, handlerCtx *HandlerContext) {
	var err error

	op := trace.NewOperation(context.Background(), "configure storage layer")

	if len(spl.Config.ImageStores) == 0 {
		op.Panicf("No image stores provided; unable to instantiate storage layer")
	}

	imageStoreURL := spl.Config.ImageStores[0]
	// TODO: support multiple image stores. Right now we only support the first one
	if len(spl.Config.ImageStores) > 1 {
		op.Warnf("Multiple image stores found. Multiple image stores are not yet supported. Using [%s] %s", imageStoreURL.Host, imageStoreURL.Path)
	}

	imageStore, err := vsimage.NewImageStore(op, handlerCtx.Session, &imageStoreURL)
	if err != nil {
		op.Panicf("Cannot instantiate storage layer: %s", err)
	}

	// The imagestore is implemented via a cache which is backed via an
	// implementation that writes to disks.  The cache is used to avoid
	// expensive metadata lookups.
	h.imageCache = image.NewLookupCache(imageStore)

	spl.RegisterImporter(op, imageStoreURL.String(), imageStore)
	spl.RegisterExporter(op, imageStoreURL.String(), imageStore)

	containerStore, err := container.NewContainerStore(op, handlerCtx.Session, h.imageCache)
	if err != nil {
		op.Panicf("Couldn't create container store: %s", err.Error())
	}

	spl.RegisterImporter(op, "container", containerStore)
	spl.RegisterExporter(op, "container", containerStore)

	// add the volume stores, errors are logged within this function.
	h.configureVolumeStores(op, handlerCtx)

	api.StorageCreateImageStoreHandler = storage.CreateImageStoreHandlerFunc(h.CreateImageStore)
	api.StorageGetImageHandler = storage.GetImageHandlerFunc(h.GetImage)
	api.StorageListImagesHandler = storage.ListImagesHandlerFunc(h.ListImages)
	api.StorageWriteImageHandler = storage.WriteImageHandlerFunc(h.WriteImage)
	api.StorageImageJoinHandler = storage.ImageJoinHandlerFunc(h.ImageJoin)
	api.StorageDeleteImageHandler = storage.DeleteImageHandlerFunc(h.DeleteImage)

	api.StorageVolumeStoresListHandler = storage.VolumeStoresListHandlerFunc(h.VolumeStoresList)
	api.StorageCreateVolumeHandler = storage.CreateVolumeHandlerFunc(h.CreateVolume)
	api.StorageRemoveVolumeHandler = storage.RemoveVolumeHandlerFunc(h.RemoveVolume)
	api.StorageVolumeJoinHandler = storage.VolumeJoinHandlerFunc(h.VolumeJoin)
	api.StorageListVolumesHandler = storage.ListVolumesHandlerFunc(h.VolumesList)
	api.StorageGetVolumeHandler = storage.GetVolumeHandlerFunc(h.GetVolume)

	api.StorageExportArchiveHandler = storage.ExportArchiveHandlerFunc(h.ExportArchive)
	api.StorageImportArchiveHandler = storage.ImportArchiveHandlerFunc(h.ImportArchive)
	api.StorageStatPathHandler = storage.StatPathHandlerFunc(h.StatPath)
}

func (h *StorageHandlersImpl) configureVolumeStores(op trace.Operation, handlerCtx *HandlerContext) {
	var (
		vs  volume.VolumeStorer
		err error
	)

	h.volumeCache = volume.NewVolumeLookupCache(op)

	// register the pseudo-store to handle the generic "volume" store name
	spl.RegisterImporter(op, "volume", h.volumeCache)
	spl.RegisterExporter(op, "volume", h.volumeCache)

	// Configure the datastores
	// Each volume store name maps to a datastore + path, which can be referred to by the name.
	for name, dsurl := range spl.Config.VolumeLocations {
		switch dsurl.Scheme {
		case nfsScheme:
			vs, err = createNFSVolumeStore(op, dsurl, name)
		case dsScheme:
			vs, err = createVsphereVolumeStore(op, dsurl, name, handlerCtx)
		default:
			err = fmt.Errorf("unknown scheme for %s", dsurl.String())
			op.Error(err)
		}

		// if an error has been logged skip volume store cache addition
		if err != nil {
			continue
		}

		op.Infof("Adding volume store %s (%s)", name, dsurl.String())
		if _, err = h.volumeCache.AddStore(op, name, vs); err != nil {
			op.Errorf("volume addition error %s", err)
		}

		spl.RegisterImporter(op, dsurl.String(), vs)
		spl.RegisterExporter(op, dsurl.String(), vs)

		// get the mangled store URLs that the cache uses
		// #nosec: Errors unhandled.
		cURL, _ := h.volumeCache.GetVolumeStore(op, name)
		if cURL != nil {
			spl.RegisterImporter(op, cURL.String(), vs)
			spl.RegisterExporter(op, cURL.String(), vs)
		}
	}
}

// CreateImageStore creates a new image store
func (h *StorageHandlersImpl) CreateImageStore(params storage.CreateImageStoreParams) middleware.Responder {
	op := trace.NewOperationFromID(context.Background(), params.OpID, "CreateImageStore(%s)", params.Body.Name)
	defer trace.End(trace.Begin("CreateImageStore", op))

	name := params.Body.Name
	defer trace.End(trace.Begin(fmt.Sprintf("CreateImageStore: %s", name), op))

	registerImageStore := func(h *StorageHandlersImpl, name string) {
		// register image store importer/export
		spl.RegisterImporter(op, name, h.imageCache.DataStore)
		spl.RegisterExporter(op, name, h.imageCache.DataStore)

		storeURL, err := util.ImageStoreNameToURL(name)
		if err == nil {
			spl.RegisterImporter(op, storeURL.String(), h.imageCache.DataStore)
			spl.RegisterExporter(op, storeURL.String(), h.imageCache.DataStore)
		}
	}

	url, err := h.imageCache.CreateImageStore(op, name)
	if err != nil {
		if os.IsExist(err) {
			registerImageStore(h, name)
			return storage.NewCreateImageStoreConflict().WithPayload(
				&models.Error{
					Code:    http.StatusConflict,
					Message: "An image store with that name already exists",
				})
		}

		return storage.NewCreateImageStoreDefault(http.StatusInternalServerError).WithPayload(
			&models.Error{
				Code:    http.StatusInternalServerError,
				Message: err.Error(),
			})
	}
	registerImageStore(h, name)

	s := &models.StoreURL{
		Code: http.StatusCreated,
		URL:  url.String(),
	}
	return storage.NewCreateImageStoreCreated().WithPayload(s)
}

// GetImage retrieves an image from a store
func (h *StorageHandlersImpl) GetImage(params storage.GetImageParams) middleware.Responder {
	op := trace.NewOperationFromID(context.Background(), params.OpID, "GetImage(%s)", params.ID)
	defer trace.End(trace.Begin("GetImage", op))

	id := params.ID

	url, err := util.ImageStoreNameToURL(params.StoreName)
	if err != nil {
		return storage.NewGetImageDefault(http.StatusInternalServerError).WithPayload(
			&models.Error{
				Code:    http.StatusInternalServerError,
				Message: err.Error(),
			})
	}

	image, err := h.imageCache.GetImage(op, url, id)
	if err != nil {
		e := &models.Error{
			Code:    http.StatusNotFound,
			Message: err.Error(),
		}
		return storage.NewGetImageNotFound().WithPayload(e)
	}

	result := convertImage(image)
	return storage.NewGetImageOK().WithPayload(result)
}

// DeleteImage deletes an image from a store
func (h *StorageHandlersImpl) DeleteImage(params storage.DeleteImageParams) middleware.Responder {
	op := trace.NewOperationFromID(context.Background(), params.OpID, "DeleteImage(%s)", params.ID)
	defer trace.End(trace.Begin("DeleteImage", op))

	ferr := func(err error, code int) middleware.Responder {
		log.Errorf("DeleteImage: error %s", err.Error())
		return storage.NewDeleteImageDefault(code).WithPayload(
			&models.Error{
				Code:    int64(code),
				Message: err.Error(),
			})
	}

	imageURL, err := util.ImageURL(params.StoreName, params.ID)
	if err != nil {
		return ferr(err, http.StatusInternalServerError)
	}

	img, err := image.Parse(imageURL)
	if err != nil {
		return ferr(err, http.StatusInternalServerError)
	}

	keepNodes := make([]*url.URL, len(params.KeepNodes))
	for idx, kn := range params.KeepNodes {
		k, err := url.Parse(kn)
		if err != nil {
			return ferr(err, http.StatusInternalServerError)
		}

		keepNodes[idx] = k
	}

	deletedImages, err := h.imageCache.DeleteBranch(op, img, keepNodes)
	if err != nil {
		switch {
		case image.IsErrImageInUse(err):
			return ferr(err, http.StatusLocked)

		case os.IsNotExist(err):
			return ferr(err, http.StatusNotFound)

		default:
			return ferr(err, http.StatusInternalServerError)
		}
	}

	result := make([]*models.Image, len(deletedImages))
	for idx, img := range deletedImages {
		result[idx] = convertImage(img)
	}

	return storage.NewDeleteImageOK().WithPayload(result)
}

// ListImages returns a list of images in a store
func (h *StorageHandlersImpl) ListImages(params storage.ListImagesParams) middleware.Responder {
	op := trace.NewOperationFromID(context.Background(), params.OpID, "ListImages(%s, %q)", params.StoreName, params.Ids)
	defer trace.End(trace.Begin("ListImages", op))

	u, err := util.ImageStoreNameToURL(params.StoreName)
	if err != nil {
		return storage.NewListImagesDefault(http.StatusInternalServerError).WithPayload(
			&models.Error{
				Code:    http.StatusInternalServerError,
				Message: err.Error(),
			})
	}

	op.Debugf("URL for image store: %s", u.String())

	images, err := h.imageCache.ListImages(op, u, params.Ids)
	if err != nil {
		return storage.NewListImagesNotFound().WithPayload(
			&models.Error{
				Code:    http.StatusNotFound,
				Message: err.Error(),
			})
	}

	result := make([]*models.Image, 0, len(images))

	for _, image := range images {
		result = append(result, convertImage(image))
	}
	return storage.NewListImagesOK().WithPayload(result)
}

// WriteImage writes an image to an image store
func (h *StorageHandlersImpl) WriteImage(params storage.WriteImageParams) middleware.Responder {
	op := trace.NewOperationFromID(context.Background(), params.OpID, "WriteImage(%s)", params.ImageID)
	defer trace.End(trace.Begin("WriteImage", op))

	u, err := util.ImageStoreNameToURL(params.StoreName)
	if err != nil {
		return storage.NewWriteImageDefault(http.StatusInternalServerError).WithPayload(
			&models.Error{
				Code:    http.StatusInternalServerError,
				Message: err.Error(),
			})
	}

	parent := &image.Image{
		Store: u,
		ID:    params.ParentID,
	}

	var meta map[string][]byte

	if params.Metadatakey != nil && params.Metadataval != nil {
		meta = map[string][]byte{*params.Metadatakey: []byte(*params.Metadataval)}
	}

	image, err := h.imageCache.WriteImage(op, parent, params.ImageID, meta, params.Sum, params.ImageFile)
	if err != nil {
		return storage.NewWriteImageDefault(http.StatusInternalServerError).WithPayload(
			&models.Error{
				Code:    http.StatusInternalServerError,
				Message: err.Error(),
			})
	}
	i := convertImage(image)
	return storage.NewWriteImageCreated().WithPayload(i)
}

//ImageJoin modifies the config spec of a container to include the specified image
func (h *StorageHandlersImpl) ImageJoin(params storage.ImageJoinParams) middleware.Responder {
	op := trace.NewOperation(context.Background(), "ImageJoin %s", params.ID)
	defer trace.End(trace.Begin("", op))

	handle := epl.HandleFromInterface(params.Config.Handle)
	if handle == nil {
		err := &models.Error{Message: "Failed to get the Handle"}
		return storage.NewImageJoinInternalServerError().WithPayload(err)
	}

	storeURL, _ := util.ImageStoreNameToURL(params.StoreName)
	img, err := h.imageCache.GetImage(op, storeURL, params.ID)
	if err != nil {
		op.Errorf("Volumes: StorageHandler : %#v", err)
		return storage.NewImageJoinNotFound().WithPayload(&models.Error{Code: http.StatusNotFound, Message: err.Error()})
	}

	handleprime, err := image.Join(op, handle, params.Config.DeltaID, params.Config.ImageID, params.Config.RepoName, img)
	if err != nil {
		op.Errorf("join image failed: %#v", err)
		return storage.NewImageJoinInternalServerError().WithPayload(&models.Error{Message: err.Error()})
	}

	op.Debugf("image %s has been joined to %s as %s", params.ID, handle.Spec.ID(), params.Config.DeltaID)
	res := &models.ImageJoinResponse{
		Handle: epl.ReferenceFromHandle(handleprime),
	}
	return storage.NewImageJoinOK().WithPayload(res)
}

// VolumeStoresList lists the configured volume stores and their datastore path URIs.
func (h *StorageHandlersImpl) VolumeStoresList(params storage.VolumeStoresListParams) middleware.Responder {
	op := trace.NewOperationFromID(context.Background(), params.OpID, "VolumeStoresList")
	defer trace.End(trace.Begin("VolumeStoresList", op))

	stores, err := h.volumeCache.VolumeStoresList(op)
	if err != nil {
		return storage.NewVolumeStoresListInternalServerError().WithPayload(
			&models.Error{
				Code:    http.StatusInternalServerError,
				Message: err.Error(),
			})
	}

	resp := &models.VolumeStoresListResponse{Stores: stores}

	return storage.NewVolumeStoresListOK().WithPayload(resp)
}

//CreateVolume : Create a Volume
func (h *StorageHandlersImpl) CreateVolume(params storage.CreateVolumeParams) middleware.Responder {
	op := trace.NewOperationFromID(context.Background(), params.OpID, "CreateVolume(%s)", params.VolumeRequest.Name)
	defer trace.End(trace.Begin("CreateVolume", op))

	//TODO: FIXME: add more errorcodes as we identify error scenarios.
	storeURL, err := util.VolumeStoreNameToURL(params.VolumeRequest.Store)
	if err != nil {
		log.Errorf("storagehandler: VolumeStoreName error: %s", err)
		return storage.NewCreateVolumeInternalServerError().WithPayload(&models.Error{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
	}

	byteMap := make(map[string][]byte)
	for key, value := range params.VolumeRequest.Metadata {
		byteMap[key] = []byte(value)
	}

	capacity := uint64(0)
	if params.VolumeRequest.Capacity < 0 {
		capacity = uint64(1024) //FIXME: this should look for a default cap and set or fail here.
	} else {
		capacity = uint64(params.VolumeRequest.Capacity)
	}

	vol, err := h.volumeCache.VolumeCreate(op, params.VolumeRequest.Name, storeURL, capacity*1024, byteMap)
	if err != nil {

		if os.IsExist(err) {
			op.Warnf("Reusing existing volume with target identity")
			return storage.NewCreateVolumeConflict().WithPayload(&models.Error{
				Code:    http.StatusConflict,
				Message: err.Error(),
			})
		}

		op.Errorf("storagehandler: VolumeCreate error: %#v", err)
		if _, ok := err.(volume.VolumeStoreNotFoundError); ok {
			return storage.NewCreateVolumeNotFound().WithPayload(&models.Error{
				Code:    http.StatusNotFound,
				Message: err.Error(),
			})
		}

		return storage.NewCreateVolumeInternalServerError().WithPayload(&models.Error{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
	}

	response := volumeToCreateResponse(vol, params.VolumeRequest)
	return storage.NewCreateVolumeCreated().WithPayload(&response)
}

//GetVolume : Gets a handle to a volume
func (h *StorageHandlersImpl) GetVolume(params storage.GetVolumeParams) middleware.Responder {
	op := trace.NewOperationFromID(context.Background(), params.OpID, "GetVolume(%s)", params.Name)
	defer trace.End(trace.Begin("GetVolume", op))

	data, err := h.volumeCache.VolumeGet(op, params.Name)
	if err == os.ErrNotExist {
		return storage.NewGetVolumeNotFound().WithPayload(&models.Error{
			Code:    http.StatusNotFound,
			Message: err.Error(),
		})
	}

	response, err := fillVolumeModel(data)
	if err != nil {
		return storage.NewListVolumesInternalServerError().WithPayload(&models.Error{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
	}

	op.Debugf("VolumeGet returned : %#v", response)
	return storage.NewGetVolumeOK().WithPayload(&response)
}

//RemoveVolume : Remove a Volume from existence
func (h *StorageHandlersImpl) RemoveVolume(params storage.RemoveVolumeParams) middleware.Responder {
	op := trace.NewOperationFromID(context.Background(), params.OpID, "RemoveVolume(%s)", params.Name)
	defer trace.End(trace.Begin("RemoveVolume", op))

	err := h.volumeCache.VolumeDestroy(op, params.Name)
	if err != nil {
		switch {
		case os.IsNotExist(err):
			return storage.NewRemoveVolumeNotFound().WithPayload(&models.Error{
				Message: err.Error(),
			})

		case volume.IsErrVolumeInUse(err):
			return storage.NewRemoveVolumeConflict().WithPayload(&models.Error{
				Message: err.Error(),
			})

		default:
			return storage.NewRemoveVolumeInternalServerError().WithPayload(&models.Error{
				Message: err.Error(),
			})
		}
	}
	return storage.NewRemoveVolumeOK()
}

//VolumesList : Lists available volumes for use
func (h *StorageHandlersImpl) VolumesList(params storage.ListVolumesParams) middleware.Responder {
	op := trace.NewOperationFromID(context.Background(), params.OpID, "VolumesList")
	defer trace.End(trace.Begin("VolumesList", op))

	var result []*models.VolumeResponse
	portlayerVolumes, err := h.volumeCache.VolumesList(op)
	if err != nil {
		op.Error(err)
		return storage.NewListVolumesInternalServerError().WithPayload(&models.Error{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
	}

	op.Debugf("volumes fetched from list call : %#v", portlayerVolumes)

	for i := range portlayerVolumes {
		model, err := fillVolumeModel(portlayerVolumes[i])
		if err != nil {
			op.Error(err)
			return storage.NewListVolumesInternalServerError().WithPayload(&models.Error{
				Code:    http.StatusInternalServerError,
				Message: err.Error(),
			})
		}

		result = append(result, &model)
	}

	op.Debugf("volumes returned from list call : %#v", result)
	return storage.NewListVolumesOK().WithPayload(result)
}

//VolumeJoin : modifies the config spec of a container to mount the specified container
func (h *StorageHandlersImpl) VolumeJoin(params storage.VolumeJoinParams) middleware.Responder {
	op := trace.NewOperationFromID(context.Background(), params.OpID, "VolumeJoin(%s)", params.Name)
	defer trace.End(trace.Begin("VolumeJoin", op))

	actualHandle := epl.GetHandle(params.JoinArgs.Handle)

	//Note: Name should already be populated by now.
	volume, err := h.volumeCache.VolumeGet(op, params.Name)
	if err != nil {
		op.Errorf("Volumes: StorageHandler : %#v", err)

		return storage.NewVolumeJoinInternalServerError().WithPayload(&models.Error{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
	}

	// NOTE: unclear to me why we are leaking this logic at this level - the volume should be able to switch Join implementations
	// based on its type
	switch volume.Device.DiskPath().Scheme {
	case nfsScheme:
		actualHandle, err = nfs.VolumeJoin(op, actualHandle, volume, params.JoinArgs.MountPath, params.JoinArgs.Flags)
	case dsScheme:
		actualHandle, err = vsvolume.VolumeJoin(op, actualHandle, volume, params.JoinArgs.MountPath, params.JoinArgs.Flags)
	default:
		err = fmt.Errorf("unknown scheme (%s) for Volume (%#v)", volume.Device.DiskPath().Scheme, *volume)
	}

	if err != nil {
		op.Errorf("Volumes: StorageHandler : %#v", err)

		return storage.NewVolumeJoinInternalServerError().WithPayload(&models.Error{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
	}

	op.Infof("volume %s has been joined to a container", volume.ID)
	return storage.NewVolumeJoinOK().WithPayload(actualHandle.String())
}

// ImportArchive takes an input tar archive and unpacks to destination
func (h *StorageHandlersImpl) ImportArchive(params storage.ImportArchiveParams) middleware.Responder {
	op := trace.NewOperationFromID(context.Background(), params.OpID, "ImportArchive(%s)", params.DeviceID)
	defer trace.End(trace.Begin("ImportArchive", op))

	id := params.DeviceID

	filterSpec, err := archive.DecodeFilterSpec(op, params.FilterSpec)
	if err != nil {
		return storage.NewImportArchiveUnprocessableEntity()
	}

	store, ok := spl.GetImporter(params.Store)
	if !ok {
		op.Errorf("Failed to locate import capable store %s", params.Store)
		op.Debugf("Available importers are: %+q", spl.GetImporters())

		return storage.NewImportArchiveNotFound()
	}

	err = store.Import(op, id, filterSpec, params.Archive)
	if err != nil {
		op.Errorf("import failed: %s", err)
		// error checking for no such file/directory
		if os.IsNotExist(err) {
			return storage.NewImportArchiveNotFound()
		}
		// error checking for internal server error from toolbox
		if vsphere.IsToolBoxStateChangeErr(err) {
			return storage.NewImportArchiveConflict()
		}
		return storage.NewExportArchiveInternalServerError()
	}

	return storage.NewImportArchiveOK()
}

// ExportArchive creates a tar archive and returns to caller
func (h *StorageHandlersImpl) ExportArchive(params storage.ExportArchiveParams) middleware.Responder {
	id := params.DeviceID
	ancestor := ""
	if params.Ancestor != nil {
		ancestor = *params.Ancestor
	}

	op := trace.NewOperationFromID(context.Background(), params.OpID, "ExportArchive(%s, %s)", id, ancestor)
	defer trace.End(trace.Begin("ExportArchive", op))

	filterSpec, err := archive.DecodeFilterSpec(op, params.FilterSpec)
	if err != nil {
		return storage.NewExportArchiveUnprocessableEntity()
	}

	store, ok := spl.GetExporter(params.Store)
	if !ok {
		op.Errorf("Failed to locate export capable store %s", params.Store)
		op.Debugf("Available exporters are: %+q", spl.GetExporters())

		return storage.NewExportArchiveNotFound()
	}

	r, err := store.Export(op, id, ancestor, filterSpec, params.Data)
	if err != nil {
		// hickeng: we're in need of typed errors - should check for id not found for 404 return
		op.Errorf("export failed: %s", err)
		if r != nil {
			r.Close()
		}
		return storage.NewExportArchiveInternalServerError()
	}

	return NewStreamOutputHandler("ExportArchive").WithPayload(NewFlushingReader(r), params.DeviceID, func() { r.Close() })
}

// StatPath returns file info on the target path of a container copy
func (h *StorageHandlersImpl) StatPath(params storage.StatPathParams) middleware.Responder {
	op := trace.NewOperationFromID(context.Background(), params.OpID, "StatPath(%s)", params.DeviceID)
	defer trace.End(trace.Begin("StatPath", op))

	filterSpec, err := archive.DecodeFilterSpec(op, params.FilterSpec)
	if err != nil {
		return storage.NewStatPathUnprocessableEntity()
	}

	if len(filterSpec.Inclusions) != 1 {
		return storage.NewStatPathUnprocessableEntity()
	}

	store, ok := spl.GetExporter(params.Store)
	if !ok {
		op.Errorf("Error getting exporter: %s", err.Error())
		return storage.NewStatPathNotFound()
	}

	dataSource, err := store.NewDataSource(op, params.DeviceID)
	if err != nil {
		op.Errorf("Error getting data source: %s", err.Error())
		return storage.NewStatPathInternalServerError()
	}
	defer dataSource.Close()

	fileStat, err := dataSource.Stat(op, filterSpec)
	if err != nil {
		if os.IsNotExist(err) {
			// would like to be able to differentiate between store and files, but....
			op.Debugf("Stat target did not exist: %s", err)
			return storage.NewStatPathNotFound()
		}
		op.Errorf("Error getting datasource stats: %s", err)
		return storage.NewStatPathInternalServerError()
	}

	modTimeBytes, err := fileStat.ModTime.GobEncode()
	if err != nil {
		return storage.NewStatPathUnprocessableEntity()
	}

	op.Debugf("found data successfully")
	return storage.
		NewStatPathOK().
		WithMode(fileStat.Mode).
		WithLinkTarget(fileStat.LinkTarget).
		WithName(fileStat.Name).
		WithSize(fileStat.Size).
		WithModTime(string(modTimeBytes))

}

//utility functions

// convert an SPL Image to a swagger-defined Image
func convertImage(image *image.Image) *models.Image {
	var parent, selfLink string

	// scratch image
	if image.ParentLink != nil {
		parent = image.ParentLink.String()
	}

	if image.SelfLink != nil {
		selfLink = image.SelfLink.String()
	}

	meta := make(map[string]string)
	if image.Metadata != nil {
		for k, v := range image.Metadata {
			meta[k] = string(v)
		}
	}

	return &models.Image{
		ID:       image.ID,
		SelfLink: selfLink,
		Parent:   parent,
		Metadata: meta,
		Store:    image.Store.String(),
	}
}

func volumeToCreateResponse(volume *volume.Volume, model *models.VolumeRequest) models.VolumeResponse {
	response := models.VolumeResponse{
		Driver:   model.Driver,
		Name:     volume.ID,
		Label:    volume.Label,
		Store:    model.Store,
		Metadata: model.Metadata,
	}
	return response
}

func fillVolumeModel(volume *volume.Volume) (models.VolumeResponse, error) {
	storeName, err := util.VolumeStoreName(volume.Store)
	if err != nil {
		return models.VolumeResponse{}, err
	}

	metadata := createMetadataMap(volume)

	model := models.VolumeResponse{
		Name:     volume.ID,
		Driver:   "vsphere",
		Store:    storeName,
		Metadata: metadata,
		Label:    volume.Label,
	}

	return model, nil
}

func createMetadataMap(volume *volume.Volume) map[string]string {
	stringMap := make(map[string]string)
	for k, v := range volume.Info {
		stringMap[k] = string(v)
	}
	return stringMap
}

func createNFSVolumeStore(op trace.Operation, dsurl *url.URL, name string) (volume.VolumeStorer, error) {
	var err error
	uid, gid, err := parseUIDAndGID(dsurl)
	if err != nil {
		op.Errorf("%s", err.Error())
		return nil, err
	}

	// XXX replace with the vch name
	mnt := nfs.NewMount(dsurl, "vic", uint32(uid), uint32(gid))
	vs, err := nfs.NewVolumeStore(op, name, mnt)
	if err != nil {
		op.Errorf("%s", err.Error())
		return nil, err
	}

	return vs, nil
}

func parseUIDAndGID(queryURL *url.URL) (int, int, error) {
	var err error
	uid := nfs.DefaultUID
	gid := nfs.DefaultUID

	vsUID := queryURL.Query().Get(uidQueryKey)
	vsGID := queryURL.Query().Get(gidQueryKey)

	if vsGID == "" {
		vsGID = vsUID
	}

	if vsUID != "" {
		uid, err = strconv.Atoi(vsUID)
		if err != nil {
			return -1, -1, err
		}
	}

	if vsGID != "" {
		gid, err = strconv.Atoi(vsGID)
		if err != nil {
			return -1, -1, err
		}
	}

	if uid < 0 {
		return -1, -1, fmt.Errorf("supplied url (%s) for nfs volume store has invalid uid : (%d)", queryURL.String(), uid)
	}

	if gid < 0 {
		return -1, -1, fmt.Errorf("supplied url (%s) for nfs volume store has invalid gid : (%d)", queryURL.String(), gid)
	}

	return uid, gid, nil
}

func createVsphereVolumeStore(op trace.Operation, dsurl *url.URL, name string, handlerCtx *HandlerContext) (volume.VolumeStorer, error) {
	ds, err := datastore.NewHelperFromURL(op, handlerCtx.Session, dsurl)
	if err != nil {
		err = fmt.Errorf("cannot find datastores: %s", err)
		op.Errorf("%s", err.Error())
		return nil, err
	}

	vs, err := vsvolume.NewVolumeStore(op, name, handlerCtx.Session, ds)
	if err != nil {
		err = fmt.Errorf("cannot instantiate the volume store: %s", err)
		op.Errorf("%s", err.Error())
		return nil, err
	}
	return vs, nil
}
