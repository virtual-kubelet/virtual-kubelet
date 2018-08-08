// Copyright 2016-2018 VMware, Inc. All Rights Reserved.
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

package backends

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"

	"golang.org/x/net/context"

	"github.com/docker/distribution/digest"
	"github.com/docker/docker/api/types"
	eventtypes "github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/pkg/streamformatter"
	"github.com/docker/docker/reference"

	"github.com/vmware/vic/lib/apiservers/engine/backends/cache"
	vicfilter "github.com/vmware/vic/lib/apiservers/engine/backends/filter"
	"github.com/vmware/vic/lib/apiservers/engine/errors"
	"github.com/vmware/vic/lib/apiservers/portlayer/client/storage"
	"github.com/vmware/vic/lib/imagec"
	"github.com/vmware/vic/lib/metadata"
	"github.com/vmware/vic/lib/portlayer/util"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/uid"
	"github.com/vmware/vic/pkg/vsphere/sys"
)

// valid filters as of docker commit 49bf474
var acceptedImageFilterTags = map[string]bool{
	"dangling":  true,
	"label":     true,
	"before":    true,
	"since":     true,
	"reference": true,
}

// currently not supported by vic
var unSupportedImageFilters = map[string]bool{
	"dangling": false,
}

type ImageBackend struct {
}

// All the API entry points create an Operation and log a message at INFO
// Level, this is done so that the Operation can be tracked as it moves
// through the server and propagates to the portlayer

func NewImageBackend() *ImageBackend {
	return &ImageBackend{}
}

func (i *ImageBackend) Exists(containerName string) bool {
	return false
}

// TODO fix the errors so the client doesnt print the generic POST or DELETE message
func (i *ImageBackend) ImageDelete(imageRef string, force, prune bool) ([]types.ImageDelete, error) {
	op := trace.NewOperation(context.Background(), "ImageDelete: %s", imageRef)
	defer trace.End(trace.Audit(imageRef, op))

	var (
		deletedRes  []types.ImageDelete
		userRefIsID bool
	)

	// Use the image cache to go from the reference to the ID we use in the image store
	img, err := cache.ImageCache().Get(imageRef)
	if err != nil {
		return nil, err
	}

	tags := img.Tags
	digests := img.Digests

	// did the user pass an id or partial id
	userRefIsID = cache.ImageCache().IsImageID(imageRef)
	// do we have any reference conflicts
	if len(tags) > 1 && userRefIsID && !force {
		t := uid.Parse(img.ImageID).Truncate()
		return nil,
			fmt.Errorf("conflict: unable to delete %s (must be forced) - image is referenced in one or more repositories", t)
	}

	// if we have an ID or only 1 tag/digest lets delete the vmdk(s) via the PL
	if userRefIsID || len(tags) == 1 || len(digests) == 1 {
		log.Infof("Deleting image via PL %s (%s)", img.ImageID, img.ID)

		// storeName is the uuid of the host this service is running on.
		storeName, err := sys.UUID()
		if err != nil {
			return nil, err
		}

		// We're going to delete all of the images in the layer branch starting
		// at the given leaf.  BUT!  we need to keep the images which may be
		// referenced by tags.  Therefore, we need to assemble a list of images
		// (by URI) which are referred to by tags.
		allImages := cache.ImageCache().GetImages()
		keepNodes := make([]string, len(allImages))
		for idx, node := range allImages {
			imgURL, err := util.ImageURL(storeName, node.ImageID)
			if err != nil {
				return nil, err
			}

			keepNodes[idx] = imgURL.String()
		}

		params := storage.NewDeleteImageParamsWithContext(op).WithStoreName(storeName).WithID(img.ID).WithKeepNodes(keepNodes)
		// TODO: This will fail if any containerVMs are referencing the vmdk - vanilla docker
		// allows the removal of an image (via force flag) even if a container is referencing it
		// should vic?
		res, err := PortLayerClient().Storage.DeleteImage(params)

		// We may have deleted images despite error.  Account for that in the cache.
		if res != nil {
			for _, deletedImage := range res.Payload {

				// map the layer id to the blob sum so the ids map to what we
				// present to the user on pull
				id := deletedImage.ID
				i, err := imagec.LayerCache().Get(deletedImage.ID)
				if err == nil {
					id = i.Layer.BlobSum
				}

				// remove the layer from the layer cache (used by imagec)
				imagec.LayerCache().Remove(deletedImage.ID)

				// form the response
				imageDeleted := types.ImageDelete{Deleted: strings.TrimPrefix(id, "sha256:")}
				deletedRes = append(deletedRes, imageDeleted)
			}

			if err := imagec.LayerCache().Save(); err != nil {
				return nil, fmt.Errorf("failed to save layer cache: %s", err)
			}
		}

		if err != nil {
			switch err := err.(type) {
			case *storage.DeleteImageLocked:
				return nil, fmt.Errorf("Failed to remove image %q: %s", imageRef, err.Payload.Message)
			default:
				return nil, err
			}
		}

		// we've deleted the image so remove from cache
		cache.ImageCache().RemoveImageByConfig(img)
		if err := cache.ImageCache().Save(); err != nil {
			return nil, fmt.Errorf("failed to save image cache: %s", err)
		}

		actor := CreateImageEventActorWithAttributes(imageRef, imageRef, map[string]string{})
		EventService().Log("delete", eventtypes.ImageEventType, actor)
	} else {

		// only untag the ref supplied
		n, err := reference.ParseNamed(imageRef)
		if err != nil {
			return nil, fmt.Errorf("unable to parse reference(%s): %s", imageRef, err.Error())
		}
		tag := reference.WithDefaultTag(n)
		tags = []string{tag.String()}

		actor := CreateImageEventActorWithAttributes(imageRef, imageRef, map[string]string{})
		EventService().Log("untag", eventtypes.ImageEventType, actor)
	}
	// loop thru and remove from repoCache
	for i := range tags {
		// remove from cache, but don't save -- we'll do that afer all
		// updates
		// #nosec: Errors unhandled.
		refNamed, _ := cache.RepositoryCache().Remove(tags[i], false)
		deletedRes = append(deletedRes, types.ImageDelete{Untagged: refNamed})
	}

	for i := range digests {
		// #nosec: Errors unhandled.
		refNamed, _ := cache.RepositoryCache().Remove(digests[i], false)
		deletedRes = append(deletedRes, types.ImageDelete{Untagged: refNamed})
	}

	// save repo now -- this will limit the number of PL
	// calls to one per rmi call
	err = cache.RepositoryCache().Save()
	if err != nil {
		return nil, fmt.Errorf("Untag error: %s", err.Error())
	}

	return deletedRes, err
}

