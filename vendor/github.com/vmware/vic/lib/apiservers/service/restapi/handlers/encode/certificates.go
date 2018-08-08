// Copyright 2018 VMware, Inc. All Rights Reserved.
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

package encode

import (
	"bytes"
	"encoding/pem"

	"github.com/vmware/vic/lib/apiservers/service/models"
)

func AsPemCertificates(certificates []byte) []*models.X509Data {
	var buf bytes.Buffer

	m := make([]*models.X509Data, 0)
	for c := &certificates; len(*c) > 0; {
		b, rest := pem.Decode(*c)

		err := pem.Encode(&buf, b)
		if err != nil {
			continue // TODO (#6716): Handle? (We probably don't want to let this fail the request, but may want to convey that something unexpected happened.)
		}

		m = append(m, &models.X509Data{
			Pem: models.PEM(buf.String()),
		})

		c = &rest
	}

	return m
}

func AsPemCertificate(certificates []byte) *models.X509Data {
	m := AsPemCertificates(certificates)

	if len(m) > 1 {
		// TODO (#6716): Error? (We probably don't want to let this fail the request, but may want to convey that something unexpected happened.)
	}

	return m[0]
}
