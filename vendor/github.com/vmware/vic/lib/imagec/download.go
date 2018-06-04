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
// See the License for the specific language governing permissi[ons and
// limitations under the License.

package imagec

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/vmware/vic/lib/apiservers/engine/backends/cache"
	"github.com/vmware/vic/pkg/trace"

	"github.com/docker/docker/distribution/xfer"
	"github.com/docker/docker/pkg/progress"
	"github.com/docker/docker/pkg/streamformatter"
)

const (
	maxDownloadAttempts    = 5
	maxConcurrentDownloads = 3
)

type downloadTransfer struct {
	xfer.Transfer

	layer *ImageWithMeta
	err   error
}

// result returns the layer resulting from the download, if the download
// and registration were successful.
func (d *downloadTransfer) result() error {
	return d.err
}

// LayerDownloader keeps track of what layers are currently being downloaded
type LayerDownloader struct {
	m             sync.Mutex
	downloadsByID map[string]*downloadTransfer

	tm xfer.TransferManager
}

// NewLayerDownloader creates and returns a new LayerDownloadManager
func NewLayerDownloader() *LayerDownloader {
	return &LayerDownloader{
		tm:            xfer.NewTransferManager(maxConcurrentDownloads),
		downloadsByID: make(map[string]*downloadTransfer),
	}
}

func (ldm *LayerDownloader) registerDownload(download *downloadTransfer) {
	ldm.downloadsByID[download.layer.ID] = download
}

func (ldm *LayerDownloader) unregisterDownload(layer *ImageWithMeta) {
	// stop tracking the download transfer
	delete(ldm.downloadsByID, layer.ID)
}

type prog struct {
	err chan error
	p   progress.Progress
}

type serialProgressOutput struct {
	c   chan prog
	out progress.Output
}

func (s *serialProgressOutput) WriteProgress(p progress.Progress) error {
	pr := prog{err: make(chan error, 1), p: p}
	s.c <- pr
	err := <-pr.err
	close(pr.err)
	return err
}

func (s *serialProgressOutput) run() {
	for pr := range s.c {
		err := s.out.WriteProgress(pr.p)
		pr.err <- err
	}
}

func (s *serialProgressOutput) stop() {
	close(s.c)
}

// DownloadLayers ensures layers end up in the portlayer's image store
// It handles existing and simultaneous layer download de-duplication
// This code is utilizes Docker's xfer package: https://github.com/docker/docker/tree/v1.11.2/distribution/xfer
func (ldm *LayerDownloader) DownloadLayers(ctx context.Context, ic *ImageC) error {
	defer trace.End(trace.Begin(""))

	var (
		topDownload    *downloadTransfer
		watcher        *xfer.Watcher
		d              xfer.Transfer
		layerCount     = 0
		sf             = streamformatter.NewJSONStreamFormatter()
		progressOutput = &serialProgressOutput{
			c:   make(chan prog, 100),
			out: sf.NewProgressOutput(ic.Outstream, false),
		}
	)

	go progressOutput.run()
	defer progressOutput.stop()

	// lock here so that we get all layers in flight before another client comes along
	ldm.m.Lock()

	// Grab the imageLayers
	layers := ic.ImageLayers

	// iterate backwards through layers to download
	for i := len(layers) - 1; i >= 0; i-- {

		layer := layers[i]
		id := layer.ID

		layerConfig, err := LayerCache().Get(id)
		if err != nil {

			switch err := err.(type) {
			case LayerNotFoundError:

				layerCount++

				// layer does not already exist in store and is not currently in flight, so download it
				progress.Update(progressOutput, layer.String(), "Pulling fs layer")

				xferFunc := ldm.makeDownloadFunc(layer, ic, topDownload, layers)
				d, watcher = ldm.tm.Transfer(id, xferFunc, progressOutput)
				topDownload = d.(*downloadTransfer)

				defer topDownload.Transfer.Release(watcher)

				ldm.registerDownload(topDownload)
				layer.Downloading = true
				LayerCache().Add(layer)

				continue
			default:
				return err
			}
		}

		if layerConfig.Downloading {

			layerCount++

			if existingDownload, ok := ldm.downloadsByID[id]; ok {

				xferFunc := ldm.makeDownloadFuncFromDownload(layer, existingDownload, topDownload, layers)
				d, watcher = ldm.tm.Transfer(id, xferFunc, progressOutput)
				topDownload = d.(*downloadTransfer)

				defer topDownload.Transfer.Release(watcher)

			}
			continue
		}

		progress.Update(progressOutput, layer.String(), "Already exists")
	}

	ldm.m.Unlock()

	// each layer download will block until the parent download finishes,
	// so this will block until the child-most layer, and thus all layers, have finished downloading
	if layerCount > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-topDownload.Done():
		default:
			<-topDownload.Done()
		}
		err := topDownload.result()
		if err != nil {
			return err
		}
	} else {
		if err := UpdateRepoCache(ic); err != nil {
			return err
		}
	}

	progress.Message(progressOutput, "", "Digest: "+ic.ManifestDigest)

	tagOrDigest := tagOrDigest(ic.Reference, ic.Tag)
	if layerCount > 0 {
		progress.Message(progressOutput, "", "Status: Downloaded newer image for "+ic.Image+":"+tagOrDigest)
	} else {
		progress.Message(progressOutput, "", "Status: Image is up to date for "+ic.Image+":"+tagOrDigest)
	}

	return nil
}

