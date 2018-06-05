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

package management

import (
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/docker/docker/opts"

	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/lib/config"
	"github.com/vmware/vic/lib/constants"
	"github.com/vmware/vic/pkg/certificate"
	"github.com/vmware/vic/pkg/errors"
	"github.com/vmware/vic/pkg/ip"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/vm"
)

func (d *Dispatcher) InspectVCH(vch *vm.VirtualMachine, conf *config.VirtualContainerHostConfigSpec, certPath string) error {
	defer trace.End(trace.Begin(conf.Name, d.op))

	state, err := vch.PowerState(d.op)
	if err != nil {
		d.op.Error("Failed to get VM power state, service might not be available at this moment.")
	}
	if state != types.VirtualMachinePowerStatePoweredOn {
		err = errors.Errorf("VCH is not powered on, state %s", state)
		d.op.Errorf("%s", err)
		return err
	}

	var clientIP net.IP
	var publicIP net.IP

	clientNet := conf.ExecutorConfig.Networks["client"]
	if clientNet != nil {
		clientIP = clientNet.Assigned.IP
	}
	publicNet := conf.ExecutorConfig.Networks["public"]
	if publicNet != nil {
		publicIP = publicNet.Assigned.IP
	}

	if ip.IsUnspecifiedIP(clientIP) {
		err = errors.Errorf("No client IP address assigned")
		d.op.Errorf("%s", err)
		return err
	}

	if ip.IsUnspecifiedIP(publicIP) {
		err = errors.Errorf("No public IP address assigned")
		d.op.Errorf("%s", err)
		return err
	}

	d.HostIP = clientIP.String()
	d.op.Debugf("IP address for client interface: %s", d.HostIP)
	if !conf.HostCertificate.IsNil() {
		d.DockerPort = fmt.Sprintf("%d", opts.DefaultTLSHTTPPort)
	} else {
		d.DockerPort = fmt.Sprintf("%d", opts.DefaultHTTPPort)
	}

	// try looking up preferred name, irrespective of CAs
	if cert, err := conf.HostCertificate.X509Certificate(); err == nil {
		d.HostIP = d.GetTLSFriendlyHostIP(clientIP, cert, conf.CertificateAuthorities)
	} else {
		d.op.Debug("No host certificates provided, using assigned client IP as host address.")
	}

	// Check for valid client cert for a tls-verify configuration
	if len(conf.CertificateAuthorities) > 0 {
		possibleCertPaths := findCertPaths(d.op, conf.Name, certPath)

		// Check if a valid client cert exists in one of possibleCertPaths
		certPath = ""
		for _, path := range possibleCertPaths {
			certFile := filepath.Join(path, certificate.ClientCert)
			keyFile := filepath.Join(path, certificate.ClientKey)
			ckp := certificate.NewKeyPair(certFile, keyFile, nil, nil)
			if err = ckp.LoadCertificate(); err != nil {
				d.op.Debugf("Unable to load client cert in %s: %s", path, err)
				continue
			}

			if _, err = certificate.VerifyClientCert(conf.CertificateAuthorities, ckp); err != nil {
				d.op.Debug(err)
				continue
			}

			certPath = path
			break
		}
	}

	d.ShowVCH(conf, "", "", "", "", certPath)
	return nil
}

func (d *Dispatcher) GetTLSFriendlyHostIP(clientIP net.IP, cert *x509.Certificate, CertificateAuthorities []byte) string {
	hostIP := clientIP.String()
	name, _ := viableHostAddress(d.op, []net.IP{clientIP}, cert, CertificateAuthorities)

	if name != "" {
		d.op.Debugf("Retrieved proposed name from host certificate: %q", name)
		d.op.Debugf("Using the first proposed name returned from the set of all preferred name from host certificate: %s", name)

		if name != hostIP {
			d.op.Infof("Using address %s from host certificate for host address over assigned client IP %s", name, d.HostIP)
			// found a preferred name by CA, return the preferred hostIP
			return name
		}
	}

	d.op.Warn("Unable to identify address acceptable to host certificate, using assigned client IP as host address.")

	return hostIP
}

