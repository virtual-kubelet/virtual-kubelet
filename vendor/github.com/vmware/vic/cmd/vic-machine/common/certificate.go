// Copyright 2017 VMware, Inc. All Rights Reserved.
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

package common

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/urfave/cli.v1"

	"github.com/vmware/vic/pkg/certificate"
	"github.com/vmware/vic/pkg/errors"
	"github.com/vmware/vic/pkg/trace"
)

// CertFactory has all input parameters for vic-machine certificate commands needed to create a certificate
type CertFactory struct {
	Networks

	CertPath     string
	DisplayName  string
	Scert        string
	Skey         string
	Ccert        string
	Ckey         string
	Cacert       string
	Cakey        string
	ClientCert   *tls.Certificate
	ClientCAsArg cli.StringSlice `arg:"tls-ca"`
	ClientCAs    []byte
	EnvFile      string
	Cname        string
	Org          cli.StringSlice
	KeySize      int
	NoTLS        bool
	NoTLSverify  bool
	KeyPEM       []byte
	CertPEM      []byte
	NoSaveToDisk bool
}

func (c *CertFactory) CertFlags() []cli.Flag {
	return []cli.Flag{
		cli.StringFlag{
			Name:        "tls-server-key",
			Value:       "",
			Usage:       "Virtual Container Host private key file (server certificate)",
			Destination: &c.Skey,
		},
		cli.StringFlag{
			Name:        "tls-server-cert",
			Value:       "",
			Usage:       "Virtual Container Host x509 certificate file (server certificate)",
			Destination: &c.Scert,
		},
		cli.StringFlag{
			Name:        "tls-cname",
			Value:       "",
			Usage:       "Common Name to use in generated CA certificate when requiring client certificate authentication",
			Destination: &c.Cname,
		},
		cli.StringFlag{
			Name:        "tls-cert-path",
			Value:       "",
			Usage:       "The path to check for existing certificates and in which to save generated certificates. Defaults to './<vch name>/'",
			Destination: &c.CertPath,
		},
		cli.BoolFlag{
			Name:        "no-tlsverify, kv",
			Usage:       "Disable authentication via client certificates - for more tls options see advanced help (-x)",
			Destination: &c.NoTLSverify,
		},
		cli.StringSliceFlag{
			Name:   "organization",
			Usage:  "A list of identifiers to record in the generated certificates. Defaults to VCH name and IP/FQDN if not provided.",
			Value:  &c.Org,
			Hidden: true,
		},
		cli.IntFlag{
			Name:        "certificate-key-size, ksz",
			Usage:       "Size of key to use when generating certificates",
			Value:       2048,
			Destination: &c.KeySize,
			Hidden:      true,
		},
		cli.StringSliceFlag{
			Name:   "tls-ca, ca",
			Usage:  "Specify a list of certificate authority files to use for client verification",
			Value:  &c.ClientCAsArg,
			Hidden: true,
		},
	}
}

