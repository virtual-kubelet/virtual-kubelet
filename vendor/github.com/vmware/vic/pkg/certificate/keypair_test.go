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
	"os"
	"strings"
	"testing"

	"crypto/tls"

	log "github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/vmware/vic/pkg/trace"
)

const (
	keyFile  = "./key.pem"
	certFile = "./cert.pem"
)

func TestMain(m *testing.M) {
	log.SetLevel(log.DebugLevel)
	trace.Logger.Level = log.DebugLevel

	code := m.Run()
	os.Exit(code)
}

func TestCreateSelfSigned(t *testing.T) {
	cert, key, err := CreateSelfSigned("somewhere.com", []string{"MyOrg"}, 2048)
	if err != nil {
		t.Errorf("CreateSelfSigned failed with error %s", err)
	}

	certString := cert.String()
	keyString := key.String()

	log.Infof("cert: %s", certString)
	log.Infof("key: %s", keyString)

	if !strings.HasPrefix(certString, "-----BEGIN CERTIFICATE-----") {
		t.Errorf("Certificate lacks proper prefix; must not have been generated properly.")
	}

	if !strings.HasSuffix(certString, "-----END CERTIFICATE-----\n") {
		t.Errorf("Certificate lacks proper suffix; must not have been generated properly.")
	}

	if !strings.HasPrefix(keyString, "-----BEGIN RSA PRIVATE KEY-----") {
		t.Errorf("Private key lacks proper prefix; must not have been generated properly.")
	}

	if !strings.HasSuffix(keyString, "-----END RSA PRIVATE KEY-----\n") {
		t.Errorf("Private key lacks proper suffix; must not have been generated properly.")
	}

	_, err = tls.X509KeyPair([]byte(certString), []byte(keyString))
	if err != nil {
		t.Errorf("Unable to load X509 key pair(%s,%s): %s", certString, keyString, err)
	}

}

func TestGenerate(t *testing.T) {
	log.SetLevel(log.InfoLevel)
	if _, err := os.Stat(keyFile); err == nil {
		os.Remove(keyFile)
	}

	pair := NewKeyPair(keyFile, certFile, nil, nil)

	err := pair.CreateSelfSigned("somewhere.com", []string{"MyOrg"}, 2048)
	assert.NoError(t, err, "Failed generating self-signed certificate")

	err = pair.SaveCertificate()
	assert.NoError(t, err, "Failed saving generated certificate")
	defer os.Remove(keyFile)
	defer os.Remove(certFile)

	assert.NotEmpty(t, pair.KeyPEM, "Expected contents in key PEM data")
	assert.NotEmpty(t, pair.CertPEM, "Expected contents in cert PEM data")

	_, err = os.Stat(keyFile)
	assert.NoError(t, err, "Key file was not created")

	assert.Contains(t, string(pair.KeyPEM), "RSA PRIVATE KEY", "Key is not correctly generated")
}

func TestGetCertificate(t *testing.T) {
	log.SetLevel(log.InfoLevel)
	if _, err := os.Stat(keyFile); err == nil {
		os.Remove(keyFile)
	}

	pair := NewKeyPair(keyFile, certFile, nil, nil)

	err := pair.CreateSelfSigned("somewhere.com", []string{"MyOrg"}, 2048)
	assert.NoError(t, err, "Failed generating self-signed certificate")

	err = pair.SaveCertificate()
	assert.NoError(t, err, "Failed saving generated certificate")
	defer os.Remove(keyFile)
	defer os.Remove(certFile)

	pair2 := NewKeyPair(keyFile, certFile, nil, nil)

	err = pair2.LoadCertificate()
	assert.NoError(t, err, "Failed loading self-signed certificate")

	assert.Equal(t, pair, pair2, "Expected loads to be consistent")
}
