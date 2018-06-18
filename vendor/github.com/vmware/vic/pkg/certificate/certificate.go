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

package certificate

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"math/big"
	"net"
	"os"
	"time"

	"github.com/vmware/vic/pkg/errors"
	"github.com/vmware/vic/pkg/trace"
)

// Default certificate file names
const (
	ClientCert = "cert.pem"
	ClientKey  = "key.pem"
	ServerCert = "server-cert.pem"
	ServerKey  = "server-key.pem"
	CACert     = "ca.pem"
	CAKey      = "ca-key.pem"
)

func hashPublicKey(key *rsa.PublicKey) ([]byte, error) {
	b, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		return nil, fmt.Errorf("Unable to hash key: %s", err)
	}

	h := sha1.New()
	h.Write(b)
	return h.Sum(nil), nil
}

func template(org []string) *x509.Certificate {
	now := time.Now().UTC()
	// help address issues with clock drift
	notBefore := now.AddDate(0, 0, -1)
	notAfter := now.AddDate(1, 0, 0) // 1 year

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		err = errors.Errorf("Failed to generate random number: %s", err)
		return nil
	}

	// ensure that org is set to something
	if len(org) == 0 {
		org = []string{"default"}
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: org,
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageKeyAgreement,
		BasicConstraintsValid: true,
	}

	return &template
}

func templateWithKey(template *x509.Certificate, size int) (*x509.Certificate, *rsa.PrivateKey, error) {
	priv, err := rsa.GenerateKey(rand.Reader, size)
	if err != nil {
		return nil, nil, err
	}

	keyID, err := hashPublicKey(&priv.PublicKey)
	if err != nil {
		return nil, nil, err
	}

	template.SubjectKeyId = keyID
	template.PublicKey = priv.Public()

	return template, priv, nil
}

func templateWithCA(template *x509.Certificate) *x509.Certificate {
	template.IsCA = true
	template.KeyUsage |= x509.KeyUsageCertSign
	template.KeyUsage |= x509.KeyUsageKeyEncipherment
	template.KeyUsage |= x509.KeyUsageKeyAgreement
	template.ExtKeyUsage = nil

	return template
}

// templateAsClientOnly restricts the capabilities of the certificate to be only used for client auth
func templateAsClientOnly(template *x509.Certificate) *x509.Certificate {
	template.KeyUsage = x509.KeyUsageDigitalSignature
	template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}

	return template
}

// templateWithServer adds the capabilities of the certificate to be only used for server auth
func templateWithServer(template *x509.Certificate, domain string) *x509.Certificate {
	template.ExtKeyUsage = append(template.ExtKeyUsage, x509.ExtKeyUsageServerAuth)

	template.Subject.CommonName = domain

	// abide by the spec - if CN is an IP, put it in the subjectAltName as well
	ip := net.ParseIP(domain)
	if ip == nil {
		// see if CIDR works
		// #nosec: Errors unhandled.
		ip, _, _ = net.ParseCIDR(domain)
	}

	if ip != nil {
		// use the normalized address
		template.Subject.CommonName = ip.String()
		template.IPAddresses = []net.IP{ip}

		// try best guess at DNSNames entries
		names, err := net.LookupAddr(domain)
		if err == nil && len(names) > 0 {
			template.DNSNames = names
		}

		return template
	}

	if domain != "" {
		template.DNSNames = []string{domain}

		// try best guess at IPAddresses entries
		ips, err := net.LookupIP(domain)
		if err == nil && len(ips) > 0 {
			template.IPAddresses = ips
		}
	}

	return template
}

// createCertificate creates a certificate from the supplied template:
// template: an x509 template describing the certificate to generate.
// parent: either a CA certificate, or template (for self-signed). If nil, will use template.
// templateKey: the private key for the certificate supplied as template
// parentKey: the private key for the certificate supplied as parent (whether CA or self-signed). If nil will use templateKey
//
// return PEM encoded certificate and key
func createCertificate(template, parent *x509.Certificate, templateKey, parentKey *rsa.PrivateKey) (cert bytes.Buffer, key bytes.Buffer, err error) {
	defer trace.End(trace.Begin(""))

	if parent == nil {
		parent = template
	}

	if parentKey == nil {
		parentKey = templateKey
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, template, parent, &templateKey.PublicKey, parentKey)
	if err != nil {
		err = errors.Errorf("Failed to generate x509 certificate: %s", err)
		return cert, key, err
	}

	err = pem.Encode(&cert, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	if err != nil {
		err = errors.Errorf("Failed to encode x509 certificate: %s", err)
		return cert, key, err
	}

	err = pem.Encode(&key, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(templateKey)})
	if err != nil {
		err = errors.Errorf("Failed to encode tls key pairs: %s", err)
		return cert, key, err
	}

	return cert, key, nil
}

