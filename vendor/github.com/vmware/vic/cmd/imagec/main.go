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

package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"runtime/debug"
	"runtime/trace"
	"strings"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/streamformatter"
	"github.com/docker/docker/reference"
	"github.com/go-openapi/runtime"
	rc "github.com/go-openapi/runtime/client"
	"github.com/pkg/profile"

	"github.com/vmware/vic/lib/apiservers/engine/proxy"
	apiclient "github.com/vmware/vic/lib/apiservers/portlayer/client"
	vicarchive "github.com/vmware/vic/lib/archive"
	"github.com/vmware/vic/lib/constants"
	"github.com/vmware/vic/lib/imagec"
	optrace "github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/version"
)

var (
	imageCOptions = ImageCOptions{}

	// https://raw.githubusercontent.com/docker/docker/master/distribution/pull_v2.go
	sf = streamformatter.NewJSONStreamFormatter()
)

const (
	PullImage  = "pull"
	PushImage  = "push"
	ListLayers = "listlayers"
	Save       = "save"
)

// ImageCOptions wraps the cli arguments
type ImageCOptions struct {
	imageName string

	options imagec.Options

	logfile string

	stdout bool
	debug  bool

	profiling string
	tracing   bool

	operation string
}

func init() {

	flag.StringVar(&imageCOptions.imageName, "reference", "", "Name of the reference")

	flag.StringVar(&imageCOptions.options.Destination, "destination", imagec.DefaultDestination, "Destination directory")

	flag.StringVar(&imageCOptions.options.Host, "host", imagec.DefaultPortLayerHost, "Host that runs portlayer API (FQDN:port format)")

	flag.StringVar(&imageCOptions.logfile, "logfile", imagec.DefaultLogfile, "Path of the imagec log file")

	flag.StringVar(&imageCOptions.options.Username, "username", "", "Username")
	flag.StringVar(&imageCOptions.options.Password, "password", "", "Password")

	flag.DurationVar(&imageCOptions.options.Timeout, "timeout", imagec.DefaultHTTPTimeout, "HTTP timeout")

	flag.BoolVar(&imageCOptions.stdout, "stdout", false, "Enable writing to stdout")
	flag.BoolVar(&imageCOptions.debug, "debug", false, "Show debug logging")
	flag.BoolVar(&imageCOptions.options.InsecureSkipVerify, "insecure-skip-verify", false, "Don't verify certificates when fetching images")
	flag.BoolVar(&imageCOptions.options.InsecureAllowHTTP, "insecure-allow-http", false, "Uses unencrypted connections when fetching images")
	flag.BoolVar(&imageCOptions.options.Standalone, "standalone", false, "Disable port-layer integration")

	flag.StringVar(&imageCOptions.profiling, "profile.mode", "", "Enable profiling mode, one of [cpu, mem, block]")
	flag.BoolVar(&imageCOptions.tracing, "tracing", false, "Enable runtime tracing")

	flag.StringVar(&imageCOptions.operation, "operation", "pull", "Pull image/push image/listlayers/save image")

	flag.StringVar(&imageCOptions.options.Registry, "registry", imagec.DefaultDockerURL, "Registry to pull/push images (default: registry-1.docker.io)")

	flag.StringVar(&imageCOptions.options.ImageStore, "image-store", imagec.DefaultDockerURL, "portlayer image store name or url used to query image data")

	flag.Parse()

	var err error
	if imageCOptions.options.Reference, err = reference.ParseNamed(imageCOptions.imageName); err != nil {
		panic(err)
	}
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			_, err := fmt.Fprintf(os.Stderr, string(sf.FormatError(fmt.Errorf("%s : %s", r, debug.Stack()))))
			if err != nil {
				//do this to pass security check
			}
		}
	}()

	if version.Show() {
		_, err := fmt.Fprintf(os.Stdout, "%s\n", version.String())
		if err != nil {
			panic(err)
		}
		return
	}

	// Enable profiling if mode is set
	switch imageCOptions.profiling {
	case "cpu":
		defer profile.Start(profile.CPUProfile, profile.ProfilePath("."), profile.Quiet).Stop()
	case "mem":
		defer profile.Start(profile.MemProfile, profile.ProfilePath("."), profile.Quiet).Stop()
	case "block":
		defer profile.Start(profile.BlockProfile, profile.ProfilePath("."), profile.Quiet).Stop()
	default:
		// do nothing
	}

	// Register our custom Error hook
	log.AddHook(NewErrorHook(os.Stderr))

	// Enable runtime tracing if tracing is true
	if imageCOptions.tracing {
		tracing, err := os.Create(time.Now().Format("2006-01-02T150405.pprof"))
		if err != nil {
			log.Fatalf("Failed to create tracing logfile: %s", err)
		}
		defer tracing.Close()

		if err := trace.Start(tracing); err != nil {
			log.Fatalf("Failed to start tracing: %s", err)
		}
		defer trace.Stop()
	}

	// Open the log file
	// #nosec: Expect file permissions to be 0600 or less
	f, err := os.OpenFile(imageCOptions.logfile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		log.Fatalf("Failed to open the logfile %s: %s", imageCOptions.logfile, err)
	}
	defer f.Close()

	// Initiliaze logger with default TextFormatter
	log.SetFormatter(&log.TextFormatter{DisableColors: true, FullTimestamp: true})

	// Set the log level
	if imageCOptions.debug {
		log.SetLevel(log.DebugLevel)
	}

	// SetOutput to log file and/or stdout
	log.SetOutput(f)
	if imageCOptions.stdout {
		log.SetOutput(io.MultiWriter(os.Stdout, f))
	}

	switch imageCOptions.operation {
	case PullImage:
		options := imageCOptions.options

		options.Outstream = os.Stdout

		ic := imagec.NewImageC(options, streamformatter.NewJSONStreamFormatter())
		if err := ic.PullImage(); err != nil {
			log.Fatalf("Pulling image failed due to %s\n", err)
		}
	case PushImage:
		log.Errorf("The operation '%s' is not implemented\n", PushImage)
	case ListLayers:
		options := imageCOptions.options

		options.Outstream = os.Stdout

		ic := imagec.NewImageC(options, streamformatter.NewJSONStreamFormatter())
		if err := ic.ListLayers(); err != nil {
			log.Fatalf("Listing layers for image failed due to %s\n", err)
		}
	case Save:
		options := imageCOptions.options

		options.Outstream = os.Stdout

		ap := archiveProxy(options.Host)
		ic := imagec.NewImageC(options, streamformatter.NewJSONStreamFormatter())
		if err := saveImage(ap, ic); err != nil {
			log.Fatalf("Saving image %s failed due to %s\n", options.Reference.String(), err)
		}
	default:
		log.Errorf("The operation '%s' is not valid\n", imageCOptions.operation)
	}
}

