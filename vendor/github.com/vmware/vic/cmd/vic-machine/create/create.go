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

package create

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/url"
	"path"
	"reflect"
	"strings"
	"time"

	"gopkg.in/urfave/cli.v1"

	"github.com/vmware/vic/cmd/vic-machine/common"
	"github.com/vmware/vic/lib/constants"
	"github.com/vmware/vic/lib/install/data"
	"github.com/vmware/vic/lib/install/management"
	"github.com/vmware/vic/lib/install/validate"
	"github.com/vmware/vic/lib/install/vchlog"
	"github.com/vmware/vic/pkg/errors"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/version"
)

const (
	// Max permitted length of Virtual Machine name
	MaxVirtualMachineNameLen = 80
	// Max permitted length of Virtual Switch name
	MaxDisplayNameLen = 31
)

// Create has all input parameters for vic-machine create command
type Create struct {
	common.Networks
	*data.Data
	Certs             common.CertFactory
	containerNetworks common.CNetworks
	Registries        common.Registries

	volumeStores common.VolumeStores

	Nameservers common.DNS

	memoryReservLimits string
	cpuReservLimits    string

	help          common.Help
	BridgeIPRange string

	Proxies common.Proxies

	SyslogAddr string

	executor *management.Dispatcher
}

func NewCreate() *Create {
	create := &Create{}
	create.Data = data.NewData()

	return create
}

// SetFields iterates through the fields in the Create struct, searching for fields
// tagged with the `arg` key. If the value of that tag matches the supplied `flag`
// string, a nil check is performed. If the field is not nil, then the user supplied
// this flag on the command line and we need to persist it.
// This is a workaround for cli.Context.IsSet() returning false when
// the short option for a cli.StringSlice is supplied instead of the long option.
// See https://github.com/urfave/cli/issues/314
func (c *Create) SetFields() map[string]struct{} {
	result := make(map[string]struct{})
	create := reflect.ValueOf(c).Elem()
	for i := 0; i < create.NumField(); i++ {
		t := create.Type().Field(i)
		if tag := t.Tag.Get("arg"); tag != "" {
			ss := create.Field(i)
			if !ss.IsNil() {
				result[tag] = struct{}{}
			}
		}
	}
	return result
}

