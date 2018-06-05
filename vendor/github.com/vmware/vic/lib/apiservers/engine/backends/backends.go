// Copyright 2016 VMware, Inc. All Rights Reserved.
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
	"context"
	"crypto/x509"
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/events"
	"github.com/go-openapi/runtime"
	rc "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/swag"
	"golang.org/x/sync/singleflight"

	"github.com/vmware/vic/lib/apiservers/engine/backends/cache"
	"github.com/vmware/vic/lib/apiservers/engine/backends/container"
	"github.com/vmware/vic/lib/apiservers/engine/network"
	"github.com/vmware/vic/lib/apiservers/engine/proxy"
	apiclient "github.com/vmware/vic/lib/apiservers/portlayer/client"
	"github.com/vmware/vic/lib/apiservers/portlayer/client/containers"
	"github.com/vmware/vic/lib/apiservers/portlayer/client/misc"
	"github.com/vmware/vic/lib/apiservers/portlayer/client/scopes"
	"github.com/vmware/vic/lib/apiservers/portlayer/client/storage"
	"github.com/vmware/vic/lib/apiservers/portlayer/models"
	"github.com/vmware/vic/lib/config"
	"github.com/vmware/vic/lib/config/dynamic"
	"github.com/vmware/vic/lib/config/dynamic/admiral"
	"github.com/vmware/vic/lib/constants"
	"github.com/vmware/vic/lib/imagec"
	"github.com/vmware/vic/pkg/errors"
	"github.com/vmware/vic/pkg/registry"
	"github.com/vmware/vic/pkg/vsphere/session"
	"github.com/vmware/vic/pkg/vsphere/sys"
)

const (
	PortlayerName = "Backend Engine"

	// RetryTimeSeconds defines how many seconds to wait between retries
	RetryTimeSeconds        = 2
	defaultSessionKeepAlive = 20 * time.Second
	APITimeout              = constants.PropertyCollectorTimeout + 3*time.Second
)

var (
	portLayerClient     *apiclient.PortLayer
	portLayerServerAddr string
	portLayerName       string
	productName         string
	productVersion      string

	vchConfig        *dynConfig
	RegistryCertPool *x509.CertPool
	archiveProxy     proxy.VicArchiveProxy

	eventService *events.Events

	servicePort uint
)

type dynConfig struct {
	sync.Mutex

	Cfg    *config.VirtualContainerHostConfigSpec
	src    dynamic.Source
	merger dynamic.Merger
	sess   *session.Session

	Whitelist, Blacklist, Insecure registry.Set
	remoteWl                       bool

	group   singleflight.Group
	lastCfg *dynConfig
}

func Init(portLayerAddr, product string, port uint, config *config.VirtualContainerHostConfigSpec) error {
	servicePort = port
	_, _, err := net.SplitHostPort(portLayerAddr)
	if err != nil {
		return err
	}

	if config == nil {
		return fmt.Errorf("docker API server requires VCH config")
	}

	productName = product

	if config.Version != nil {
		productVersion = config.Version.ShortVersion()
	}
	if productVersion == "" {
		portLayerName = product + " Backend Engine"
	} else {
		portLayerName = product + " " + productVersion + " Backend Engine"
	}

	if vchConfig, err = newDynConfig(ctx, config); err != nil {
		return err
	}

	loadRegistryCACerts()

	t := rc.New(portLayerAddr, "/", []string{"http"})
	t.Consumers["application/x-tar"] = runtime.ByteStreamConsumer()
	t.Consumers["application/octet-stream"] = runtime.ByteStreamConsumer()
	t.Producers["application/x-tar"] = runtime.ByteStreamProducer()
	t.Producers["application/octet-stream"] = runtime.ByteStreamProducer()

	portLayerClient = apiclient.New(t, nil)
	portLayerServerAddr = portLayerAddr

	log.Infof("*** Portlayer Address = %s", portLayerAddr)

	// block indefinitely while waiting on the portlayer to respond to pings
	// the vic-machine installer timeout will intervene if this blocks for too long
	pingPortLayer()

	if err := hydrateCaches(); err != nil {
		return err
	}

	log.Info("Creating image store")
	if err := createImageStore(); err != nil {
		log.Errorf("Failed to create image store")
		return err
	}

	archiveProxy = proxy.NewArchiveProxy(portLayerClient)

	eventService = events.New()

	return nil
}

