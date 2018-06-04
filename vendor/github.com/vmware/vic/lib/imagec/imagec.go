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

package imagec

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"golang.org/x/net/context"

	"github.com/docker/distribution/manifest/schema2"
	docker "github.com/docker/docker/image"
	dockerLayer "github.com/docker/docker/layer"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/pkg/progress"
	"github.com/docker/docker/pkg/streamformatter"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/reference"

	"github.com/vmware/vic/lib/apiservers/engine/backends/cache"
	"github.com/vmware/vic/lib/apiservers/portlayer/models"
	"github.com/vmware/vic/lib/constants"
	"github.com/vmware/vic/lib/metadata"
	urlfetcher "github.com/vmware/vic/pkg/fetcher"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/sys"
)

// ImageC is responsible for pulling docker images from a repository
type ImageC struct {
	Options

	// https://raw.githubusercontent.com/docker/docker/master/distribution/pull_v2.go
	sf             *streamformatter.StreamFormatter
	progressOutput progress.Output

	// ImageLayers are sourced from the manifest file
	ImageLayers []*ImageWithMeta

	// ImageID is the docker ImageID calculated during download
	ImageID string
}

// NewImageC returns a new instance of ImageC
func NewImageC(options Options, strfmtr *streamformatter.StreamFormatter) *ImageC {
	return &ImageC{
		Options:        options,
		sf:             strfmtr,
		progressOutput: strfmtr.NewProgressOutput(options.Outstream, false),
	}
}

// Options contain all options for a single instance of imagec
type Options struct {
	Reference reference.Named

	Registry string
	Image    string
	Tag      string

	Destination string

	Host      string
	Storename string

	Username string
	Password string

	Token *urlfetcher.Token

	Timeout time.Duration

	Outstream io.Writer

	InsecureSkipVerify bool
	InsecureAllowHTTP  bool

	// Get both schema 1 and schema 2 manifests.  Schema 1 is used to get history
	// and was imageC implementation predated schema 2.  Schema 2 is used to
	// calculate digest.
	ImageManifestSchema1 *Manifest
	ImageManifestSchema2 *schema2.DeserializedManifest

	//Digest of manifest schema 2 or schema 1; therefore, it may not match hash
	//of the above manifest.
	ManifestDigest string

	// RegistryCAs will not be modified by imagec
	RegistryCAs *x509.CertPool

	// If true, do not bother portlayer or persona
	Standalone bool

	// image store name or url
	ImageStore string
}

// ImageWithMeta wraps the models.Image with some additional metadata
type ImageWithMeta struct {
	*models.Image

	DiffID string
	Layer  FSLayer
	Meta   string
	Size   int64

	Downloading bool
}

func (i *ImageWithMeta) String() string {
	return stringid.TruncateID(i.Layer.BlobSum)
}

var (
	ldm *LayerDownloader
)

const (
	// DefaultDockerURL holds the URL of Docker registry
	DefaultDockerURL = "registry.hub.docker.com"

	// DefaultDestination specifies the default directory to use
	DefaultDestination = "images"

	// DefaultPortLayerHost specifies the default port layer server
	DefaultPortLayerHost = "localhost:2377"

	// DefaultLogfile specifies the default log file name
	DefaultLogfile = "imagec.log"

	// DefaultHTTPTimeout specifies the default HTTP timeout
	DefaultHTTPTimeout = 3600 * time.Second

	// attribute update actions
	Add = iota + 1
)

func init() {
	ldm = NewLayerDownloader()
}

// ParseReference parses the -reference parameter and populate options struct
func (ic *ImageC) ParseReference() {
	if reference.IsNameOnly(ic.Reference) {
		ic.Tag = reference.DefaultTag
	} else {
		if tagged, isTagged := ic.Reference.(reference.NamedTagged); isTagged {
			ic.Tag = tagged.Tag()
		}
	}

	ic.Registry = ic.Reference.Hostname()
	if ic.Registry == reference.DefaultHostname {
		ic.Registry = DefaultDockerURL
	}

	ic.Image = ic.Reference.RemoteName()
}