// Flags return all cli flags for create
func (c *Create) Flags() []cli.Flag {
	create := []cli.Flag{
		// images
		cli.StringFlag{
			Name:        "image-store, i",
			Value:       "",
			Usage:       "Image datastore path in format \"datastore/path\"",
			Destination: &c.ImageDatastorePath,
		},
		cli.StringFlag{
			Name:        "base-image-size",
			Value:       constants.DefaultBaseImageScratchSize,
			Usage:       "Specify the size of the base image from which all other images are created e.g. 8GB/8000MB",
			Destination: &c.ScratchSize,
			Hidden:      true,
		},
	}

	networks := []cli.Flag{
		// bridge
		cli.StringFlag{
			Name:        "bridge-network, b",
			Value:       "",
			Usage:       "The bridge network port group name (private port group for containers). Defaults to the Virtual Container Host name",
			Destination: &c.BridgeNetworkName,
		},
		cli.StringFlag{
			Name:        "bridge-network-range, bnr",
			Value:       "172.16.0.0/12",
			Usage:       "The IP range from which bridge networks are allocated",
			Destination: &c.BridgeIPRange,
			Hidden:      true,
		},

		// client
		cli.StringFlag{
			Name:        "client-network, cln",
			Value:       "",
			Usage:       "The client network port group name (restricts DOCKER_API access to this network). Defaults to DCHP - see advanced help (-x)",
			Destination: &c.ClientNetworkName,
		},
		cli.StringFlag{
			Name:        "client-network-gateway",
			Value:       "",
			Usage:       "Gateway for the VCH on the client network, including one or more routing destinations in a comma separated list, e.g. 10.1.0.0/16,10.2.0.0/16:10.0.0.1",
			Destination: &c.ClientNetworkGateway,
			Hidden:      true,
		},
		cli.StringFlag{
			Name:        "client-network-ip",
			Value:       "",
			Usage:       "IP address with a network mask for the VCH on the client network, e.g. 10.0.0.2/24",
			Destination: &c.ClientNetworkIP,
			Hidden:      true,
		},

		// public
		cli.StringFlag{
			Name:        "public-network, pn",
			Value:       "VM Network",
			Usage:       "The public network port group name (port forwarding and default route). Defaults to 'VM Network' and DHCP -- see advanced help (-x)",
			Destination: &c.PublicNetworkName,
		},
		cli.StringFlag{
			Name:        "public-network-gateway",
			Value:       "",
			Usage:       "Gateway for the VCH on the public network, e.g. 10.0.0.1",
			Destination: &c.PublicNetworkGateway,
			Hidden:      true,
		},
		cli.StringFlag{
			Name:        "public-network-ip",
			Value:       "",
			Usage:       "IP address with a network mask for the VCH on the public network, e.g. 10.0.1.2/24",
			Destination: &c.PublicNetworkIP,
			Hidden:      true,
		},

		// management
		cli.StringFlag{
			Name:        "management-network, mn",
			Value:       "",
			Usage:       "The management network port group name (provides route to target hosting vSphere). Defaults to DCHP - see advanced help (-x)",
			Destination: &c.ManagementNetworkName,
		},
		cli.StringFlag{
			Name:        "management-network-gateway",
			Value:       "",
			Usage:       "Gateway for the VCH on the management network, including one or more routing destinations in a comma separated list, e.g. 10.1.0.0/16,10.2.0.0/16:10.0.0.1",
			Destination: &c.ManagementNetworkGateway,
			Hidden:      true,
		},
		cli.StringFlag{
			Name:        "management-network-ip",
			Value:       "",
			Usage:       "IP address with a network mask for the VCH on the management network, e.g. 10.0.2.2/24",
			Destination: &c.ManagementNetworkIP,
			Hidden:      true,
		},
	}
	var memory, cpu []cli.Flag
	memory = append(memory, c.VCHMemoryLimitFlags()...)
	memory = append(memory,
		cli.IntFlag{
			Name:        "endpoint-memory",
			Value:       constants.DefaultEndpointMemoryMB,
			Usage:       "Memory for the VCH endpoint VM, in MB. Does not impact resources allocated per container.",
			Hidden:      true,
			Destination: &c.MemoryMB,
		})
	cpu = append(cpu, c.VCHCPULimitFlags()...)
	cpu = append(cpu,
		cli.IntFlag{
			Name:        "endpoint-cpu",
			Value:       1,
			Usage:       "vCPUs for the VCH endpoint VM. Does not impact resources allocated per container.",
			Hidden:      true,
			Destination: &c.NumCPUs,
		})

	tls := c.Certs.CertFlags()

	tls = append(tls, cli.BoolFlag{
		Name:        "no-tls, k",
		Usage:       "Disable TLS support completely",
		Destination: &c.Certs.NoTLS,
		Hidden:      true,
	})

	registries := c.Registries.Flags()
	registries = append(registries,
		cli.StringSliceFlag{
			Name:  "insecure-registry, dir",
			Value: &c.Registries.InsecureRegistriesArg,
			Usage: "Specify a list of permitted insecure registry server addresses",
		})
	registries = append(registries,
		cli.StringSliceFlag{
			Name:  "whitelist-registry, wr",
			Value: &c.Registries.WhitelistRegistriesArg,
			Usage: "Specify a list of permitted whitelist registry server addresses (insecure addresses still require the --insecure-registry option in addition)",
		})

	syslog := []cli.Flag{
		cli.StringFlag{
			Name:        "syslog-address",
			Value:       "",
			Usage:       "Address of the syslog server to send Virtual Container Host logs to. Must be in the format transport://host[:port], where transport is udp or tcp. port defaults to 514 if not specified",
			Destination: &c.SyslogAddr,
			Hidden:      true,
		},
	}

	util := []cli.Flag{
		// miscellaneous
		cli.BoolFlag{
			Name:        "force, f",
			Usage:       "Ignore error messages and proceed",
			Destination: &c.Force,
		},
		cli.DurationFlag{
			Name:        "timeout",
			Value:       3 * time.Minute,
			Usage:       "Time to wait for create",
			Destination: &c.Timeout,
		},
		cli.BoolFlag{
			Name:        "asymmetric-routes, ar",
			Usage:       "Set up the Virtual Container Host for asymmetric routing",
			Destination: &c.AsymmetricRouting,
			Hidden:      true,
		},
	}

	target := c.TargetFlags()
	ops := c.OpsCredentials.Flags()
	compute := c.ComputeFlags()
	affinity := c.AffinityFlags()
	container := c.ContainerFlags()
	volume := c.volumeStores.Flags()
	iso := c.ImageFlags(true)
	cNetwork := c.containerNetworks.CNetworkFlags()
	dns := c.Nameservers.DNSFlags()
	proxies := c.Proxies.ProxyFlags()
	kubelet := c.Kubelet.Flags(true)
	debug := c.DebugFlags(true)
	help := c.help.HelpFlags()

	// flag arrays are declared, now combined
	var flags []cli.Flag
	for _, f := range [][]cli.Flag{target, compute, ops, create, affinity, container, volume, dns, networks, cNetwork, memory, cpu, tls, registries, proxies, syslog, iso, util, kubelet, debug, help} {
		flags = append(flags, f...)
	}

	return flags
}

