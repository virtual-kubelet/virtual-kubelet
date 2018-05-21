package proxy

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/vmware/vic/lib/apiservers/engine/backends/cache"
	"github.com/vmware/vic/lib/apiservers/portlayer/client"
	"github.com/vmware/vic/lib/metadata"
	"github.com/vmware/vic/pkg/trace"
)

//TODO:  This image cache needs to hook into the VIC persona to receive an event when users pulled images
// via docker.

type ImageStore interface {
	Get(op trace.Operation, idOrRef, tag string, actuate bool) (*metadata.ImageConfig, error)
	GetImages(op trace.Operation) []*metadata.ImageConfig
	PullImage(op trace.Operation, image, tag, username, password string) error
}

type VicImageStore struct {
	client        *client.PortLayer
	personaAddr   string
	portlayerAddr string
}

type ImageStoreError string

func (e ImageStoreError) Error() string { return string(e) }

const (
	ImageStorePortlayerClientError = ImageStoreError("ImageStore cannot be created without a valid portlayer client")
	ImageStorePersonaAddrError     = ImageStoreError("ImageStore cannot be created without a valid VIC persona addr")
	ImageStorePortlayerAddrError   = ImageStoreError("ImageStore cannot be created without a valid VIC portlayer addr")
	ImageStoreContainerIDError     = ImageStoreError("ImageStore called with empty container ID")
	ImageStoreEmptyUserNameError   = ImageStoreError("ImageStore called with empty username")
	ImageStoreEmptyPasswordError   = ImageStoreError("ImageStore called with empty password")
)

func NewImageStore(plClient *client.PortLayer, personaAddr, portlayerAddr string) (ImageStore, error) {
	if plClient == nil {
		return nil, ImageStorePortlayerClientError
	}
	if personaAddr == "" {
		return nil, ImageStorePersonaAddrError
	}
	if portlayerAddr == "" {
		return nil, ImageStorePortlayerAddrError
	}

	err := cache.InitializeImageCache(plClient)
	if err != nil {
		return nil, err
	}

	vs := &VicImageStore{
		client:        plClient,
		personaAddr:   personaAddr,
		portlayerAddr: portlayerAddr,
	}

	return vs, nil
}

// Get retrieves the VIC ImageConfig data structure.  If the config is not cached,
// VicImageStore can request imagec to pull the image if actuate is set to true.
//
// arguments:
//		op		operation trace logger
//		idOrRef	docker image id or reference
//		tag		docker image tag
//		realize	determines whether the image is pulled if not in the cache
// returns:
// 		error
func (v *VicImageStore) Get(op trace.Operation, idOrRef, tag string, realize bool) (*metadata.ImageConfig, error) {
	defer trace.End(trace.Begin(fmt.Sprintf("Get - %s:%s", idOrRef, tag), op))

	if idOrRef == "" {
		op.Errorf(ImageStoreContainerIDError.Error())
		return nil, ImageStoreContainerIDError
	}

	c, err := cache.ImageCache().Get(idOrRef)
	if err != nil && realize {
		err = v.PullImage(op, idOrRef, tag, "", "")
		if err == nil {
			//TODO:  Find a better way to get update imageconfig instead of this hammer
			err := cache.InitializeImageCache(v.client)
			if err != nil {
				return nil, err
			}
			c, err = cache.ImageCache().Get(idOrRef)
			if err != nil {
				return nil, err
			}
		}
	}

	return c, nil
}

// Get retrieves all the VIC ImageConfig data structure.
//
// arguments:
//		op		operation trace logger
// returns:
// 		array of ImageConfig
func (v *VicImageStore) GetImages(op trace.Operation) []*metadata.ImageConfig {
	defer trace.End(trace.Begin("", op))

	return cache.ImageCache().GetImages()
}

// PullImage makes a request to the VIC persona server (imageC component) to retrieve a container image.
//
// arguments:
//		op			operation trace logger
//		idOrRef		docker image id or reference
//		tag			docker image tag
//		username	user name for the registry server
//		password	password for the registry server
// returns:
// 		array of ImageConfig
func (v *VicImageStore) PullImage(op trace.Operation, image, tag, username, password string) error {
	defer trace.End(trace.Begin(fmt.Sprintf("Get - %s:%s", image, tag), op))

	if image == "" {
		op.Errorf(ImageStoreContainerIDError.Error())
		return ImageStoreContainerIDError
	}

	pullClient := &http.Client{Timeout: 60 * time.Second}
	var personaServer string
	if tag == "" {
		personaServer = fmt.Sprintf("http://%s/v1.35/images/create?fromImage=%s", v.personaAddr, image)
	} else {
		personaServer = fmt.Sprintf("http://%s/v1.35/images/create?fromImage=%s&tag=%s", v.personaAddr, image, tag)
	}
	op.Infof("POST %s", personaServer)
	reader := bytes.NewBuffer([]byte(""))
	resp, err := pullClient.Post(personaServer, "application/json", reader)
	if err != nil {
		op.Errorf("Error from docker pull: error = %s", err.Error())
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		msg := fmt.Sprintf("Error from docker pull: status = %d", resp.StatusCode)
		op.Errorf(msg)
		return fmt.Errorf(msg)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		msg := fmt.Sprintf("Error reading docker pull response: error = %s", err.Error())
		op.Errorf(msg)
		return fmt.Errorf(msg)
	}
	op.Infof("Response from docker pull: body = %s", string(body))

	return nil
}