func saveImage(ap proxy.VicArchiveProxy, ic *imagec.ImageC) error {
	log.Debugf("Save image %s", ic.Options.Reference)
	err := ic.ListLayers()
	if err != nil {
		return err
	}
	layers, err := ic.LayersToDownload()
	if err != nil {
		return err
	}

	dest := imagec.DestinationDirectory(ic.Options)
	var wg sync.WaitGroup
	errChan := make(chan error, len(layers))

	for i := len(layers) - 1; i >= 0; i-- {
		if layers[i].ID == constants.ScratchLayerID {
			continue
		}
		wg.Add(1)
		go func(pid, cid string) {
			defer wg.Done()
			var err error
			log.Debugf("parent id: %s, layer id: %s", pid, cid)

			defer func() {
				errChan <- err
			}()

			fileDir := path.Join(dest, cid)
			err = os.MkdirAll(fileDir, 0755) /* #nosec */
			if err != nil {
				return
			}
			filePath := path.Join(fileDir, cid+".tar")
			log.Debugf("save layer %s to file %s", cid, filePath)
			err = writeArchiveFile(ap, ic.ImageStore, ic.ImageStore, cid, pid, filePath)
		}(layers[i].Parent, layers[i].ID)
	}
	wg.Wait()
	close(errChan)

	var errs []string
	for e := range errChan {
		if e == nil {
			continue
		}
		errs = append(errs, e.Error())
	}
	if len(errs) > 0 {
		return fmt.Errorf("save image layers failed: %s", strings.Join(errs, ","))
	}
	return nil
}

func portlayerClient(portLayerAddr string) *apiclient.PortLayer {
	t := rc.New(portLayerAddr, "/", []string{"http"})
	t.Consumers["application/x-tar"] = runtime.ByteStreamConsumer()
	t.Consumers["application/octet-stream"] = runtime.ByteStreamConsumer()
	t.Producers["application/x-tar"] = runtime.ByteStreamProducer()
	t.Producers["application/octet-stream"] = runtime.ByteStreamProducer()

	client := apiclient.New(t, nil)
	return client
}

func archiveProxy(portLayerAddr string) proxy.VicArchiveProxy {
	plClient := portlayerClient(portLayerAddr)
	archiveProxy := proxy.NewArchiveProxy(plClient)

	return archiveProxy
}

func writeArchiveFile(archiveProxy proxy.VicArchiveProxy, store, ancestorStore, layerID, ancestorID, archivePath string) error {
	var filterSpec vicarchive.FilterSpec

	op := optrace.NewOperation(context.Background(), "export layer %s:%s", layerID, ancestorID)
	//Initialize an archive stream from the portlayer for the layer
	ar, err := archiveProxy.ArchiveExportReader(op, store, ancestorStore, layerID, ancestorID, true, filterSpec)
	if err != nil || ar == nil {
		return fmt.Errorf("Failed to get reader for layer %s", layerID)
	}

	log.Infof("Obtain archive reader for layer %s, parent %s", layerID, ancestorID)

	// #nosec - there's nothing secret/sensitive about these image layer files
	tarFile, err := os.OpenFile(archivePath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		msg := fmt.Sprintf("Failed to create tmp file: %s", err.Error())
		log.Info(msg)
		return fmt.Errorf(msg)
	}
	defer tarFile.Close()

	_, err = io.Copy(tarFile, ar)
	if err != nil {
		msg := fmt.Sprintf("Failed to read from acrhive stream: %s", err.Error())
		log.Info(msg)
		return fmt.Errorf(msg)
	}

	ar.Close()

	if err = tarFile.Sync(); err != nil {
		msg := fmt.Sprintf("Failed to flush tar file: %s", err.Error())
		log.Info(msg)
		return fmt.Errorf(msg)
	}

	return nil
}