func hydrateCaches() error {
	const waiters = 3

	wg := sync.WaitGroup{}
	wg.Add(waiters)
	errChan := make(chan error, waiters)

	go func() {
		defer wg.Done()
		if err := imagec.InitializeLayerCache(portLayerClient); err != nil {
			errChan <- fmt.Errorf("Failed to initialize layer cache: %s", err)
			return
		}
		log.Info("Layer cache initialized successfully")
		errChan <- nil
	}()

	go func() {
		defer wg.Done()
		if err := cache.InitializeImageCache(portLayerClient); err != nil {
			errChan <- fmt.Errorf("Failed to initialize image cache: %s", err)
			return
		}
		log.Info("Image cache initialized successfully")

		// container cache relies on image cache so we share a goroutine to update
		// them serially
		if err := syncContainerCache(); err != nil {
			errChan <- fmt.Errorf("Failed to update container cache: %s", err)
			return
		}
		log.Info("Container cache updated successfully")
		errChan <- nil
	}()

	go func() {
		log.Info("Refreshing repository cache")
		defer wg.Done()
		if err := cache.NewRepositoryCache(portLayerClient); err != nil {
			errChan <- fmt.Errorf("Failed to create repository cache: %s", err.Error())
			return
		}
		errChan <- nil
		log.Info("Repository cache updated successfully")
	}()

	wg.Wait()
	close(errChan)

	var errs []string
	for err := range errChan {
		if err != nil {
			// accumulate all errors into one
			errs = append(errs, err.Error())
		}
	}

	var e error
	if len(errs) > 0 {
		e = fmt.Errorf(strings.Join(errs, ", "))
	}

	if e != nil {
		log.Errorf("Errors occurred during cache hydration at VCH start: %s", e)
	}

	return e
}

func PortLayerClient() *apiclient.PortLayer {
	return portLayerClient
}

func PortLayerServer() string {
	return portLayerServerAddr
}

func PortLayerName() string {
	return portLayerName
}

func ProductName() string {
	return productName
}

func ProductVersion() string {
	return productVersion
}

func pingPortLayer() {
	ticker := time.NewTicker(RetryTimeSeconds * time.Second)
	defer ticker.Stop()
	params := misc.NewPingParamsWithContext(context.TODO())

	log.Infof("Waiting for portlayer to come up")

	for range ticker.C {
		if _, err := portLayerClient.Misc.Ping(params); err == nil {
			log.Info("Portlayer is up and responding to pings")
			return
		}
	}
}

func createImageStore() error {
	// TODO(jzt): we should move this to a utility package or something
	host, err := sys.UUID()
	if err != nil {
		log.Errorf("Failed to determine host UUID")
		return err
	}

	log.Infof("*** UUID = %s", host)

	// attempt to create the image store if it doesn't exist
	store := &models.ImageStore{Name: host}
	_, err = portLayerClient.Storage.CreateImageStore(
		storage.NewCreateImageStoreParamsWithContext(ctx).WithBody(store),
	)

	if err != nil {
		if _, ok := err.(*storage.CreateImageStoreConflict); ok {
			log.Debugf("Store already exists")
			return nil
		}
		return err
	}
	log.Infof("Image store created successfully")
	return nil
}

// syncContainerCache runs once at startup to populate the container cache
func syncContainerCache() error {
	log.Debugf("Updating container cache")

	backend := NewContainerBackend()
	client := PortLayerClient()

	reqParams := containers.NewGetContainerListParamsWithContext(ctx).WithAll(swag.Bool(true))
	containme, err := client.Containers.GetContainerList(reqParams)
	if err != nil {
		return errors.Errorf("Failed to retrieve container list from portlayer: %s", err)
	}

	log.Debugf("Found %d containers", len(containme.Payload))
	cc := cache.ContainerCache()
	var errs []string
	for _, info := range containme.Payload {
		container := proxy.ContainerInfoToVicContainer(*info, portLayerName)
		cc.AddContainer(container)
		if err = setPortMapping(info, backend, container); err != nil {
			errs = append(errs, err.Error())
		}
	}

	if len(errs) > 0 {
		return errors.Errorf("Failed to set port mapping: %s", strings.Join(errs, "\n"))
	}
	return nil
}

func setPortMapping(info *models.ContainerInfo, backend *ContainerBackend, container *container.VicContainer) error {
	if info.ContainerConfig.State == "" {
		log.Infof("container state is nil")
		return nil
	}

	if info.ContainerConfig.State != "Running" || len(container.HostConfig.PortBindings) == 0 {
		log.Infof("No need to restore port bindings, state: %s, portbinding: %+v", info.ContainerConfig.State, container.HostConfig.PortBindings)
		return nil
	}

	log.Debugf("Set port mapping for container %q, portmapping %+v", container.Name, container.HostConfig.PortBindings)
	client := PortLayerClient()
	endpointsOK, err := client.Scopes.GetContainerEndpoints(
		scopes.NewGetContainerEndpointsParamsWithContext(ctx).WithHandleOrID(container.ContainerID))
	if err != nil {
		return err
	}
	for _, e := range endpointsOK.Payload {
		if len(e.Ports) > 0 && e.Scope == constants.BridgeScopeType {
			if err = network.MapPorts(container, e, container.ContainerID); err != nil {
				log.Errorf(err.Error())
				return err
			}
		}
	}
	return nil
}

func loadRegistryCACerts() {
	var err error

	RegistryCertPool, err = x509.SystemCertPool()
	log.Debugf("Loaded %d CAs for registries from system CA bundle", len(RegistryCertPool.Subjects()))
	if err != nil {
		log.Errorf("Unable to load system CAs")
		return
	}

	vchConfig.Lock()
	defer vchConfig.Unlock()
	if !RegistryCertPool.AppendCertsFromPEM(vchConfig.Cfg.RegistryCertificateAuthorities) {
		log.Errorf("Unable to load CAs for registry access in config")
		return
	}

	log.Debugf("Loaded %d CAs for registries from config", len(RegistryCertPool.Subjects()))
}

