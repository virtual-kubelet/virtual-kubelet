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

package handlers

import (
	"fmt"
	"net/http"

	"github.com/go-openapi/runtime"
	"github.com/go-openapi/runtime/middleware"

	"github.com/vmware/vic/lib/apiservers/service/models"
	"github.com/vmware/vic/lib/apiservers/service/restapi/handlers/util"
	"github.com/vmware/vic/lib/apiservers/service/restapi/operations"
	"github.com/vmware/vic/lib/config"
	"github.com/vmware/vic/lib/install/data"
	"github.com/vmware/vic/lib/install/validate"
	"github.com/vmware/vic/pkg/trace"
)

type VCHCertGet struct{}

type VCHDatacenterCertGet struct{}

func (h *VCHCertGet) Handle(params operations.GetTargetTargetVchVchIDCertificateParams, principal interface{}) middleware.Responder {
	op := trace.FromContext(params.HTTPRequest.Context(), "VCHCertGet: %s", params.VchID)

	b := buildDataParams{
		target:     params.Target,
		thumbprint: params.Thumbprint,
		vchID:      &params.VchID,
	}

	d, validator, err := buildDataAndValidateTarget(op, b, principal)
	if err != nil {
		return operations.NewGetTargetTargetVchVchIDCertificateDefault(
			util.StatusCode(err)).WithPayload(&models.Error{Message: err.Error()})
	}

	c, err := getVCHCert(op, d, validator)
	if err != nil {
		return operations.NewGetTargetTargetVchVchIDCertificateDefault(
			util.StatusCode(err)).WithPayload(&models.Error{Message: err.Error()})
	}

	cert := asPemCertificate(c.Cert)
	return NewGetTargetTargetVchVchIDCertificateOK(cert.Pem)
}

func (h *VCHDatacenterCertGet) Handle(params operations.GetTargetTargetDatacenterDatacenterVchVchIDCertificateParams, principal interface{}) middleware.Responder {
	op := trace.FromContext(params.HTTPRequest.Context(), "VCHDatacenterCertGet: %s", params.VchID)

	b := buildDataParams{
		target:     params.Target,
		thumbprint: params.Thumbprint,
		datacenter: &params.Datacenter,
		vchID:      &params.VchID,
	}

	d, validator, err := buildDataAndValidateTarget(op, b, principal)
	if err != nil {
		return operations.NewGetTargetTargetDatacenterDatacenterVchVchIDCertificateDefault(
			util.StatusCode(err)).WithPayload(&models.Error{Message: err.Error()})
	}

	c, err := getVCHCert(op, d, validator)
	if err != nil {
		return operations.NewGetTargetTargetDatacenterDatacenterVchVchIDCertificateDefault(
			util.StatusCode(err)).WithPayload(&models.Error{Message: err.Error()})
	}

	cert := asPemCertificate(c.Cert)
	return NewGetTargetTargetDatacenterDatacenterVchVchIDCertificateOK(cert.Pem)
}

func getVCHCert(op trace.Operation, d *data.Data, validator *validate.Validator) (*config.RawCertificate, error) {
	vchConfig, err := getVCHConfig(op, d, validator)
	if err != nil {
		return nil, err
	}

	if vchConfig.HostCertificate.IsNil() {
		return nil, util.NewError(http.StatusNotFound, fmt.Sprintf("No certificate found for VCH %s", d.ID))
	}

	return vchConfig.HostCertificate, nil
}

// GetTargetTargetVchVchIDCertificateOK and the methods below are actually borrowed directly from generated swagger code.
// They are moved into this file and altered to use the TextProducer when returning a PEM certificate, as swagger does not
// directly support application/x-pem-file on the server side.
type GetTargetTargetVchVchIDCertificateOK struct {
	*operations.GetTargetTargetVchVchIDCertificateOK
}

func NewGetTargetTargetVchVchIDCertificateOK(payload models.PEM) *GetTargetTargetVchVchIDCertificateOK {
	return &GetTargetTargetVchVchIDCertificateOK{operations.NewGetTargetTargetVchVchIDCertificateOK().WithPayload(payload)}
}

func (o *GetTargetTargetVchVchIDCertificateOK) WriteResponse(rw http.ResponseWriter, producer runtime.Producer) {
	o.GetTargetTargetVchVchIDCertificateOK.WriteResponse(rw, runtime.TextProducer())
}

type GetTargetTargetDatacenterDatacenterVchVchIDCertificateOK struct {
	*operations.GetTargetTargetDatacenterDatacenterVchVchIDCertificateOK
}

func NewGetTargetTargetDatacenterDatacenterVchVchIDCertificateOK(payload models.PEM) *GetTargetTargetDatacenterDatacenterVchVchIDCertificateOK {
	return &GetTargetTargetDatacenterDatacenterVchVchIDCertificateOK{operations.NewGetTargetTargetDatacenterDatacenterVchVchIDCertificateOK().WithPayload(payload)}
}

func (o *GetTargetTargetDatacenterDatacenterVchVchIDCertificateOK) WriteResponse(rw http.ResponseWriter, producer runtime.Producer) {
	o.GetTargetTargetDatacenterDatacenterVchVchIDCertificateOK.WriteResponse(rw, runtime.TextProducer())
}
