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

package extraconfig

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"

	log "github.com/Sirupsen/logrus"

	"golang.org/x/crypto/nacl/secretbox"
)

// The value of this key is hidden from API requests, but visible within the guest
// #nosec: Potential hardcoded credentials
const GuestInfoSecretKey = GuestInfoPrefix + "ovfEnv"

// SecretKey provides helpers to encrypt/decrypt extraconfig values
type SecretKey struct {
	key [32]byte
}

// NewSecretKey generates a new secret key
func NewSecretKey() (*SecretKey, error) {
	s := new(SecretKey)

	if _, err := rand.Read(s.key[:]); err != nil {
		return nil, err
	}

	return s, nil
}

// FromString base64 decodes an existing SecretKey
func (s *SecretKey) FromString(key string) error {
	b, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return err
	}

	if len(b) != 32 {
		return errors.New("invalid secret key")
	}

	copy(s.key[:], b)

	return nil
}

// String base64 encodes a SecretKey
func (s *SecretKey) String() string {
	return base64.StdEncoding.EncodeToString(s.key[:])
}

// Source wraps the given DataSource, decrypting any secret values
func (s *SecretKey) Source(ds DataSource) DataSource {
	// If GuestInfoSecretKey has a value, it should be our secret key.
	// #nosec: Errors unhandled.
	if val, _ := ds(GuestInfoSecretKey); val != "" {
		if err := s.FromString(val); err != nil {
			log.Errorf("failed to decode %s: %s", GuestInfoSecretKey, err)
		} else {
			log.Debugf("secret key decoded from %s", GuestInfoSecretKey)
		}
	}

	return func(key string) (string, error) {
		val, err := ds(key)

		if err == nil && isSecret(key) {
			b, err := base64.StdEncoding.DecodeString(val)
			if err != nil {
				return "", err
			}

			var nonce [24]byte
			copy(nonce[:], b[:24])

			plaintext, ok := secretbox.Open([]byte{}, b[24:], &nonce, &s.key)
			if !ok {
				return "", fmt.Errorf("failed to decrypt value for %s", key)
			}

			val = string(plaintext)
		}

		return val, err
	}
}

// Sink wraps the given DataSink, encrypting any secret values
func (s *SecretKey) Sink(ds DataSink) DataSink {
	// Store our secret key.
	if err := ds(GuestInfoSecretKey, s.String()); err != nil {
		log.Errorf("failed to store %s: %s", GuestInfoSecretKey, err)
	}

	return func(key, value string) error {
		if isSecret(key) {
			var nonce [24]byte

			if _, err := rand.Read(nonce[:]); err != nil {
				return err
			}

			ciphertext := secretbox.Seal(nonce[:], []byte(value), &nonce, &s.key)

			value = base64.StdEncoding.EncodeToString(ciphertext)
		}

		return ds(key, value)
	}
}