// DestinationDirectory returns the path of the output directory
func DestinationDirectory(options Options) string {
	// #nosec: Errors unhandled.
	u, _ := url.Parse(options.Registry)

	// Use a hierarchy like following so that we can support multiple schemes, registries and versions
	/*
		https/
		├── 192.168.218.5:5000
		│   └── v2
		│       └── busybox
		│           └── latest
		...
		│               ├── fef924a0204a00b3ec67318e2ed337b189c99ea19e2bf10ed30a13b87c5e17ab
		│               │   ├── fef924a0204a00b3ec67318e2ed337b189c99ea19e2bf10ed30a13b87c5e17ab.json
		│               │   └── fef924a0204a00b3ec67318e2ed337b189c99ea19e2bf10ed30a13b87c5e17ab.tar
		│               └── manifest.json
		└── registry-1.docker.io
		    └── v2
		        └── library
		            └── golang
		                └── latest
		                    ...
		                    ├── f61ebe2817bb4e6a7f0a4cf249a5316223f7ecc886feac24b9887a490feaed57
		                    │   ├── f61ebe2817bb4e6a7f0a4cf249a5316223f7ecc886feac24b9887a490feaed57.json
		                    │   └── f61ebe2817bb4e6a7f0a4cf249a5316223f7ecc886feac24b9887a490feaed57.tar
		                    └── manifest.json

	*/
	if u.Scheme == "" && u.Host == "" && u.Path == "" {
		return path.Join(
			options.Destination,
			"localhost",
			options.Image,
			options.Tag,
		)
	}
	return path.Join(
		options.Destination,
		u.Scheme,
		u.Host,
		u.Path,
		options.Image,
		options.Tag,
	)
}

// LayersToDownload creates a slice of ImageWithMeta for the layers that need to be downloaded
func (ic *ImageC) LayersToDownload() ([]*ImageWithMeta, error) {
	images := make([]*ImageWithMeta, len(ic.ImageManifestSchema1.FSLayers))

	manifest := ic.ImageManifestSchema1
	v1 := docker.V1Image{}

	// iterate from parent to children
	for i := len(ic.ImageManifestSchema1.History) - 1; i >= 0; i-- {
		history := manifest.History[i]
		layer := manifest.FSLayers[i]

		// unmarshall V1Compatibility to get the image ID
		if err := json.Unmarshal([]byte(history.V1Compatibility), &v1); err != nil {
			return nil, fmt.Errorf("Failed to unmarshall image history: %s", err)
		}

		// if parent is empty set it to scratch
		parent := constants.ScratchLayerID
		if v1.Parent != "" {
			parent = v1.Parent
		}

		// add image to ImageWithMeta list
		images[i] = &ImageWithMeta{
			Image: &models.Image{
				ID:     v1.ID,
				Parent: parent,
				Store:  ic.Storename,
			},
			Meta:  history.V1Compatibility,
			Layer: layer,
		}

		// populate manifest layer with existing cached data
		if layer, err := LayerCache().Get(images[i].ID); err == nil {
			if !layer.Downloading { // possibly unnecessary but won't hurt anything
				images[i] = layer
			}
		}
	}

	return images, nil
}

// UpdateRepositoryCache will update the repository cache
// that resides in the docker persona.  This will add image tag,
// digest and layer information.
func UpdateRepoCache(ic *ImageC) error {
	// if standalone then no persona, so exit
	if ic.Standalone {
		return nil
	}

	// LayerID for the image layer
	imageLayerID := ic.ImageLayers[0].ID

	// get the repoCache
	repoCache := cache.RepositoryCache()

	// In the case that we don't have the ImageID, then we need
	// to go to the RepositoryCache to get it.
	if ic.ImageID == "" {
		// call to repository cache for the imageID for this layer
		ic.ImageID = repoCache.GetImageID(imageLayerID)

		// if we still don't have an imageID we can't continue
		if ic.ImageID == "" {
			return fmt.Errorf("ImageID not found by LayerID(%s) in RepositoryCache", imageLayerID)
		}
	}
	// AddReference will add the repo:tag to the repositoryCache and save to the portLayer
	err := repoCache.AddReference(ic.Reference, ic.ImageID, true, imageLayerID, true)
	if err != nil {
		return fmt.Errorf("Unable to Add Image Reference(%s): %s", ic.Reference.String(), err.Error())
	}

	dig, err := reference.ParseNamed(fmt.Sprintf("%s@%s", ic.Reference.Name(), ic.ManifestDigest))
	if err != nil {
		return fmt.Errorf("Unable to parse digest: %s", err.Error())
	}

	// AddReference will add the digest and persist to the portLayer
	err = repoCache.AddReference(dig, ic.ImageID, true, imageLayerID, true)
	if err != nil {
		return fmt.Errorf("Unable to Add Image Digest(%s): %s", dig.String(), err.Error())
	}

	return nil
}

// WriteImageBlob writes the image blob to the storage layer
func (ic *ImageC) WriteImageBlob(image *ImageWithMeta, progressOutput progress.Output, cleanup bool) error {
	defer trace.End(trace.Begin(image.Image.ID))

	destination := DestinationDirectory(ic.Options)

	id := image.Image.ID
	filePath := path.Join(destination, id, id+".tar")
	log.Infof("Path: %s", filePath)
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("Failed to open file: %s", err)
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return fmt.Errorf("Failed to stat file: %s", err)
	}

	in := progress.NewProgressReader(
		ioutils.NewCancelReadCloser(context.Background(), f),
		progressOutput,
		fi.Size(),
		image.String(),
		"Extracting",
	)
	defer in.Close()

	if !ic.Standalone {
		// Write the image
		err = WriteImage(ic.Host, image, in)
		if err != nil {
			return fmt.Errorf("Failed to write to image store: %s", err)
		}
	} else {
		// If standalone, write to a local directory
		cleanup = false
	}

	progress.Update(progressOutput, image.String(), "Pull complete")

	if cleanup {
		if err := os.RemoveAll(destination); err != nil {
			return fmt.Errorf("Failed to remove download directory: %s", err)
		}
	}
	return nil
}