func (c *CertFactory) ProcessCertificates(op trace.Operation, displayName string, force bool, debug int) error {
	// set up the locations for the certificates and env file
	if c.CertPath == "" {
		c.CertPath = displayName
	}
	c.EnvFile = fmt.Sprintf("%s/%s.env", c.CertPath, displayName)

	// check for insecure case
	if c.NoTLS {
		op.Warn("Configuring without TLS - all communications will be insecure")
		return nil
	}

	if c.Scert != "" && c.Skey == "" {
		return cli.NewExitError("key and cert should be specified at the same time", 1)
	}
	if c.Scert == "" && c.Skey != "" {
		return cli.NewExitError("key and cert should be specified at the same time", 1)
	}

	// if we've not got a specific CommonName but do have a static IP then go with that.
	if c.Cname == "" {
		if c.ClientNetworkIP != "" {
			c.Cname = c.ClientNetworkIP
			op.Infof("Using client-network-ip as cname where needed - use --tls-cname to override: %s", c.Cname)
		} else if c.PublicNetworkIP != "" && (c.PublicNetworkName == c.ClientNetworkName || c.ClientNetworkName == "") {
			c.Cname = c.PublicNetworkIP
			op.Infof("Using public-network-ip as cname where needed - use --tls-cname to override: %s", c.Cname)
		} else if c.ManagementNetworkIP != "" && (c.ManagementNetworkName == c.ClientNetworkName || (c.ClientNetworkName == "" && c.ManagementNetworkName == c.PublicNetworkName)) {
			c.Cname = c.ManagementNetworkIP
			op.Infof("Using management-network-ip as cname where needed - use --tls-cname to override: %s", c.Cname)
		}

		if c.Cname != "" {
			// Strip network mask from IP address if set.
			// #nosec: Errors unhandled.
			if cnameIP, _, _ := net.ParseCIDR(c.Cname); cnameIP != nil {
				c.Cname = cnameIP.String()
			}
		}
	}

	// load what certificates we can
	cas, keypair, err := c.loadCertificates(op, debug)
	if err != nil {
		op.Errorf("Unable to load certificates: %s", err)
		if !force {
			return err
		}

		op.Warnf("Ignoring error loading certificates due to --force")
		cas = nil
		keypair = nil
		err = nil
	}

	// we need to generate some part of the certificate configuration
	gcas, gkeypair, err := c.generateCertificates(op, keypair == nil, !c.NoTLSverify && len(cas) == 0)
	if err != nil {
		op.Error("cannot continue: unable to generate certificates")
		return err
	}

	if keypair != nil {
		c.KeyPEM = keypair.KeyPEM
		c.CertPEM = keypair.CertPEM
	} else if gkeypair != nil {
		c.KeyPEM = gkeypair.KeyPEM
		c.CertPEM = gkeypair.CertPEM
	}

	if len(cas) == 0 {
		cas = gcas
	}

	if len(c.KeyPEM) == 0 {
		return errors.New("Failed to load or generate server certificates")
	}

	if len(cas) == 0 && !c.NoTLSverify {
		return errors.New("Failed to load or generate certificate authority")
	}

	// do we have key, cert, and --no-tlsverify
	if c.NoTLSverify || len(cas) == 0 {
		op.Warnf("Configuring without TLS verify - certificate-based authentication disabled")
		return nil
	}

	c.ClientCAs = cas
	return nil
}