func (c *Create) ProcessParams(op trace.Operation) error {
	defer trace.End(trace.Begin("", op))

	if err := c.HasCredentials(op); err != nil {
		return err
	}

	// prevent usage of special characters for certain user provided values
	if err := common.CheckUnsupportedChars(c.DisplayName); err != nil {
		return cli.NewExitError(fmt.Sprintf("--name contains unsupported characters: %s Allowed characters are alphanumeric, space and symbols - _ ( )", err), 1)
	}

	if len(c.DisplayName) > MaxDisplayNameLen {
		return cli.NewExitError(fmt.Sprintf("Display name %s exceeds the permitted 31 characters limit. Please use a shorter -name parameter", c.DisplayName), 1)
	}

	if c.BridgeNetworkName == "" {
		c.BridgeNetworkName = c.DisplayName
	}

	// Pass admin credentials for use as ops credentials if ops credentials are not supplied.
	if err := c.OpsCredentials.ProcessOpsCredentials(op, true, c.Target.User, c.Target.Password); err != nil {
		return err
	}

	var err error
	c.ContainerNetworks, err = c.containerNetworks.ProcessContainerNetworks(op)
	if err != nil {
		return err
	}

	if err = c.ProcessBridgeNetwork(); err != nil {
		return err
	}

	if err = c.ProcessNetwork(op, &c.Data.ClientNetwork, "client", c.ClientNetworkName,
		c.ClientNetworkIP, c.ClientNetworkGateway); err != nil {
		return err
	}

	if err = c.ProcessNetwork(op, &c.Data.PublicNetwork, "public", c.PublicNetworkName,
		c.PublicNetworkIP, c.PublicNetworkGateway); err != nil {
		return err
	}

	if err = c.ProcessNetwork(op, &c.Data.ManagementNetwork, "management", c.ManagementNetworkName,
		c.ManagementNetworkIP, c.ManagementNetworkGateway); err != nil {
		return err
	}

	if c.DNS, err = c.Nameservers.ProcessDNSServers(op); err != nil {
		return err
	}

	// must come after client network processing as it checks for static IP on that interface
	if err = c.processCertificates(op); err != nil {
		return err
	}

	if err = common.CheckUnsupportedCharsDatastore(c.ImageDatastorePath); err != nil {
		return cli.NewExitError(fmt.Sprintf("--image-store contains unsupported characters: %s Allowed characters are alphanumeric, space and symbols - _ ( ) / :", err), 1)
	}

	c.VolumeLocations, err = c.volumeStores.ProcessVolumeStores()
	if err != nil {
		return err
	}

	if err = c.Registries.ProcessRegistries(op); err != nil {
		return err
	}

	c.InsecureRegistries = c.Registries.InsecureRegistries
	c.WhitelistRegistries = c.Registries.WhitelistRegistries
	c.RegistryCAs = c.Registries.RegistryCAs

	hproxy, sproxy, err := c.Proxies.ProcessProxies()
	if err != nil {
		return err
	}
	c.HTTPProxy = hproxy
	c.HTTPSProxy = sproxy

	if err = c.ProcessSyslog(); err != nil {
		return err
	}

	return nil
}

