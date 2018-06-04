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
	"crypto/x509"
	"os"
	"testing"

	log "github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestCreateCA(t *testing.T) {
	log.SetLevel(log.DebugLevel)

	cacert, cakey, err := CreateRootCA("somewhere.com", []string{"MyOrg"}, 2048)
	assert.NoError(t, err, "Failed generating CA certificate")

	_, _, err = ParseCertificate(cacert.Bytes(), cakey.Bytes())
	assert.NoError(t, err, "Failed reparsing CA certificate")

}

func TestSignedCertificate(t *testing.T) {
	log.SetLevel(log.DebugLevel)

	cacert, cakey, err := CreateRootCA("somewhere.com", []string{"MyOrg"}, 2048)
	assert.NoError(t, err, "Failed generating ca certificate")

	cert, key, err := CreateServerCertificate("somewere.com", []string{"MyOrg"}, 2048, cacert.Bytes(), cakey.Bytes())
	assert.NoError(t, err, "Failed generating signed certificate")

	// validate
	roots := x509.NewCertPool()
	ok := roots.AppendCertsFromPEM(cacert.Bytes())
	assert.Equal(t, true, ok, "Failed to append CA to roots")

	opts := x509.VerifyOptions{
		Roots: roots,
	}

	tlsCert, _, err := ParseCertificate(cert.Bytes(), key.Bytes())
	assert.NoError(t, err, "Failed loading signed certificate")

	_, err = tlsCert.Verify(opts)
	assert.NoError(t, err, "Failed loading signed certificate")
}

func TestFailedValidation(t *testing.T) {
	log.SetLevel(log.DebugLevel)

	cacert, cakey, err := CreateRootCA("somewhere.com", []string{"MyOrg"}, 2048)
	assert.NoError(t, err, "Failed generating ca certificate")

	cert, key, err := CreateServerCertificate("somewere.com", []string{"MyOrg"}, 2048, cacert.Bytes(), cakey.Bytes())
	assert.NoError(t, err, "Failed generating signed certificate")

	// validate
	roots := x509.NewCertPool()
	ok := roots.AppendCertsFromPEM(cacert.Bytes())
	assert.Equal(t, true, ok, "Failed to append CA to roots")

	tlsCert, _, err := ParseCertificate(cert.Bytes(), key.Bytes())
	assert.NoError(t, err, "Failed loading signed certificate")

	opts := x509.VerifyOptions{
		Roots:   roots,
		DNSName: "somewhereELSE.com",
	}

	_, err = tlsCert.Verify(opts)
	assert.Error(t, err, "Expected to fail initial verify")

	opts = x509.VerifyOptions{
		Roots:   roots,
		DNSName: "somewhere.com",
	}

	_, err = tlsCert.Verify(opts)
	assert.Error(t, err, "Expected to pass second verify")

}

func TestVerifyClientCert(t *testing.T) {
	cacert, cakey, err := CreateRootCA("foo.com", []string{"FooOrg"}, 2048)
	assert.NoError(t, err)

	cert, key, err := CreateClientCertificate("foo.com", []string{"FooOrg"}, 2048, cacert.Bytes(), cakey.Bytes())
	assert.NoError(t, err)

	kp := NewKeyPair(ClientCert, ClientKey, cert.Bytes(), key.Bytes())
	err = kp.SaveCertificate()
	assert.NoError(t, err)
	defer func() {
		os.Remove(ClientCert)
		os.Remove(ClientKey)
	}()

	// Validate client certificate keypair created with the right CA
	_, err = VerifyClientCert(cacert.Bytes(), kp)
	assert.NoError(t, err)

	cacert, cakey, err = CreateRootCA("bar.com", []string{"BarOrg"}, 2048)
	assert.NoError(t, err)

	// Attempt to validate client certificate keypair created with a different CA
	_, err = VerifyClientCert(cacert.Bytes(), kp)
	assert.NotNil(t, err)
}