// saveCertificate saves the certificate and key to files of the form basename-cert.pem and basename-key.pem
// cf and kf are the certificate file and key file respectively
func saveCertificate(cf, kf string, cert, key *bytes.Buffer) error {
	defer trace.End(trace.Begin(""))

	// #nosec: Expect file permissions to be 0600 or less
	certFile, err := os.OpenFile(cf, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		err = errors.Errorf("Failed to create certificate file %s: %s", cf, err)
		return err
	}
	defer certFile.Close()

	_, err = certFile.Write(cert.Bytes())
	if err != nil {
		err = errors.Errorf("Failed to write certificate: %s", err)
		return err
	}

	keyFile, err := os.OpenFile(kf, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		err = errors.Errorf("Failed to create key file %s: %s", kf, err)
		return err
	}
	defer keyFile.Close()

	_, err = keyFile.Write(key.Bytes())
	if err != nil {
		err = errors.Errorf("Failed to write key: %s", err)
		return err
	}
	return nil
}

func loadCertificate(cf, kf string) (*x509.Certificate, *rsa.PrivateKey, error) {
	defer trace.End(trace.Begin(""))

	cb, err := ioutil.ReadFile(cf)
	if err != nil {
		err = errors.Errorf("Failed to read certificate file %s: %s", cf, err)
		return nil, nil, err
	}

	kb, err := ioutil.ReadFile(kf)
	if err != nil {
		err = errors.Errorf("Failed to read key file %s: %s", kf, err)
		return nil, nil, err
	}

	return ParseCertificate(cb, kb)
}

func ParseCertificate(cb, kb []byte) (*x509.Certificate, *rsa.PrivateKey, error) {
	defer trace.End(trace.Begin(""))

	block, _ := pem.Decode(cb)
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		err = errors.Errorf("Failed to parse certificate data: %s", err)
		return nil, nil, err
	}

	var key *rsa.PrivateKey
	block, _ = pem.Decode(kb)
	if block != nil {
		key, err = x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			err = errors.Errorf("Failed to parse key data: %s", err)
			return nil, nil, err
		}
	}

	return cert, key, nil
}

func CreateSelfSigned(domain string, org []string, size int) (cert bytes.Buffer, key bytes.Buffer, err error) {
	defer trace.End(trace.Begin(""))

	template, pkey, err := templateWithKey(templateWithServer(template(org), domain), size)
	if err != nil {
		return cert, key, err
	}

	return createCertificate(template, nil, pkey, nil)
}

func CreateRootCA(domain string, org []string, size int) (cert bytes.Buffer, key bytes.Buffer, err error) {
	defer trace.End(trace.Begin(""))

	template, pkey, err := templateWithKey(templateWithCA(template(org)), size)
	if err != nil {
		return cert, key, err
	}

	return createCertificate(template, nil, pkey, nil)
}

func CreateServerCertificate(domain string, org []string, size int, cb, kb []byte) (cert bytes.Buffer, key bytes.Buffer, err error) {
	defer trace.End(trace.Begin(""))

	// Load up the CA
	cacert, cakey, err := ParseCertificate(cb, kb)
	if err != nil {
		return cert, key, err
	}

	// Generate the new cert
	template, pkey, err := templateWithKey(templateWithServer(template(org), domain), size)
	if err != nil {
		return cert, key, err
	}

	return createCertificate(template, cacert, pkey, cakey)
}

func CreateClientCertificate(domain string, org []string, size int, cb, kb []byte) (cert bytes.Buffer, key bytes.Buffer, err error) {
	defer trace.End(trace.Begin(""))

	// Load up the CA
	cacert, cakey, err := ParseCertificate(cb, kb)

	// Generate the new cert
	template, pkey, err := templateWithKey(templateAsClientOnly(template(org)), size)
	if err != nil {
		return cert, key, err
	}

	return createCertificate(template, cacert, pkey, cakey)
}

// VerifyClientCert verifies the loaded client cert keypair against the input CA and
// returns the certificate on success.
func VerifyClientCert(ca []byte, ckp *KeyPair) (*tls.Certificate, error) {
	var err error

	cert, err := ckp.Certificate()
	if err != nil || cert.Leaf == nil {
		return nil, CertParseError{msg: err.Error()}
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(ca) {
		return nil, CreateCAPoolError{}
	}

	opts := x509.VerifyOptions{
		Roots:     pool,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	if _, err = cert.Leaf.Verify(opts); err != nil {
		return nil, CertVerifyError{}
	}

	return cert, nil
}