// makeDownloadFunc returns a func used by xfer.TransferManager to download a layer
func (ldm *LayerDownloader) makeDownloadFunc(layer *ImageWithMeta, ic *ImageC, parentDownload *downloadTransfer, layers []*ImageWithMeta) xfer.DoFunc {
	return func(progressChan chan<- progress.Progress, start <-chan struct{}, inactive chan<- struct{}) xfer.Transfer {

		d := &downloadTransfer{
			Transfer: xfer.NewTransfer(),
			layer:    layer,
		}

		go func() {

			defer func() {
				close(progressChan)

				// remove layer from cache if there was an error attempting to download
				if d.err != nil {
					LayerCache().Remove(layer.ID)
				}

			}()

			progressOutput := progress.ChanOutput(progressChan)

			// wait for TransferManager to give the go-ahead
			select {
			case <-start:
			default:
				progress.Update(progressOutput, layer.String(), "Waiting")
				<-start
			}

			if parentDownload != nil {
				// bail if parent download failed or was cancelled
				select {
				case <-parentDownload.Done():
					if err := parentDownload.result(); err != nil {
						d.err = err
						return
					}
				default:
				}
			}

			// fetch blob
			diffID, err := FetchImageBlob(d.Transfer.Context(), ic.Options, layer, progressOutput)
			if err != nil {
				d.err = fmt.Errorf("%s/%s returned %s", ic.Image, layer.ID, err)
				return
			}

			layer.DiffID = diffID

			close(inactive)

			if parentDownload != nil {
				select {
				case <-d.Transfer.Context().Done():
					d.err = errors.New("layer download cancelled")
					return
				default:
					<-parentDownload.Done() // block until parent download completes
				}

				if err := parentDownload.result(); err != nil {
					d.err = err
					return
				}
			}

			// is this the leaf layer?
			imageLayer := layer.ID == layers[0].ID

			if !ic.Standalone {
				// if this is the leaf layer, we are done and can now create the image config
				if imageLayer {
					imageConfig, err := ic.CreateImageConfig(layers)
					if err != nil {
						d.err = err
						return
					}
					// cache and persist the image
					cache.ImageCache().Add(&imageConfig)
					if err := cache.ImageCache().Save(); err != nil {
						d.err = fmt.Errorf("error saving image cache: %s", err)
						return
					}

					// place calculated ImageID in struct
					ic.ImageID = imageConfig.ImageID

					if err = UpdateRepoCache(ic); err != nil {
						d.err = err
						return
					}

				}
			}

			ldm.m.Lock()
			defer ldm.m.Unlock()

			// Write blob to the storage layer
			if err := ic.WriteImageBlob(layer, progressOutput, imageLayer); err != nil {
				d.err = err
				return
			}

			if !ic.Standalone {
				// mark the layer as finished downloading
				LayerCache().Commit(layer)
			}

			ldm.unregisterDownload(layer)

		}()

		return d
	}
}

// makeDownloadFuncFromDownload returns a func used by the TransferManager to
// handle a layer that was already seen in this image pull, or is currently being downloaded
func (ldm *LayerDownloader) makeDownloadFuncFromDownload(layer *ImageWithMeta, sourceDownload, parentDownload *downloadTransfer, layers []*ImageWithMeta) xfer.DoFunc {

	return func(progressChan chan<- progress.Progress, start <-chan struct{}, inactive chan<- struct{}) xfer.Transfer {

		d := &downloadTransfer{
			Transfer: xfer.NewTransfer(),
			layer:    layer,
		}

		go func() {
			defer close(progressChan)

			<-start

			close(inactive)

			if parentDownload != nil {
				select {
				case <-d.Transfer.Context().Done():
					d.err = errors.New("layer download cancelled")
					return
				case <-parentDownload.Done(): // wait for parent layer download to complete
				}

				if err := parentDownload.result(); err != nil {
					d.err = err
					return
				}
			}

			// sourceDownload should have already finished if
			// parentDownload finished, but wait for it explicitly
			// to be sure.
			select {
			case <-d.Transfer.Context().Done():
				d.err = errors.New("layer download cancelled")
				return
			case <-sourceDownload.Done():
			}

			if err := sourceDownload.result(); err != nil {
				d.err = err
				return
			}

		}()

		return d
	}

}