// findCertPaths returns candidate paths for client certs depending on whether
// a certPath was specified in the CLI.
func findCertPaths(op trace.Operation, vchName, certPath string) []string {
	var possibleCertPaths []string

	if certPath != "" {
		op.Infof("--tls-cert-path supplied - only checking for certs in %s/", certPath)
		possibleCertPaths = append(possibleCertPaths, certPath)
		return possibleCertPaths
	}

	possibleCertPaths = append(possibleCertPaths, vchName, ".")
	logMsg := fmt.Sprintf("--tls-cert-path not supplied - checking for certs in current directory, %s/", vchName)

	dockerConfPath := ""
	user, err := user.Current()
	if err == nil {
		dockerConfPath = filepath.Join(user.HomeDir, ".docker")
		possibleCertPaths = append(possibleCertPaths, dockerConfPath)
		logMsg = fmt.Sprintf("%s and %s/", logMsg, dockerConfPath)
	}
	op.Info(logMsg)

	return possibleCertPaths
}

func (d *Dispatcher) ShowVCH(conf *config.VirtualContainerHostConfigSpec, key string, cert string, cacert string, envfile string, certpath string) {
	moref := new(types.ManagedObjectReference)
	if ok := moref.FromString(conf.ID); ok {
		d.op.Info("")
		d.op.Infof("VCH ID: %s", moref.Value)
	}

	if d.sshEnabled {
		d.op.Info("")
		d.op.Info("SSH to appliance:")
		d.op.Infof("ssh root@%s", d.HostIP)
	}

	d.op.Info("")
	d.op.Info("VCH Admin Portal:")
	d.op.Infof("https://%s:%d", d.HostIP, constants.VchAdminPortalPort)

	d.op.Info("")
	publicIP := conf.ExecutorConfig.Networks["public"].Assigned.IP
	d.op.Info("Published ports can be reached at:")
	d.op.Infof("%s", publicIP.String())

	if management := conf.ExecutorConfig.Networks["management"]; management != nil {
		d.op.Info("")
		managementIP := management.Assigned.IP
		d.op.Info("Management traffic will use:")
		d.op.Infof("%s", managementIP.String())
	}

	cmd, env := d.GetDockerAPICommand(conf, key, cert, cacert, certpath)

	d.op.Info("")
	d.op.Info("Docker environment variables:")
	d.op.Info(env)

	if envfile != "" {
		if err := ioutil.WriteFile(envfile, []byte(env), 0644); err == nil {
			d.op.Info("")
			d.op.Infof("Environment saved in %s", envfile)
		}
	}

	d.op.Info("")
	d.op.Info("Connect to docker:")
	d.op.Info(cmd)
}

// GetDockerAPICommand generates values to display for usage of a deployed VCH
func (d *Dispatcher) GetDockerAPICommand(conf *config.VirtualContainerHostConfigSpec, key string, cert string, cacert string, certpath string) (cmd, env string) {
	var dEnv []string
	tls := ""

	if d.HostIP == "" {
		return "", ""
	}

	if !conf.HostCertificate.IsNil() {
		// if we're generating then there's no CA currently
		if len(conf.CertificateAuthorities) > 0 {
			// find the name to use
			if key != "" {
				tls = fmt.Sprintf(" --tlsverify --tlscacert=%q --tlscert=%q --tlskey=%q", cacert, cert, key)
			} else {
				tls = fmt.Sprintf(" --tlsverify")
			}

			dEnv = append(dEnv, "DOCKER_TLS_VERIFY=1")
			info, err := os.Stat(certpath)
			if err == nil && info.IsDir() {
				if abs, err := filepath.Abs(info.Name()); err == nil {
					dEnv = append(dEnv, fmt.Sprintf("DOCKER_CERT_PATH=%s", abs))
				}
			} else {
				d.op.Info("")
				d.op.Warn("Unable to find valid client certs")
				d.op.Warn("DOCKER_CERT_PATH must be provided in environment or certificates specified individually via CLI arguments")
			}
		} else {
			tls = " --tls"
		}
	}
	dEnv = append(dEnv, fmt.Sprintf("DOCKER_HOST=%s:%s", d.HostIP, d.DockerPort))

	cmd = fmt.Sprintf("docker -H %s:%s%s info", d.HostIP, d.DockerPort, tls)
	env = strings.Join(dEnv, " ")

	return cmd, env
}
