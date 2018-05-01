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

package admiral

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/go-openapi/runtime"
	rtclient "github.com/go-openapi/runtime/client"
	strfmt "github.com/go-openapi/strfmt"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"

	"github.com/vmware/govmomi/vim25/types"
	vchcfg "github.com/vmware/vic/lib/config"
	"github.com/vmware/vic/lib/config/dynamic"
	"github.com/vmware/vic/lib/config/dynamic/admiral/client"
	"github.com/vmware/vic/lib/config/dynamic/admiral/client/config_registries"
	"github.com/vmware/vic/lib/config/dynamic/admiral/client/projects"
	"github.com/vmware/vic/lib/config/dynamic/admiral/client/resources_compute"
	"github.com/vmware/vic/lib/config/dynamic/admiral/models"
	"github.com/vmware/vic/pkg/vsphere/session"
	"github.com/vmware/vic/pkg/vsphere/tags"
	"github.com/vmware/vic/pkg/vsphere/vm"
)

const (
	VicProductCategory = "VsphereIntegratedContainers"
	ProductVMTag       = "ProductVM"
	admiralEndpointKey = "guestinfo.vicova.admiral.endpoint"

	clusterFilter = "(address eq '%s' and customProperties.__containerHostType eq 'VCH')"

	// defaultVicRegistry is the query string to fetch the default VIC registry from Admiral
	defaultVicRegistry = "default-vic-registry"
)

// #nosec
const admiralTokenKey = "guestinfo.vicova.engine.token"

var (
	trueStr        = "true"
	projectsFilter = "customProperties.__enableContentTrust eq 'true'"

	// #nosec
	admClient = &http.Client{
		Transport: // copied from http.DefaultTransport
		&http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
				DualStack: true,
			}).DialContext,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
		},
	}
)

// NewSource creates a new Admiral dynamic config source. sess
// is a valid vsphere session object. vchID is a unique identifier
// that will be used to lookup the VCH in the Admiral instance; currently
// this is a URI to the docker endpoint in the VCH.
func NewSource(sess *session.Session, vchID string) dynamic.Source {
	return &source{
		d:     DefaultDiscovery,
		sess:  sess,
		vchID: vchID,
	}
}

var DefaultDiscovery = &productDiscovery{
	clients: make(map[string]*tags.RestClient),
}

type source struct {
	mu      sync.Mutex
	d       discovery
	sess    *session.Session
	vchID   string
	lastCfg *vchcfg.VirtualContainerHostConfigSpec

	// the discovered product VM
	v *vm.VirtualMachine
	// cached values to minimize running discovery
	projs []string
	c     *client.Admiral
}