func (c *Create) processCertificates(op trace.Operation) error {

	// debuglevel is a pointer now so we have to do this song and dance
	var debug int
	if c.Debug.Debug == nil {
		debug = 0
	} else {
		debug = *c.Debug.Debug
	}

	c.Certs.Networks = c.Networks

	if err := c.Certs.ProcessCertificates(op, c.DisplayName, c.Force, debug); err != nil {
		return err
	}

	// copy a few things out of seed because ProcessCertificates has side effects
	c.KeyPEM = c.Certs.KeyPEM
	c.CertPEM = c.Certs.CertPEM
	c.ClientCAs = c.Certs.ClientCAs

	return nil
}

func (c *Create) ProcessBridgeNetwork() error {
	// bridge network params
	var err error

	_, c.Data.BridgeIPRange, err = net.ParseCIDR(c.BridgeIPRange)
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("Error parsing bridge network ip range: %s. Range must be in CIDR format, e.g., 172.16.0.0/12", err), 1)
	}
	return nil
}

func parseGatewaySpec(gw string) (cidrs []net.IPNet, gwIP net.IPNet, err error) {
	ss := strings.Split(gw, ":")
	if len(ss) > 2 {
		err = fmt.Errorf("gateway %s specified incorrectly", gw)
		return
	}

	gwStr := ss[0]
	cidrsStr := ""
	if len(ss) > 1 {
		gwStr = ss[1]
		cidrsStr = ss[0]
	}

	if gwIP.IP = net.ParseIP(gwStr); gwIP.IP == nil {
		err = fmt.Errorf("Provided gateway IP address is not valid: %s", gwStr)
	}

	if err != nil {
		return
	}

	if cidrsStr != "" {
		for _, c := range strings.Split(cidrsStr, ",") {
			var ipnet *net.IPNet
			_, ipnet, err = net.ParseCIDR(c)
			if err != nil {
				err = fmt.Errorf("invalid CIDR in gateway specification: %s", err)
				return
			}
			cidrs = append(cidrs, *ipnet)
		}
	}

	return
}

// ProcessNetwork parses network args if present
func (c *Create) ProcessNetwork(op trace.Operation, network *data.NetworkConfig, netName, pgName, staticIP, gateway string) error {
	var err error

	network.Name = pgName

	if gateway != "" && staticIP == "" {
		return fmt.Errorf("Gateway provided without static IP for %s network", netName)
	}

	defer func(net *data.NetworkConfig) {
		if err == nil {
			op.Debugf("%s network: IP %s gateway %s dest: %s", netName, net.IP, net.Gateway.IP, net.Destinations)
		}
	}(network)

	var ipNet *net.IPNet
	if staticIP != "" {
		var ipAddr net.IP
		ipAddr, ipNet, err = net.ParseCIDR(staticIP)
		if err != nil {
			return fmt.Errorf("Failed to parse the provided %s network IP address %s: %s", netName, staticIP, err)
		}

		network.IP.IP = ipAddr
		network.IP.Mask = ipNet.Mask
	}

	if gateway != "" {
		network.Destinations, network.Gateway, err = parseGatewaySpec(gateway)
		if err != nil {
			return fmt.Errorf("Invalid %s network gateway: %s", netName, err)
		}

		if !network.IP.Contains(network.Gateway.IP) {
			return fmt.Errorf("%s gateway with IP %s is not reachable from %s", netName, network.Gateway.IP, ipNet.String())
		}

		// TODO(vburenin): this seems ugly, and it actually is. The reason is that a gateway required to specify
		// a network mask for it, which is just not how network configuration should be done. Network mask has to
		// be provided separately or with the IP address. It is hard to change all dependencies to keep mask
		// with IP address, so it will be stored with network gateway as it was previously.
		network.Gateway.Mask = network.IP.Mask
	}

	return nil
}