func (i *ImageBackend) ImageHistory(imageName string) ([]*types.ImageHistory, error) {
	op := trace.NewOperation(context.Background(), "ImageHistory: %s", imageName)
	defer trace.End(trace.Audit(imageName, op))

	return nil, errors.APINotSupportedMsg(ProductName(), "ImageHistory")
}

func (i *ImageBackend) Images(imageFilters filters.Args, all bool, withExtraAttrs bool) ([]*types.ImageSummary, error) {
	op := trace.NewOperation(context.Background(), "Images: %#v", imageFilters)
	defer trace.End(trace.Audit("", op))

	// validate filters for accuracy and support
	filterContext, err := vicfilter.ValidateImageFilters(imageFilters, acceptedImageFilterTags, unSupportedImageFilters)
	if err != nil {
		return nil, err
	}

	// get all images
	images := cache.ImageCache().GetImages()

	result := make([]*types.ImageSummary, 0, len(images))

imageLoop:
	for i := range images {

		// provide filter with current ImageID
		filterContext.ID = images[i].ImageID

		// provide image labels
		if images[i].Config != nil {
			filterContext.Labels = images[i].Config.Labels
		}

		// determine if image should be part of list
		action := vicfilter.IncludeImage(imageFilters, filterContext)

		switch action {
		case vicfilter.ExcludeAction:
			continue imageLoop
		case vicfilter.StopAction:
			break imageLoop
		}
		// if we are here then add image
		dockerImage := convertV1ImageToDockerImage(images[i])
		// reference is a filter, so we must add the tags / digests
		// identified by the filter
		if imageFilters.Include("reference") {
			dockerImage.RepoTags = filterContext.Tags
			dockerImage.RepoDigests = filterContext.Digests

		}
		result = append(result, dockerImage)
	}

	return result, nil
}

// Docker Inspect.  LookupImage looks up an image by name and returns it as an
// ImageInspect structure.
func (i *ImageBackend) LookupImage(name string) (*types.ImageInspect, error) {
	op := trace.NewOperation(context.Background(), "LookupImage: %s", name)
	defer trace.End(trace.Audit(name, op))

	imageConfig, err := cache.ImageCache().Get(name)
	if err != nil {
		return nil, err
	}

	return imageConfigToDockerImageInspect(imageConfig, ProductName()), nil
}

