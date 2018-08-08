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

package backends

//****
// system.go
//
// Rules for code to be in here:
// 1. No remote or swagger calls.  Move those code to ../proxy/system_proxy.go
// 2. Always return docker engine-api compatible errors.
//		- Do NOT return fmt.Errorf()
//		- Do NOT return errors.New()
//		- DO USE the aliased docker error package 'derr'
//		- It is OK to return errors returned from functions in system_proxy.go

import (
	"crypto/x509"
	"fmt"
	"net/url"
	"runtime"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/context"

	"github.com/vmware/vic/lib/apiservers/engine/backends/cache"
	"github.com/vmware/vic/lib/apiservers/engine/errors"
	"github.com/vmware/vic/lib/apiservers/engine/proxy"
	"github.com/vmware/vic/lib/apiservers/portlayer/client"
	"github.com/vmware/vic/lib/apiservers/portlayer/client/storage"
	"github.com/vmware/vic/lib/imagec"
	urlfetcher "github.com/vmware/vic/pkg/fetcher"
	"github.com/vmware/vic/pkg/registry"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/version"

	"github.com/docker/docker/api/types"
	eventtypes "github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/daemon/events"
	"github.com/docker/docker/pkg/platform"
	"github.com/docker/go-units"
)

type SystemBackend struct {
	systemProxy proxy.SystemProxy
}

const (
	systemStatusMhz          = " VCH CPU limit"
	systemStatusMemory       = " VCH memory limit"
	systemStatusCPUUsageMhz  = " VCH CPU usage"
	systemStatusMemUsage     = " VCH memory usage"
	systemOS                 = " VMware OS"
	systemOSVersion          = " VMware OS version"
	systemProductName        = " VMware Product"
	volumeStoresID           = "VolumeStores"
	loginTimeout             = 20 * time.Second
	infoTimeout              = 5 * time.Second
	vchWhitelistMode         = " Registry Whitelist Mode"
	whitelistRegistriesLabel = " Whitelisted Registries"
	insecureRegistriesLabel  = " Insecure Registries"
)

// var for use by other engine components
var systemBackend *SystemBackend
var sysOnce sync.Once

func NewSystemBackend() *SystemBackend {
	sysOnce.Do(func() {
		systemBackend = &SystemBackend{
			systemProxy: proxy.NewSystemProxy(PortLayerClient()),
		}
	})
	return systemBackend
}