func (c *Create) ProcessSyslog() error {
	if len(c.SyslogAddr) == 0 {
		return nil
	}

	u, err := url.Parse(c.SyslogAddr)
	if err != nil {
		return err
	}

	c.SyslogConfig.Addr = u
	return nil
}

func (c *Create) logArguments(op trace.Operation, cliContext *cli.Context) []string {
	args := []string{}
	sf := c.SetFields() // StringSlice options set by the user
	for _, f := range cliContext.FlagNames() {
		_, ok := sf[f]
		if !cliContext.IsSet(f) && !ok {
			continue
		}

		// avoid logging sensitive data
		if f == "user" || f == "password" || f == "ops-password" {
			op.Debugf("--%s=<censored>", f)
			continue
		}

		if f == "tls-server-cert" || f == "tls-cert-path" || f == "tls-server-key" || f == "registry-ca" || f == "tls-ca" {
			continue
		}

		if f == "target" {
			url, err := url.Parse(cliContext.String(f))
			if err != nil {
				op.Debugf("Unable to re-parse target url for logging")
				continue
			}
			url.User = nil
			flag := fmt.Sprintf("--target=%s", url.String())
			op.Debug(flag)
			args = append(args, flag)
			continue
		}

		i := cliContext.Int(f)
		if i != 0 {
			flag := fmt.Sprintf("--%s=%d", f, i)
			op.Debug(flag)
			args = append(args, flag)
			continue
		}
		d := cliContext.Duration(f)
		if d != 0 {
			flag := fmt.Sprintf("--%s=%s", f, d.String())
			op.Debug(flag)
			args = append(args, flag)
			continue
		}
		x := cliContext.Float64(f)
		if x != 0 {
			flag := fmt.Sprintf("--%s=%f", f, x)
			op.Debug(flag)
			args = append(args, flag)
			continue
		}

		// check for StringSlice before String as the cli String checker
		// will mistake a StringSlice for a String and jackaroo the formatting
		match := func() (result bool) {
			result = false
			defer func() { recover() }()
			ss := cliContext.StringSlice(f)
			if ss != nil {
				for _, o := range ss {
					flag := fmt.Sprintf("--%s=%s", f, o)
					op.Debug(flag)
					args = append(args, flag)
				}
			}
			return ss != nil
		}()
		if match {
			continue
		}

		s := cliContext.String(f)
		if s != "" {
			flag := fmt.Sprintf("--%s=%s", f, s)
			op.Debug(flag)
			args = append(args, flag)
			continue
		}

		b := cliContext.Bool(f)
		bT := cliContext.BoolT(f)
		if b && !bT {
			flag := fmt.Sprintf("--%s=%t", f, true)
			op.Debug(flag)
			args = append(args, flag)
			continue
		}

		match = func() (result bool) {
			result = false
			defer func() { recover() }()
			is := cliContext.IntSlice(f)
			if is != nil {
				flag := fmt.Sprintf("--%s=%#v", f, is)
				op.Debug(flag)
				args = append(args, flag)
			}
			return is != nil
		}()
		if match {
			continue
		}

		// generic last because it matches everything
		g := cliContext.Generic(f)
		if g != nil {
			flag := fmt.Sprintf("--%s=%#v", f, g)
			op.Debug(flag)
			args = append(args, flag)
		}
	}

	return args
}

