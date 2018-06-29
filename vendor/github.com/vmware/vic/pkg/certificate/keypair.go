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

package certificate

import (
	"bytes"
	"crypto/tls"
	"io/ioutil"

	"github.com/vmware/vic/pkg/errors"
)

type KeyPair struct {
	KeyPEM  []byte
	CertPEM []byte

	KeyFile  string
	CertFile string
}

func NewKeyPair(certFile, keyFile string, certPEM, keyPEM []byte) *KeyPair {
	return &KeyPair{
		KeyPEM:   keyPEM,
		CertPEM:  certPEM,
		KeyFile:  keyFile,
		CertFile: certFile,
	}
}

func (kp *KeyPair) LoadCertificate() error {
	c, err := ioutil.ReadFile(kp.CertFile)
	if err != nil {
		return err
	}

	k, err := ioutil.ReadFile(kp.KeyFile)
	if err != nil {
		return err
	}

	kp.CertPEM = c
	kp.KeyPEM = k

	return nil
}

func (kp *KeyPair) SaveCertificate() error {
	return saveCertificate(kp.CertFile, kp.KeyFile, bytes.NewBuffer(kp.CertPEM), bytes.NewBuffer(kp.KeyPEM))
}

func (kp *KeyPair) CreateSelfSigned(domain string, org []string, size int) error {
	c, k, err := CreateSelfSigned(domain, org, size)
	if err != nil {
		return err
	}

	kp.CertPEM = c.Bytes()
	kp.KeyPEM = k.Bytes()

	return nil
}

func (kp *KeyPair) CreateRootCA(domain string, org []string, size int) error {
	c, k, err := CreateRootCA(domain, org, size)
	if err != nil {
		return err
	}

	kp.CertPEM = c.Bytes()
	kp.KeyPEM = k.Bytes()

	return nil
}

func (kp *KeyPair) CreateServerCertificate(domain string, org []string, size int, ca *KeyPair) error {
	c, k, err := CreateServerCertificate(domain, org, size, ca.CertPEM, ca.KeyPEM)
	if err != nil {
		return err
	}

	kp.CertPEM = c.Bytes()
	kp.KeyPEM = k.Bytes()

	return nil
}

func (kp *KeyPair) CreateClientCertificate(domain string, org []string, size int, ca *KeyPair) error {
	c, k, err := CreateClientCertificate(domain, org, size, ca.CertPEM, ca.KeyPEM)
	if err != nil {
		return err
	}

	kp.CertPEM = c.Bytes()
	kp.KeyPEM = k.Bytes()

	return nil
}

// Certificate turns the KeyPair back into useful TLS constructs
// This attempts to populate the certificate.Leaf field with the x509 certificate for convenience
func (kp *KeyPair) Certificate() (*tls.Certificate, error) {
	if kp.CertPEM == nil || kp.KeyPEM == nil {
		return nil, errors.New("KeyPair has no data")
	}

	cert, err := tls.X509KeyPair(kp.CertPEM, kp.KeyPEM)
	if err != nil {
		return nil, err
	}

	// #nosec: Errors unhandled.
	cert.Leaf, _, _ = ParseCertificate(kp.CertPEM, kp.KeyPEM)

	return &cert, nil
}