// loadCertificates returns the client CA pool and the keypair for server certificates on success
func (c *CertFactory) loadCertificates(op trace.Operation, debug int) ([]byte, *certificate.KeyPair, error) {
	defer trace.End(trace.Begin("", op))

	// reads each of the files specified, assuming that they are PEM encoded certs,
	// and constructs a byte array suitable for passing to CertPool.AppendCertsFromPEM
	var certs []byte
	for _, f := range c.ClientCAsArg {
		b, err := ioutil.ReadFile(f)
		if err != nil {
			err = errors.Errorf("Failed to load authority from file %s: %s", f, err)
			return nil, nil, err
		}

		certs = append(certs, b...)
		op.Infof("Loaded CA from %s", f)
	}

	var keypair *certificate.KeyPair
	// default names
	skey := filepath.Join(c.CertPath, certificate.ServerKey)
	scert := filepath.Join(c.CertPath, certificate.ServerCert)
	ca := filepath.Join(c.CertPath, certificate.CACert)
	ckey := filepath.Join(c.CertPath, certificate.ClientKey)
	ccert := filepath.Join(c.CertPath, certificate.ClientCert)

	// if specific files are supplied, use those
	explicit := false
	if c.Scert != "" && c.Skey != "" {
		explicit = true
		skey = c.Skey
		scert = c.Scert
	}

	// load the server certificate
	keypair = certificate.NewKeyPair(scert, skey, nil, nil)
	if err := keypair.LoadCertificate(); err != nil {
		if explicit || !os.IsNotExist(err) {
			// if these files were explicit paths, or anything other than not found, fail
			op.Errorf("Failed to load certificate: %s", err)
			return certs, nil, err
		}

		op.Debugf("Unable to locate existing server certificate in cert path")
		return nil, nil, nil
	}

	// check that any supplied cname matches the server cert CN
	cert, err := keypair.Certificate()
	if err != nil {
		op.Errorf("Failed to parse certificate: %s", err)
		return certs, nil, err
	}

	if cert.Leaf == nil {
		op.Warnf("Failed to load x509 leaf: Unable to confirm server certificate cname matches provided cname %q. Continuing...", c.Cname)
	} else {
		// We just do a direct equality check here - trying to be clever is liable to lead to hard
		// to diagnose errors
		if cert.Leaf.Subject.CommonName != c.Cname {
			op.Errorf("Provided cname does not match that in existing server certificate: %s", cert.Leaf.Subject.CommonName)
			if debug > 2 {
				op.Debugf("Certificate does not match provided cname: %#+v", cert.Leaf)
			}
			return certs, nil, fmt.Errorf("cname option doesn't match existing server certificate in certificate path %s", c.CertPath)
		}
	}

	op.Infof("Loaded server certificate %s", scert)
	c.Skey = skey
	c.Scert = scert

	// only try for CA certificate if no-tlsverify has NOT been specified and we haven't already loaded an authority cert
	if !c.NoTLSverify && len(certs) == 0 {
		b, err := ioutil.ReadFile(ca)
		if err != nil {
			if os.IsNotExist(err) {
				op.Debugf("Unable to locate existing CA in cert path")
				return certs, keypair, nil
			}

			// if the CA exists but cannot be loaded then it's an error
			op.Errorf("Failed to load authority from certificate path %s: %s", c.CertPath, err)
			return certs, keypair, errors.New("failed to load certificate authority")
		}

		c.Cacert = ca

		op.Infof("Loaded CA with default name from certificate path %s", c.CertPath)
		certs = b

		// load client certs - we ensure the client certs validate with the provided CA or ignore any we find
		cpair := certificate.NewKeyPair(ccert, ckey, nil, nil)
		if err := cpair.LoadCertificate(); err != nil {
			op.Warnf("Unable to load client certificate - validation of API endpoint will be best effort only: %s", err)
		}

		clientCert, err := certificate.VerifyClientCert(certs, cpair)
		if err != nil {
			switch err.(type) {
			case certificate.CertParseError, certificate.CreateCAPoolError:
				op.Debug(err)
			case certificate.CertVerifyError:
				op.Warnf("%s - continuing without client certificate", err)
			}

			return certs, keypair, nil
		}

		c.Ckey = ckey
		c.Ccert = ccert
		c.ClientCert = clientCert

		op.Infof("Loaded client certificate with default name from certificate path %s", c.CertPath)
	}

	return certs, keypair, nil
}