func (i *ImageBackend) TagImage(imageName, repository, tag string) error {
	op := trace.NewOperation(context.Background(), "TagImage: %s", imageName)
	defer trace.End(trace.Audit(imageName, op))

	img, err := cache.ImageCache().Get(imageName)
	if err != nil {
		return err
	}

	newTag, err := reference.WithName(repository)
	if err != nil {
		return err
	}
	if tag != "" {
		if newTag, err = reference.WithTag(newTag, tag); err != nil {
			return err
		}
	}

	// place tag in repo and save to portLayer k/v store
	err = cache.RepositoryCache().AddReference(newTag, img.ImageID, true, "", true)
	if err != nil {
		return err
	}

	actor := CreateImageEventActorWithAttributes(imageName, newTag.String(), map[string]string{})
	EventService().Log("tag", eventtypes.ImageEventType, actor)

	return nil
}

func (i *ImageBackend) ImagesPrune(pruneFilters filters.Args) (*types.ImagesPruneReport, error) {
	op := trace.NewOperation(context.Background(), "ImagesPrune")
	defer trace.End(trace.Audit("", op))

	return nil, errors.APINotSupportedMsg(ProductName(), "ImagesPrune")
}

func (i *ImageBackend) LoadImage(inTar io.ReadCloser, outStream io.Writer, quiet bool) error {
	op := trace.NewOperation(context.Background(), "LoadImage")
	defer trace.End(trace.Audit("", op))

	return errors.APINotSupportedMsg(ProductName(), "LoadImage")
}

func (i *ImageBackend) ImportImage(src string, repository, tag string, msg string, inConfig io.ReadCloser, outStream io.Writer, changes []string) error {
	op := trace.NewOperation(context.Background(), "ImportImage")
	defer trace.End(trace.Audit("", op))

	return errors.APINotSupportedMsg(ProductName(), "ImportImage")
}

func (i *ImageBackend) ExportImage(names []string, outStream io.Writer) error {
	op := trace.NewOperation(context.Background(), "ExportImage")
	defer trace.End(trace.Audit("", op))

	return errors.APINotSupportedMsg(ProductName(), "ExportImage")
}

func (i *ImageBackend) PullImage(ctx context.Context, image, tag string, metaHeaders map[string][]string, authConfig *types.AuthConfig, outStream io.Writer) error {
	op := trace.NewOperation(context.Background(), "PullImage: %s", image)
	defer trace.End(trace.Audit(image, op))

	op.Debugf("PullImage: image = %s, tag = %s, metaheaders = %+v\n", image, tag, metaHeaders)

	//***** Code from Docker 1.13 PullImage to convert image and tag to a ref
	image = strings.TrimSuffix(image, ":")

	ref, err := reference.ParseNamed(image)
	if err != nil {
		return err
	}

	if tag != "" {
		// The "tag" could actually be a digest.
		var dgst digest.Digest
		dgst, err = digest.ParseDigest(tag)
		if err == nil {
			ref, err = reference.WithDigest(reference.TrimNamed(ref), dgst)
		} else {
			ref, err = reference.WithTag(ref, tag)
		}
		if err != nil {
			return err
		}
	}
	//*****

	options := imagec.Options{
		Destination: os.TempDir(),
		Reference:   ref,
		Timeout:     imagec.DefaultHTTPTimeout,
		Outstream:   outStream,
	}

	portLayerServer := PortLayerServer()
	if portLayerServer != "" {
		options.Host = portLayerServer
	}

	ic := imagec.NewImageC(options, streamformatter.NewJSONStreamFormatter())
	ic.ParseReference()
	// create url from hostname
	hostnameURL, err := url.Parse(ic.Registry)
	if err != nil || hostnameURL.Hostname() == "" {
		hostnameURL, err = url.Parse("//" + ic.Registry)
		if err != nil {
			op.Infof("Error parsing hostname %s during registry access: %s", ic.Registry, err.Error())
		}
	}

	// Check if url is contained within set of whitelisted or insecure registries
	regctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	whitelistOk, _, insecureOk := vchConfig.RegistryCheck(regctx, hostnameURL)
	if !whitelistOk {
		err = fmt.Errorf("Access denied to unauthorized registry (%s) while VCH is in whitelist mode", hostnameURL.Host)
		op.Errorf(err.Error())
		sf := streamformatter.NewJSONStreamFormatter()
		outStream.Write(sf.FormatError(err))
		return nil
	}

	ic.InsecureAllowHTTP = insecureOk
	ic.RegistryCAs = RegistryCertPool

	if authConfig != nil {
		if len(authConfig.Username) > 0 {
			ic.Username = authConfig.Username
		}
		if len(authConfig.Password) > 0 {
			ic.Password = authConfig.Password
		}
	}

	op.Infof("PullImage: reference: %s, %s, portlayer: %#v",
		ic.Reference,
		ic.Host,
		portLayerServer)

	err = ic.PullImage()
	if err != nil {
		return err
	}

	//TODO:  Need repo name as second parameter.  Leave blank for now
	actor := CreateImageEventActorWithAttributes(image, "", map[string]string{})
	EventService().Log("pull", eventtypes.ImageEventType, actor)
	return nil
}