func (s *SystemBackend) SystemInfo() (*types.Info, error) {
	op := trace.NewOperation(context.Background(), "SystemInfo")
	defer trace.End(trace.Audit("", op))

	client := PortLayerClient()

	// Retrieve container status from port layer
	running, paused, stopped, err := s.systemProxy.ContainerCount(context.Background())
	if err != nil {
		op.Infof("System.SytemInfo unable to get global status on containers: %s", err.Error())
	}

	ctx, cancel := context.WithTimeout(context.Background(), infoTimeout)
	defer cancel()

	vchConfig := vchConfig.update(ctx)

	vchConfig.Lock()
	defer vchConfig.Unlock()

	cfg := vchConfig.Cfg

	// Build up the struct that the Remote API and CLI wants
	info := &types.Info{
		Driver:             PortLayerName(),
		IndexServerAddress: imagec.DefaultDockerURL,
		ServerVersion:      ProductVersion(),
		ID:                 ProductName(),
		Containers:         running + paused + stopped,
		ContainersRunning:  running,
		ContainersPaused:   paused,
		ContainersStopped:  stopped,
		Images:             getImageCount(),
		Debug:              cfg.Diagnostics.DebugLevel > 0,
		NGoroutines:        runtime.NumGoroutine(),
		SystemTime:         time.Now().Format(time.RFC3339Nano),
		LoggingDriver:      "",
		CgroupDriver:       "",
		DockerRootDir:      "",
		ClusterStore:       "",
		ClusterAdvertise:   "",

		// FIXME: Get this info once we have event listening service
		//	NEventsListener    int

		// These are system related.  Some refer to cgroup info.  Others are
		// retrieved from the port layer and are information about the resource
		// pool.
		Name:          cfg.Name,
		KernelVersion: "",
		Architecture:  platform.Architecture, //stubbed

		// NOTE: These values have no meaning for VIC.  We default them to true to
		// prevent the CLI from displaying warning messages.
		CPUCfsPeriod:      true,
		CPUCfsQuota:       true,
		CPUShares:         true,
		CPUSet:            true,
		OomKillDisable:    true,
		MemoryLimit:       true,
		SwapLimit:         true,
		KernelMemory:      true,
		IPv4Forwarding:    true,
		BridgeNfIptables:  true,
		BridgeNfIP6tables: true,
		HTTPProxy:         "",
		HTTPSProxy:        "",
		NoProxy:           "",
	}

	// Add in vicnetwork info from the VCH via guestinfo
	for _, network := range cfg.ContainerNetworks {
		info.Plugins.Network = append(info.Plugins.Network, network.Name)
	}

	info.SystemStatus = make([][2]string, 0)

	// Add in volume label from the VCH via guestinfo
	volumeStoreString, err := FetchVolumeStores(op, client)
	if err != nil {
		op.Infof("Unable to get the volume store list from the portlayer : %s", err.Error())
	} else {
		customInfo := [2]string{volumeStoresID, volumeStoreString}
		info.SystemStatus = append(info.SystemStatus, customInfo)

		// Show a list of supported volume drivers if there's at least one volume
		// store configured for the VCH. "local" is excluded because it's the default
		// driver supplied by the Docker client and is equivalent to "vsphere" in
		// our implementation.
		if len(volumeStoreString) > 0 {
			for driver := range proxy.SupportedVolDrivers {
				if driver != "local" {
					info.Plugins.Volume = append(info.Plugins.Volume, driver)
				}
			}
		}
	}

	if s.systemProxy.PingPortlayer(context.Background()) {
		status := [2]string{PortLayerName(), "RUNNING"}
		info.SystemStatus = append(info.SystemStatus, status)
	} else {
		status := [2]string{PortLayerName(), "STOPPED"}
		info.SystemStatus = append(info.SystemStatus, status)
	}

	// Add in vch information
	vchInfo, err := s.systemProxy.VCHInfo(context.Background())
	if err != nil || vchInfo == nil {
		op.Infof("System.SystemInfo unable to get vch info from port layer: %s", err.Error())
	} else {
		if vchInfo.CPUMhz > 0 {
			info.NCPU = int(vchInfo.CPUMhz)

			customInfo := [2]string{systemStatusMhz, fmt.Sprintf("%d MHz", info.NCPU)}
			info.SystemStatus = append(info.SystemStatus, customInfo)
		}
		if vchInfo.Memory > 0 {
			info.MemTotal = vchInfo.Memory * 1024 * 1024 // Get Mebibytes

			customInfo := [2]string{systemStatusMemory, units.BytesSize(float64(info.MemTotal))}
			info.SystemStatus = append(info.SystemStatus, customInfo)
		}
		if vchInfo.CPUUsage >= 0 {
			customInfo := [2]string{systemStatusCPUUsageMhz, fmt.Sprintf("%d MHz", int(vchInfo.CPUUsage))}
			info.SystemStatus = append(info.SystemStatus, customInfo)
		}
		if vchInfo.MemUsage >= 0 {
			customInfo := [2]string{systemStatusMemUsage, units.BytesSize(float64(vchInfo.MemUsage))}
			info.SystemStatus = append(info.SystemStatus, customInfo)
		}
		if vchInfo.HostProductName != "" {
			customInfo := [2]string{systemProductName, vchInfo.HostProductName}
			info.SystemStatus = append(info.SystemStatus, customInfo)
		}
		if vchInfo.HostOS != "" {
			info.OperatingSystem = vchInfo.HostOS
			info.OSType = vchInfo.HostOS //Value for OS and OS Type the same from vmomi

			customInfo := [2]string{systemOS, vchInfo.HostOS}
			info.SystemStatus = append(info.SystemStatus, customInfo)
		}
		if vchInfo.HostOSVersion != "" {
			customInfo := [2]string{systemOSVersion, vchInfo.HostOSVersion}
			info.SystemStatus = append(info.SystemStatus, customInfo)
		}
		if len(vchConfig.Insecure) > 0 {
			customInfo := [2]string{insecureRegistriesLabel, strings.Join(vchConfig.Insecure.Strings(), ",")}
			info.SystemStatus = append(info.SystemStatus, customInfo)
		}
		if len(vchConfig.Whitelist) > 0 {
			s := "enabled"
			if vchConfig.remoteWl {
				s += "; remote source"
			}
			customInfo := [2]string{vchWhitelistMode, s}
			info.SystemStatus = append(info.SystemStatus, customInfo)
			customInfo = [2]string{whitelistRegistriesLabel, strings.Join(vchConfig.Whitelist.Strings(), ",")}
			info.SystemStatus = append(info.SystemStatus, customInfo)
		} else {
			customInfo := [2]string{vchWhitelistMode, "disabled.  All registry access allowed."}
			info.SystemStatus = append(info.SystemStatus, customInfo)
		}
	}

	return info, nil
}