func (c *Create) Run(clic *cli.Context) (err error) {

	if c.help.Print(clic) {
		return nil
	}

	// create the logger for streaming VCH log messages
	datastoreLog := vchlog.New()
	defer func(old io.Writer) {
		trace.Logger.Out = old
		datastoreLog.Close()
	}(trace.Logger.Out)
	trace.Logger.Out = io.MultiWriter(trace.Logger.Out, datastoreLog.GetPipe())
	go datastoreLog.Run()

	// These operations will be executed without timeout
	op := common.NewOperation(clic, c.Debug.Debug)
	op.Infof("### Installing VCH ####")
	ver := version.GetBuild().ShortVersion()
	op.Debugf("Version %s", ver)

	defer func() {
		// urfave/cli will print out exit in error handling, so no more information in main method can be printed out.
		err = common.LogErrorIfAny(op, clic, err)
	}()

	if err = c.ProcessParams(op); err != nil {
		return err
	}

	args := c.logArguments(op, clic)

	var images map[string]string
	if images, err = c.CheckImagesFiles(op, c.Force); err != nil {
		return err
	}

	if len(clic.Args()) > 0 {
		op.Errorf("Unknown argument: %s", clic.Args()[0])
		return errors.New("invalid CLI arguments")
	}

	validator, err := validate.NewValidator(op, c.Data)
	if err != nil {
		op.Error("Create cannot continue: failed to create validator")
		return err
	}
	defer validator.Session.Logout(op)

	vchConfig, err := validator.Validate(op, c.Data)
	if err != nil {
		op.Error("Create cannot continue: configuration validation failed")
		return err
	}

	// persist cli args used to create the VCH
	vchConfig.VicMachineCreateOptions = args

	vConfig := validator.AddDeprecatedFields(op, vchConfig, c.Data)
	vConfig.ImageFiles = images
	vConfig.ApplianceISO = path.Base(c.ApplianceISO)
	vConfig.BootstrapISO = path.Base(c.BootstrapISO)

	vConfig.HTTPProxy = c.HTTPProxy
	vConfig.HTTPSProxy = c.HTTPSProxy

	vConfig.Timeout = c.Data.Timeout

	// separate initial validation from dispatch of creation task
	op.Info("")

	executor := management.NewDispatcher(op, validator.Session, management.CreateAction, c.Force)
	executor.InitDiagnosticLogsFromConf(vchConfig)
	if err = executor.CreateVCH(vchConfig, vConfig, datastoreLog); err != nil {
		executor.CollectDiagnosticLogs()
		op.Error(err)
		return err
	}

	// Perform the remaining work using a context with a timeout to ensure the user does not wait forever
	op, cancel := trace.WithTimeout(&op, c.Timeout, "Create")
	defer cancel()
	defer func() {
		if op.Err() == context.DeadlineExceeded {
			//context deadline exceeded, replace returned error message
			err = errors.Errorf("Creating VCH exceeded time limit of %s. Please increase the timeout using --timeout to accommodate for a busy vSphere target", c.Timeout)
		}
	}()

	if err = executor.CheckServiceReady(op, vchConfig, c.Certs.ClientCert); err != nil {
		executor.CollectDiagnosticLogs()
		cmd, _ := executor.GetDockerAPICommand(vchConfig, c.Certs.Ckey, c.Certs.Ccert, c.Certs.Cacert, c.Certs.CertPath)
		op.Info("\tAPI may be slow to start - try to connect to API after a few minutes:")
		if cmd != "" {
			op.Infof("\t\tRun command: %s", cmd)
		} else {
			op.Infof("\t\tRun %s inspect to find API connection command and run the command if ip address is ready", clic.App.Name)
		}
		op.Info("\t\tIf command succeeds, VCH is started. If command fails, VCH failed to install - see documentation for troubleshooting.")
		return err
	}

	op.Info("Initialization of appliance successful")

	// We must check for the volume stores that are present after the portlayer presents.

	executor.ShowVCH(vchConfig, c.Certs.Ckey, c.Certs.Ccert, c.Certs.Cacert, c.Certs.EnvFile, c.Certs.CertPath)
	op.Info("Installer completed successfully")

	go func() {
		select {
		case <-time.After(3 * time.Second):
			op.Infof("Waiting for log upload to complete") // tell the user if the wait causes noticeable delay
		case <-op.Done():
			return
		}
	}()

	// wait on the logger to finish streaming
	datastoreLog.Close()
	datastoreLog.Wait(op)

	return nil
}