// CreateImageConfig constructs the image metadata from layers that compose the image
func (ic *ImageC) CreateImageConfig(images []*ImageWithMeta) (metadata.ImageConfig, error) {

	imageLayer := images[0] // the layer that represents the actual image

	// if we already have an imageID associated with this layerID, we don't need
	// to calculate imageID and can just grab the image config from the cache
	id := cache.RepositoryCache().GetImageID(imageLayer.ID)
	if image, err := cache.ImageCache().Get(id); err == nil {
		return *image, nil
	}

	manifest := ic.ImageManifestSchema1
	image := docker.V1Image{}
	rootFS := docker.NewRootFS()
	history := make([]docker.History, 0, len(images))
	diffIDs := make(map[string]string)
	var size int64

	// step through layers to get command history and diffID from oldest to newest
	for i := len(images) - 1; i >= 0; i-- {
		layer := images[i]
		if err := json.Unmarshal([]byte(layer.Meta), &image); err != nil {
			return metadata.ImageConfig{}, fmt.Errorf("Failed to unmarshall layer history: %s", err)
		}
		h := docker.History{
			Created:   image.Created,
			Author:    image.Author,
			CreatedBy: strings.Join(image.ContainerConfig.Cmd, " "),
			Comment:   image.Comment,
		}

		// is this an empty layer?
		if layer.DiffID == dockerLayer.DigestSHA256EmptyTar.String() {
			h.EmptyLayer = true
		} else {
			// if not empty, add diffID to rootFS
			rootFS.DiffIDs = append(rootFS.DiffIDs, dockerLayer.DiffID(layer.DiffID))
		}
		history = append(history, h)
		size += layer.Size
	}

	// result is constructed without unused fields
	result := docker.Image{
		V1Image: docker.V1Image{
			Comment:         image.Comment,
			Created:         image.Created,
			Container:       image.Container,
			ContainerConfig: image.ContainerConfig,
			DockerVersion:   image.DockerVersion,
			Author:          image.Author,
			Config:          image.Config,
			Architecture:    image.Architecture,
			OS:              image.OS,
		},
		RootFS:  rootFS,
		History: history,
	}

	imageConfigBytes, err := result.MarshalJSON()
	if err != nil {
		return metadata.ImageConfig{}, fmt.Errorf("Failed to marshall image metadata: %s", err)
	}

	// calculate image ID
	sum := fmt.Sprintf("%x", sha256.Sum256(imageConfigBytes))
	log.Infof("Image ID: sha256:%s", sum)

	// prepare metadata
	result.V1Image.Parent = image.Parent
	result.Size = size
	result.V1Image.ID = imageLayer.ID
	imageConfig := metadata.ImageConfig{
		V1Image: result.V1Image,
		ImageID: sum,
		DiffIDs: diffIDs,
		History: history,
	}

	if ic.Tag != "" {
		imageConfig.Tags = []string{ic.Tag}
	}
	if manifest != nil {
		imageConfig.Name = manifest.Name
	}
	if ic.Reference != nil {
		imageConfig.Reference = ic.Reference.String()
	}
	if _, ok := ic.Reference.(reference.Canonical); ok {
		log.Debugf("Populating digest in imageConfig for image: %s", ic.Reference.String())
		imageConfig.Digests = []string{ic.ManifestDigest}
	}

	return imageConfig, nil
}

// PullImage pulls an image from docker hub
func (ic *ImageC) PullImage() error {
	ctx, cancel := context.WithTimeout(ctx, ic.Options.Timeout)
	defer cancel()

	// Authenticate, get URL, get token
	if err := ic.prepareTransfer(ctx); err != nil {
		return err
	}

	// Output message
	tagOrDigest := tagOrDigest(ic.Reference, ic.Tag)
	progress.Message(ic.progressOutput, "", tagOrDigest+": Pulling from "+ic.Image)

	// Pull the image manifest
	if err := ic.pullManifest(ctx); err != nil {
		return err
	}
	log.Infof("Manifest for image = %#v", ic.ImageManifestSchema1)

	// Get layers to download from manifest
	layers, err := ic.LayersToDownload()
	if err != nil {
		return err
	}
	ic.ImageLayers = layers

	// Download all the layers
	if err := ldm.DownloadLayers(ctx, ic); err != nil {
		return err
	}

	return nil
}