// layout for build time as per constants defined in https://golang.org/src/time/format.go
const buildTimeLayout = "2006/01/02@15:04:05"

func (s *SystemBackend) SystemVersion() types.Version {
	op := trace.NewOperation(context.Background(), "SystemVersion")
	defer trace.End(trace.Audit("", op))

	Arch := runtime.GOARCH

	BuildTime := version.BuildDate
	if t, err := time.Parse(buildTimeLayout, BuildTime); err == nil {
		// match time format from docker version's output
		BuildTime = t.Format(time.ANSIC)
	}

	Experimental := true
	GitCommit := version.GitCommit
	GoVersion := runtime.Version()
	// FIXME: fill with real kernel version
	KernelVersion := "-"
	Os := runtime.GOOS
	Version := version.Version
	if Version != "" && Version[0] == 'v' {
		// match version format from docker version's output
		Version = Version[1:]
	}

	// go runtime panics without this so keep this here
	// until we find a repro case and report it to upstream
	_ = Arch

	version := types.Version{
		APIVersion:    version.DockerAPIVersion,
		MinAPIVersion: version.DockerMinimumVersion,
		Arch:          Arch,
		BuildTime:     BuildTime,
		Experimental:  Experimental,
		GitCommit:     GitCommit,
		GoVersion:     GoVersion,
		KernelVersion: KernelVersion,
		Os:            Os,
		Version:       Version,
	}

	op.Infof("***** version = %#v", version)

	return version
}

// SystemCPUMhzLimit will return the VCH configured Mhz limit
func (s *SystemBackend) SystemCPUMhzLimit() (int64, error) {
	vchInfo, err := s.systemProxy.VCHInfo(context.Background())
	if err != nil || vchInfo == nil {
		return 0, err
	}
	return vchInfo.CPUMhz, nil
}

func (s *SystemBackend) SystemDiskUsage() (*types.DiskUsage, error) {
	op := trace.NewOperation(context.Background(), "SystemDiskUsage")
	defer trace.End(trace.Audit("", op))

	return nil, errors.APINotSupportedMsg(ProductName(), "SystemDiskUsage")
}

func (s *SystemBackend) SubscribeToEvents(since, until time.Time, filter filters.Args) ([]eventtypes.Message, chan interface{}) {
	defer trace.End(trace.Begin(""))

	ef := events.NewFilter(filter)
	return EventService().SubscribeTopic(since, until, ef)
}

func (s *SystemBackend) UnsubscribeFromEvents(listener chan interface{}) {
	defer trace.End(trace.Begin(""))
	EventService().Evict(listener)
}