// Get returns the dynamic config portion from an Admiral instance. For now,
// this is empty pending details from the Admiral team.
func (a *source) Get(ctx context.Context) (*vchcfg.VirtualContainerHostConfigSpec, error) {
	a.mu.Lock()
	lastCfg := a.lastCfg
	a.mu.Unlock()

	c, projs, err := a.discover(ctx)
	if err != nil {
		log.Warnf("could not locate management portal, returning last known config: %s", err)
		return lastCfg, nil
	}

	if len(projs) == 0 {
		return nil, nil
	}

	wl, err := a.whitelist(ctx, c, projs)
	if err != nil {
		log.Warnf("could not get whitelist from management portal, returning last known config: %s")
		return lastCfg, nil
	}

	log.Debugf("got whitelist: %+v", wl)

	newCfg := &vchcfg.VirtualContainerHostConfigSpec{
		Registry: vchcfg.Registry{RegistryWhitelist: wl},
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	err = nil
	if !a.diff(newCfg) {
		err = dynamic.ErrConfigNotModified
	}

	a.lastCfg = newCfg
	return newCfg, err
}

func (a *source) diff(newCfg *vchcfg.VirtualContainerHostConfigSpec) bool {
	if a.lastCfg == nil {
		if newCfg == nil {
			return false
		}

		return true
	}

	if newCfg == nil {
		return true
	}

	// compare whitelists
	if len(a.lastCfg.RegistryWhitelist) != len(newCfg.RegistryWhitelist) {
		return true
	}

	for _, w1 := range a.lastCfg.RegistryWhitelist {
		found := false
		for _, w2 := range newCfg.RegistryWhitelist {
			if w2 == w1 {
				found = true
				break
			}
		}

		if !found {
			return true
		}
	}

	return false
}

func (a *source) discover(ctx context.Context) (*client.Admiral, []string, error) {
	a.mu.Lock()
	c := a.c
	v := a.v
	a.mu.Unlock()

	var vms []*vm.VirtualMachine
	removed := true
	if c != nil {
		projs, err := a.projects(ctx, c)
		if err != nil {
			log.Debugf("could not get projects: %s", err)
			vms = append(vms, v)
			removed = false
		} else if len(projs) > 0 {
			return c, projs, nil
		}
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if c != a.c && a.c != nil {
		return a.c, a.projs, nil
	}

	var err error
	// if there isn't a list of potential product VMs
	// already, then run discovery
	if removed {
		log.Infof("running product VM discovery")
		vms, err = a.d.Discover(ctx, a.sess)
		if err != nil {
			return nil, nil, err
		}
	}

	for _, v := range vms {
		u, token, err := admiralEndpoint(ctx, v)
		if err != nil {
			log.Warnf("ignoring potential product VM %s: %s", v, err)
			continue
		}

		rt := rtclient.NewWithClient(u.Host, u.Path, []string{u.Scheme}, admClient)
		rt.DefaultAuthentication = &admiralAuth{token: token}
		c := client.New(rt, strfmt.Default)
		projs, err := a.projects(ctx, c)
		if err == nil && len(projs) > 0 {
			log.Infof("using Admiral endpoint at %s", u)
			a.c = c
			a.projs = projs
			a.v = v
			return c, projs, nil
		}

		log.Warnf("ignoring product VM Admiral endpoint %s since it does not have the VCH added to any project", u)
	}

	if removed {
		a.c = nil
		a.v = nil
		a.projs = nil
		err = nil
	} else {
		// only return source unavailable if
		// the VCH was not removed from the
		// Admiral instance
		err = dynamic.ErrSourceUnavailable
	}

	return nil, nil, err
}

func admiralEndpoint(ctx context.Context, v *vm.VirtualMachine) (*url.URL, string, error) {
	values, err := keys(ctx, v, []string{admiralEndpointKey, admiralTokenKey})
	if err != nil {
		return nil, "", err
	}

	token := values[admiralTokenKey]
	if token == "" {
		// not a useable product installation
		return nil, "", fmt.Errorf("empty token set in product VM %s", v)
	}

	u, err := url.Parse(values[admiralEndpointKey])
	if err != nil {
		return nil, "", fmt.Errorf("bad admiral endpoint %s in product VM %s: %s", values[admiralEndpointKey], v, err)
	}

	return u, token, nil
}

type admiralAuth struct {
	token string
}

func (a *admiralAuth) AuthenticateRequest(req runtime.ClientRequest, _ strfmt.Registry) error {
	return req.SetHeaderParam("x-xenon-auth-token", a.token)
}

func (a *source) projects(ctx context.Context, c *client.Admiral) ([]string, error) {
	ids := []string{a.vchID}
	if u, err := url.Parse(a.vchID); err == nil {
		if u.Scheme == "" {
			ids = append(ids, "https://"+a.vchID)
		} else {
			ids = append(ids, strings.TrimPrefix(a.vchID, u.Scheme+"://"))
		}
	}

	var err error
	var comps *resources_compute.GetResourcesComputeOK
	for _, vchID := range ids {
		filter := fmt.Sprintf(clusterFilter, vchID)
		log.Debugf("getting compute resources with filter %s", filter)
		comps, err = c.ResourcesCompute.GetResourcesCompute(resources_compute.NewGetResourcesComputeParamsWithContext(ctx).WithDollarFilter(&filter))
		if err == nil && comps.Payload.DocumentCount > 0 {
			break
		}
	}

	if err != nil {
		return nil, err
	}

	if comps.Payload.DocumentCount == 0 {
		return nil, nil
	}

	// more than one project has the VCH added, just pick the first one
	comp := &models.ComVmwarePhotonControllerModelResourcesComputeServiceComputeState{}
	if err := mapstructure.Decode(comps.Payload.Documents[comps.Payload.DocumentLinks[0]], comp); err != nil {
		return nil, err
	}

	return comp.TenantLinks, nil
}

func (a *source) whitelist(ctx context.Context, c *client.Admiral, hostProjs []string) ([]string, error) {
	// find at least one project with enable content trust
	// that also contains the vch
	projs, err := c.Projects.GetProjects(projects.NewGetProjectsParamsWithContext(ctx).WithDollarFilter(&projectsFilter))
	if err != nil {
		return nil, err
	}

	trust := false
	for _, t := range hostProjs {
		for _, p := range projs.Payload.DocumentLinks {
			if t == p {
				trust = true
				break
			}
		}

		if trust {
			break
		}
	}

	if !trust {
		// no project with enable content trust and vch
		return nil, nil
	}

	regs, err := c.ConfigRegistries.GetConfigRegistriesID(config_registries.NewGetConfigRegistriesIDParamsWithContext(ctx).WithID(defaultVicRegistry))
	if err != nil {
		return nil, err
	}
	if regs.Payload == nil {
		// No default VIC registry configured.
		log.Warnf("no default VIC registry found despite content trust being enabled")
		return nil, nil
	}

	m := regs.Payload
	wl := []string{m.Address}

	return wl, nil
}

type discovery interface {
	Discover(ctx context.Context, sess *session.Session) ([]*vm.VirtualMachine, error)
}

type productDiscovery struct {
	mu      sync.Mutex
	clients map[string]*tags.RestClient
}

func (o *productDiscovery) Discover(ctx context.Context, sess *session.Session) ([]*vm.VirtualMachine, error) {
	service, err := url.Parse(sess.Service)
	if err != nil {
		return nil, err
	}

	service.User = sess.User

	o.mu.Lock()
	t := o.clients[service.String()]
	if t == nil {
		t = tags.NewClient(service, sess.Insecure, sess.Thumbprint)
		o.clients[service.String()] = t
	}
	o.mu.Unlock()

	var tag string
	tag, err = findOVATag(ctx, t)
	if err != nil {
		err = errors.Errorf("could not find ova tag: %s", err)
		return nil, err
	}

	objs, err := t.ListAttachedObjects(ctx, tag)
	if err != nil || len(objs) == 0 {
		err = errors.Errorf("could not find ova vm: %s", err)
		return nil, err
	}

	var vms []*vm.VirtualMachine
	for _, o := range objs {
		if o.Type == nil || o.ID == nil {
			log.Warnf("skipping invalid object reference %+v", o)
			continue
		}

		if *o.Type != "VirtualMachine" {
			// not a virtual machine
			continue
		}

		v := vm.NewVirtualMachine(ctx, sess, types.ManagedObjectReference{Type: *o.Type, Value: *o.ID})
		st, err := v.PowerState(ctx)
		if err != nil {
			log.Warnf("ignoring potential product VM: %s", err)
			continue
		}

		if st != types.VirtualMachinePowerStatePoweredOn {
			log.Warnf("ignoring potential product VM %s: not powered on", *o.ID)
			continue
		}

		vms = append(vms, v)
	}

	return vms, nil
}

func findOVATag(ctx context.Context, t *tags.RestClient) (string, error) {
	cats, err := t.GetCategoriesByName(ctx, VicProductCategory)
	if err != nil {
		return "", err
	}

	// just use the first one
	if len(cats) == 0 {
		return "", errors.New("could not find tag")
	}

	cat := cats[0]
	tags, err := t.GetTagByNameForCategory(ctx, ProductVMTag, cat.ID)
	if err != nil {
		return "", err
	}

	if len(tags) == 0 {
		return "", errors.New("could not find tag")
	}

	return tags[0].ID, nil
}

func keys(ctx context.Context, v *vm.VirtualMachine, keys []string) (map[string]string, error) {
	ovs, err := v.FetchExtraConfigBaseOptions(ctx)
	if err != nil {
		return nil, err
	}

	res := make(map[string]string)
	for _, k := range keys {
		found := false
		for _, ov := range ovs {
			log.Debugf("key: %s", ov.GetOptionValue().Key)
			if k == ov.GetOptionValue().Key {
				log.Debugf("found %s", ov.GetOptionValue().Key)
				res[k] = ov.GetOptionValue().Value.(string)
				found = true
				break
			}
		}

		if !found {
			return nil, errors.Errorf("key not found: %s", k)
		}
	}

	return res, nil
}