func (c *CertFactory) generateCertificates(op trace.Operation, server bool, client bool) ([]byte, *certificate.KeyPair, error) {
	defer trace.End(trace.Begin("", op))

	if !server && !client {
		op.Debug("Not generating server or client certs, nothing for generateCertificates to do")
		return nil, nil, nil
	}

	var certs []byte
	// generate the certs and keys with names conforming the default the docker client expects
	files, err := ioutil.ReadDir(c.CertPath)
	if len(files) > 0 {
		return nil, nil, fmt.Errorf("Specified directory to store certificates is not empty. Specify a new path in which to store generated certificates using --tls-cert-path or remove the contents of \"%s\" and run vic-machine again.", c.CertPath)
	}

	if !c.NoSaveToDisk {
		err = os.MkdirAll(c.CertPath, 0700)
		if err != nil {
			op.Errorf("Unable to make directory \"%s\" to hold certificates (set via --tls-cert-path)", c.CertPath)
			return nil, nil, err
		}
	}

	c.Skey = filepath.Join(c.CertPath, certificate.ServerKey)
	c.Scert = filepath.Join(c.CertPath, certificate.ServerCert)

	c.Ckey = filepath.Join(c.CertPath, certificate.ClientKey)
	c.Ccert = filepath.Join(c.CertPath, certificate.ClientCert)

	cakey := filepath.Join(c.CertPath, certificate.CAKey)
	c.Cacert = filepath.Join(c.CertPath, certificate.CACert)

	if server && !client {
		op.Infof("Generating self-signed certificate/key pair - private key in %s", c.Skey)
		keypair := certificate.NewKeyPair(c.Scert, c.Skey, nil, nil)
		err := keypair.CreateSelfSigned(c.Cname, nil, c.KeySize)
		if err != nil {
			op.Errorf("Failed to generate self-signed certificate: %s", err)
			return nil, nil, err
		}
		if !c.NoSaveToDisk {
			if err = keypair.SaveCertificate(); err != nil {
				op.Errorf("Failed to save server certificates: %s", err)
				return nil, nil, err
			}
		}

		return certs, keypair, nil
	}

	// client auth path
	if c.Cname == "" {
		op.Error("Common Name must be provided when generating certificates for client authentication:")
		op.Info("  --tls-cname=<FQDN or static IP> # for the appliance VM")
		op.Info("  --tls-cname=<*.yourdomain.com>  # if DNS has entries in that form for DHCP addresses (less secure)")
		op.Info("  --no-tlsverify                  # disables client authentication (anyone can connect to the VCH)")
		op.Info("  --no-tls                        # disables TLS entirely")
		op.Info("")

		return certs, nil, errors.New("provide Common Name for server certificate")
	}

	// for now re-use the display name as the organisation if unspecified
	if len(c.Org) == 0 {
		c.Org = []string{c.DisplayName}
	}
	if len(c.Org) == 1 && !strings.HasPrefix(c.Cname, "*") {
		// Add in the cname if it's not a wildcard
		c.Org = append(c.Org, c.Cname)
	}

	// Certificate authority
	op.Infof("Generating CA certificate/key pair - private key in %s", cakey)
	cakp := certificate.NewKeyPair(c.Cacert, cakey, nil, nil)
	err = cakp.CreateRootCA(c.Cname, c.Org, c.KeySize)
	if err != nil {
		op.Errorf("Failed to generate CA: %s", err)
		return nil, nil, err
	}
	if !c.NoSaveToDisk {
		if err = cakp.SaveCertificate(); err != nil {
			op.Errorf("Failed to save CA certificates: %s", err)
			return nil, nil, err
		}
	}

	// Server certificates
	var skp *certificate.KeyPair
	if server {
		op.Infof("Generating server certificate/key pair - private key in %s", c.Skey)
		skp = certificate.NewKeyPair(c.Scert, c.Skey, nil, nil)
		err = skp.CreateServerCertificate(c.Cname, c.Org, c.KeySize, cakp)
		if err != nil {
			op.Errorf("Failed to generate server certificates: %s", err)
			return nil, nil, err
		}
		if !c.NoSaveToDisk {
			if err = skp.SaveCertificate(); err != nil {
				op.Errorf("Failed to save server certificates: %s", err)
				return nil, nil, err
			}
		}
	}

	// Client certificates
	if client {
		op.Infof("Generating client certificate/key pair - private key in %s", c.Ckey)
		ckp := certificate.NewKeyPair(c.Ccert, c.Ckey, nil, nil)
		err = ckp.CreateClientCertificate(c.Cname, c.Org, c.KeySize, cakp)
		if err != nil {
			op.Errorf("Failed to generate client certificates: %s", err)
			return nil, nil, err
		}
		if !c.NoSaveToDisk {
			if err = ckp.SaveCertificate(); err != nil {
				op.Errorf("Failed to save client certificates: %s", err)
				return nil, nil, err
			}
		}

		c.ClientCert, err = ckp.Certificate()
		if err != nil {
			op.Warnf("Failed to stash client certificate for later application level validation: %s", err)
		}

		// If openssl is present, try to generate a browser friendly pfx file (a bundle of the public certificate AND the private key)
		// The pfx file can be imported directly into keychains for client certificate authentication
		certPath := filepath.Clean(c.CertPath)
		args := strings.Split(fmt.Sprintf("pkcs12 -export -out %[1]s/cert.pfx -inkey %[1]s/key.pem -in %[1]s/cert.pem -certfile %[1]s/ca.pem -password pass:", certPath), " ")
		// #nosec: Subprocess launching with variable
		pfx := exec.Command("openssl", args...)
		out, err := pfx.CombinedOutput()
		if err != nil {
			op.Debug(out)
			op.Warnf("Failed to generate browser friendly PFX client certificate: %s", err)
		} else {
			op.Infof("Generated browser friendly PFX client certificate - certificate in %s/cert.pfx", certPath)
		}
	}

	return cakp.CertPEM, skp, nil
}