// AuthenticateToRegistry handles docker logins
func (s *SystemBackend) AuthenticateToRegistry(ctx context.Context, authConfig *types.AuthConfig) (string, string, error) {
	op := trace.NewOperation(context.Background(), "AuthenticateToRegistry")
	defer trace.End(trace.Audit("", op))

	// Only look at V2 registries
	registryAddress := authConfig.ServerAddress
	if !strings.Contains(authConfig.ServerAddress, "/v2") {
		registryAddress = registryAddress + "/v2/"
	}

	if !strings.HasPrefix(registryAddress, "http") {
		registryAddress = "//" + registryAddress
	}

	loginURL, err := url.Parse(registryAddress)
	if err != nil {
		msg := fmt.Sprintf("Bad login address: %s", registryAddress)
		op.Errorf(msg)
		return msg, "", err
	}

	// Check if registry is contained within whitelisted or insecure registries
	regctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	whitelistOk, _, insecureOk := vchConfig.RegistryCheck(regctx, loginURL)
	if !whitelistOk {
		msg := fmt.Sprintf("Access denied to unauthorized registry (%s) while VCH is in whitelist mode", loginURL.Host)
		return msg, "", fmt.Errorf(msg)
	}

	var certPool *x509.CertPool
	if insecureOk {
		op.Infof("Attempting to log into %s insecurely", loginURL.Host)
		certPool = nil
	} else {
		certPool = RegistryCertPool
	}

	dologin := func(scheme string, skipVerify bool) (string, error) {
		loginURL.Scheme = scheme

		var authURL *url.URL

		fetcher := urlfetcher.NewURLFetcher(urlfetcher.Options{
			Timeout:            loginTimeout,
			Username:           authConfig.Username,
			Password:           authConfig.Password,
			RootCAs:            certPool,
			InsecureSkipVerify: skipVerify,
		})

		// Attempt to get the Auth URL from a simple ping operation (GET) to the registry
		hdr, err := fetcher.Ping(loginURL)
		if err == nil {
			if fetcher.IsStatusUnauthorized() {
				op.Debugf("Looking up OAuth URL from server %s", loginURL)
				authURL, err = fetcher.ExtractOAuthURL(hdr.Get("www-authenticate"), nil)
			} else {
				// We're not suppose to be here, but if we do end up here, use the login
				//	URL for the auth URL.
				authURL = loginURL
			}
		}
		if err != nil {
			op.Errorf("Looking up OAuth URL failed: %s", err)
			return "", err
		}

		op.Debugf("logging onto %s", authURL.String())

		// Just check if we get a token back.
		token, err := fetcher.FetchAuthToken(authURL)
		if err != nil || token.Token == "" {
			// At this point, if a request cannot be solved by a retry, it is an authentication error.
			op.Errorf("Fetch auth token failed: %s", err)
			if _, ok := err.(urlfetcher.DoNotRetry); ok {
				err = fmt.Errorf("Get %s: unauthorized: incorrect username or password", loginURL)
			} else {
				err = urlfetcher.AuthTokenError{TokenServer: *authURL}
			}
			return "", err
		}

		return token.Token, nil
	}

	_, err = dologin("https", insecureOk)
	if err != nil && insecureOk {
		_, err = dologin("http", insecureOk)
	}

	if err != nil {
		return "", "", err
	}

	// We don't return the token.  The config.json will store token if we return
	// it, but the regular docker daemon doesn't seem to return it either.
	return "Login Succeeded", "", nil
}

// Utility functions

func getImageCount() int {
	images := cache.ImageCache().GetImages()
	return len(images)
}

func FetchVolumeStores(op trace.Operation, client *client.PortLayer) (string, error) {

	res, err := client.Storage.VolumeStoresList(storage.NewVolumeStoresListParamsWithContext(op))
	if err != nil {
		return "", err
	}

	return strings.Join(res.Payload.Stores, " "), nil
}

func entryStrJoin(entries registry.Set, sep string) string {
	var s string
	for _, e := range entries {
		s += e.String() + sep
	}

	return s[:len(s)-len(sep)]
}