// ListLayer prints out the layers for an image to progress.  This is used by imagec standalone binary
// for debug/validation.
func (ic *ImageC) ListLayers() error {
	defer trace.End(trace.Begin(""))

	ctx, cancel := context.WithTimeout(ctx, ic.Options.Timeout)
	defer cancel()

	// Authenticate, get URL, get token
	if err := ic.prepareTransfer(ctx); err != nil {
		return err
	}

	// Output message
	tagOrDigest := tagOrDigest(ic.Reference, ic.Tag)
	progress.Message(ic.progressOutput, "", tagOrDigest+": Fetching layers from "+ic.Image)

	// Pull the image manifest
	if err := ic.pullManifest(ctx); err != nil {
		return err
	}

	// Get layers to download from manifest
	layers, err := ic.LayersToDownload()
	if err != nil {
		return err
	}

	progress.Message(ic.progressOutput, "", constants.ScratchLayerID)
	for i := len(layers) - 1; i >= 0; i-- {
		progress.Message(ic.progressOutput, "", layers[i].ID)
	}

	return nil
}

// prepareTransfer Looks up URLs and fetch auth token
func (ic *ImageC) prepareTransfer(ctx context.Context) error {

	// Parse the -reference parameter
	ic.ParseReference()

	// Host is either the host's UUID (if run on vsphere) or the hostname of
	// the system (if run standalone)
	host, err := sys.UUID()
	if err != nil {
		log.Errorf("Failed to return host name: %s", err)
		return err
	}

	if host != "" {
		log.Infof("Using UUID (%s) for imagestore name", host)
	} else if ic.Standalone {
		host, err = os.Hostname()
		log.Infof("Using host (%s) for imagestore name", host)
	}

	ic.Storename = host

	if !ic.Standalone {
		log.Debugf("Running with portlayer")

		// Ping the server to ensure it's at least running
		ok, err := PingPortLayer(ic.Host)
		if err != nil || !ok {
			log.Errorf("Failed to ping portlayer: %s", err)
			return err
		}
	} else {
		log.Debugf("Running standalone")
	}

	// Calculate (and overwrite) the registry URL and make sure that it responds to requests
	ic.Registry, err = LearnRegistryURL(&ic.Options)
	if err != nil {
		log.Errorf("Error while pulling image: %s", err)
		return err
	}

	// Get the URL of the OAuth endpoint
	url, err := LearnAuthURL(ic.Options)
	if err != nil {
		log.Warnf("LearnAuthURL returned %s", err.Error())
		switch err := err.(type) {
		case urlfetcher.ImageNotFoundError:
			return fmt.Errorf("Error: image %s not found", ic.Reference)
		default:
			return fmt.Errorf("Failed to obtain OAuth endpoint: %s", err)
		}
	}

	// Get the OAuth token - if only we have a URL
	if url != nil {
		token, err := FetchToken(ctx, ic.Options, url, ic.progressOutput)
		if err != nil {
			log.Errorf("Failed to fetch OAuth token: %s", err)
			return err
		}
		ic.Token = token
	}

	return nil
}

// pullManifest attempts to pull manifest for an image.  Attempts to get schema 2 but will fall back to schema 1.
func (ic *ImageC) pullManifest(ctx context.Context) error {
	// Get the schema1 manifest
	manifest, digest, err := FetchImageManifest(ctx, ic.Options, 1, ic.progressOutput)
	if err != nil {
		log.Infof(err.Error())
		switch err := err.(type) {
		case urlfetcher.ImageNotFoundError:
			return fmt.Errorf("Error: image %s not found", ic.Image)
		case urlfetcher.TagNotFoundError:
			return fmt.Errorf("Tag %s not found in repository %s", ic.Tag, ic.Image)
		default:
			return fmt.Errorf("Error while pulling image manifest: %s", err)
		}
	}

	schema1, ok := manifest.(*Manifest)
	if !ok {
		return fmt.Errorf("Error pulling manifest schema 1")
	}

	ic.ImageManifestSchema1 = schema1
	ic.ManifestDigest = digest

	// Attempt to get schema2 manifest
	manifest, digest, err = FetchImageManifest(ctx, ic.Options, 2, ic.progressOutput)
	if err == nil {
		if schema2, ok := manifest.(*schema2.DeserializedManifest); ok {
			if schema2 != nil {
				log.Infof("pullManifest - schema 2: %#v", schema2)
			}
			ic.ImageManifestSchema2 = schema2

			// Override the manifest digest as Docker uses schema 2, unless the image
			// is pulled by digest since we only support pull-by-digest for schema 1.
			// TODO(anchal): this check should be removed once issue #5187 is implemented.
			if _, ok := ic.Reference.(reference.Canonical); !ok {
				ic.ManifestDigest = digest
			}
		}
	}

	return nil
}