func (i *ImageBackend) PushImage(ctx context.Context, image, tag string, metaHeaders map[string][]string, authConfig *types.AuthConfig, outStream io.Writer) error {
	op := trace.NewOperation(context.Background(), "PushImage: %s", image)
	defer trace.End(trace.Audit(image, op))

	return errors.APINotSupportedMsg(ProductName(), "PushImage")
}

func (i *ImageBackend) SearchRegistryForImages(ctx context.Context, filtersArgs string, term string, limit int, authConfig *types.AuthConfig, metaHeaders map[string][]string) (*registry.SearchResults, error) {
	op := trace.NewOperation(context.Background(), "SearchRegistryForImages")
	defer trace.End(trace.Audit("", op))

	return nil, errors.APINotSupportedMsg(ProductName(), "SearchRegistryForImages")
}

// Utility functions

func convertV1ImageToDockerImage(image *metadata.ImageConfig) *types.ImageSummary {
	var labels map[string]string
	if image.Config != nil {
		labels = image.Config.Labels
	}

	return &types.ImageSummary{
		ID:          image.ImageID,
		ParentID:    image.Parent,
		RepoTags:    image.Tags,
		RepoDigests: image.Digests,
		Created:     image.Created.Unix(),
		Size:        image.Size,
		VirtualSize: image.Size,
		Labels:      labels,
	}
}

// Converts the data structure retrieved from the portlayer.  This src datastructure
// represents the unmarshalled data saved in the storage port layer.  The return
// data is what the Docker CLI understand and returns to user.
func imageConfigToDockerImageInspect(imageConfig *metadata.ImageConfig, productName string) *types.ImageInspect {
	if imageConfig == nil {
		return nil
	}

	rootfs := types.RootFS{
		Type:      "layers",
		Layers:    make([]string, 0, len(imageConfig.History)),
		BaseLayer: "",
	}

	for k := range imageConfig.DiffIDs {
		rootfs.Layers = append(rootfs.Layers, k)
	}

	inspectData := &types.ImageInspect{
		RepoTags:        imageConfig.Tags,
		RepoDigests:     imageConfig.Digests,
		Parent:          imageConfig.Parent,
		Comment:         imageConfig.Comment,
		Created:         imageConfig.Created.Format(time.RFC3339Nano),
		Container:       imageConfig.Container,
		ContainerConfig: &imageConfig.ContainerConfig,
		DockerVersion:   imageConfig.DockerVersion,
		Author:          imageConfig.Author,
		Config:          imageConfig.Config,
		Architecture:    imageConfig.Architecture,
		Os:              imageConfig.OS,
		Size:            imageConfig.Size,
		VirtualSize:     imageConfig.Size,
		RootFS:          rootfs,
	}

	inspectData.GraphDriver.Name = productName + " " + PortlayerName

	// ImageID is currently stored within VIC without the "sha256:" prefix
	// so we add it here to match Docker output.
	inspectData.ID = digest.Canonical.String() + ":" + imageConfig.ImageID

	return inspectData
}

func CreateImageEventActorWithAttributes(imageID, refName string, attributes map[string]string) eventtypes.Actor {
	if imageConfig, err := cache.ImageCache().Get(imageID); err == nil && imageConfig != nil {
		for k, v := range imageConfig.Config.Labels {
			attributes[k] = v
		}
	}

	if refName != "" {
		attributes["name"] = refName
	}

	return eventtypes.Actor{
		ID:         imageID,
		Attributes: attributes,
	}
}