func EventService() *events.Events {
	return eventService
}

// RegistryCheck checkes the given url against the registry whitelist, blacklist, and insecure
// registries lists. It returns true for each list where u matches that list.
func (d *dynConfig) RegistryCheck(ctx context.Context, u *url.URL) (wl bool, bl bool, insecure bool) {
	m := d.update(ctx)

	us := u.String()
	wl = len(m.Whitelist) == 0 || m.Whitelist.Match(us)
	bl = len(m.Blacklist) == 0 || !m.Blacklist.Match(us)
	insecure = m.Insecure.Match(us)
	return
}

func (d *dynConfig) update(ctx context.Context) *dynConfig {
	const key = "RegistryCheck"
	resCh := d.group.DoChan(key, func() (interface{}, error) {
		d.Lock()
		src := d.src
		d.Unlock()

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		c, err := src.Get(ctx)
		if err != nil {
			log.Warnf("error getting config from source: %s", err)
		}

		d.Lock()
		defer d.Unlock()

		m := d
		if c != nil {
			// update config
			if m, err = d.merged(c); err != nil {
				log.Errorf("error updating config: %s", err)
				m = d
			} else {
				if len(c.RegistryWhitelist) > 0 {
					m.remoteWl = true
				}
			}
		} else if err == nil && src == d.src {
			// err == nil and c == nil, which
			// indicates no remote sources
			// were found, try resetting the
			// source for next time
			if err := d.resetSrc(); err != nil {
				log.Warnf("could not reset config source: %s", err)
			}
		}

		d.lastCfg = m
		return m, nil
	})

	select {
	case res := <-resCh:
		return res.Val.(*dynConfig)
	case <-ctx.Done():
		return func() *dynConfig {
			d.Lock()
			defer d.Unlock()

			if d.lastCfg == nil {
				return d
			}

			return d.lastCfg
		}()
	}
}

func (d *dynConfig) resetSrc() error {
	ep, err := d.clientEndpoint()
	if err != nil {
		return err
	}

	d.src = admiral.NewSource(d.sess, ep.String())
	return nil
}

func newDynConfig(ctx context.Context, c *config.VirtualContainerHostConfigSpec) (*dynConfig, error) {
	d := &dynConfig{
		Cfg: c,
	}
	var err error
	if d.Insecure, err = dynamic.ParseRegistries(c.InsecureRegistries); err != nil {
		return nil, err
	}
	if d.Whitelist, err = dynamic.ParseRegistries(c.RegistryWhitelist); err != nil {
		return nil, err
	}

	if d.sess, err = newSession(ctx, c); err != nil {
		return nil, err
	}

	d.merger = dynamic.NewMerger()
	if err := d.resetSrc(); err != nil {
		return nil, err
	}

	return d, nil
}

// update merges another config into this config. d should be locked before
// calling this.
func (d *dynConfig) merged(c *config.VirtualContainerHostConfigSpec) (*dynConfig, error) {
	if c == nil {
		return d, nil
	}

	newcfg, err := d.merger.Merge(d.Cfg, c)
	if err != nil {
		return nil, err
	}

	var wl, bl, insecure registry.Set
	if wl, err = dynamic.ParseRegistries(newcfg.RegistryWhitelist); err != nil {
		return nil, err
	}
	if bl, err = dynamic.ParseRegistries(newcfg.RegistryBlacklist); err != nil {
		return nil, err
	}
	if insecure, err = dynamic.ParseRegistries(newcfg.InsecureRegistries); err != nil {
		return nil, err
	}

	return &dynConfig{
		Whitelist: wl,
		Blacklist: bl,
		Insecure:  insecure,
		Cfg:       newcfg,
		src:       d.src,
	}, nil
}

func (d *dynConfig) clientEndpoint() (*url.URL, error) {
	ips, err := net.LookupIP("client.localhost")
	if err != nil {
		return nil, err
	}

	scheme := "https"
	if d.Cfg.HostCertificate.IsNil() {
		scheme = "http"
	}

	return url.Parse(fmt.Sprintf("%s://%s:%d", scheme, ips[0], servicePort))
}

func newSession(ctx context.Context, config *config.VirtualContainerHostConfigSpec) (*session.Session, error) {
	// strip the path off of the target url since it may contain the
	// datacenter
	u, err := url.Parse(config.Target)
	if err != nil {
		return nil, err
	}

	u.Path = ""
	sessCfg := &session.Config{
		Service:    u.String(),
		User:       url.UserPassword(config.Username, config.Token),
		Thumbprint: config.TargetThumbprint,
		Keepalive:  defaultSessionKeepAlive,
	}

	sess := session.NewSession(sessCfg)
	if sess.Connect(ctx); err != nil {
		return nil, err
	}

	return sess, nil
}
