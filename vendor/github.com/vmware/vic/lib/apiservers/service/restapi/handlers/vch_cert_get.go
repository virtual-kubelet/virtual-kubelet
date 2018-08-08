// Copyright 2017-2018 VMware, Inc. All Rights Reserved.
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
	"net/http"

	"github.com/go-openapi/runtime"
	"github.com/go-openapi/runtime/middleware"

	"github.com/vmware/vic/lib/apiservers/service/models"
	"github.com/vmware/vic/lib/apiservers/service/restapi/handlers/encode"
	"github.com/vmware/vic/lib/apiservers/service/restapi/handlers/errors"
	"github.com/vmware/vic/lib/apiservers/service/restapi/handlers/target"
	"github.com/vmware/vic/lib/apiservers/service/restapi/operations"
	"github.com/vmware/vic/lib/install/management"
	"github.com/vmware/vic/pkg/trace"
)

// VCHCertGet is the Handler for getting the certificate for a VCH without specifying a datacenter
type VCHCertGet struct {
	vchCertGet
}

// VCHDatacenterCertGet is the Handler for getting the certificate for a VCH within a specified datacenter
type VCHDatacenterCertGet struct {
	vchCertGet
}

// vchCertGet allows for VCHCertGet and VCHDatacenterCertGet to share common code without polluting the package
type vchCertGet struct{}

// Handle is the handler implementation for getting the certificate for a VCH without specifying a datacenter
func (h *VCHCertGet) Handle(params operations.GetTargetTargetVchVchIDCertificateParams, principal interface{}) middleware.Responder {
	op := trace.FromContext(params.HTTPRequest.Context(), "VCHCertGet: %s", params.VchID)

	b := target.Params{
		Target:     params.Target,
		Thumbprint: params.Thumbprint,
		VCHID:      &params.VchID,
	}

	cert, err := h.handle(op, b, principal)
	if err != nil {
		return operations.NewGetTargetTargetVchVchIDCertificateDefault(errors.StatusCode(err)).WithPayload(&models.Error{Message: err.Error()})
	}

	return NewGetTargetTargetVchVchIDCertificateOK(cert.Pem)
}

// Handle is the handler implementation for getting the certificate for a VCH within a specified datacenter
func (h *VCHDatacenterCertGet) Handle(params operations.GetTargetTargetDatacenterDatacenterVchVchIDCertificateParams, principal interface{}) middleware.Responder {
	op := trace.FromContext(params.HTTPRequest.Context(), "VCHDatacenterCertGet: %s", params.VchID)

	b := target.Params{
		Target:     params.Target,
		Thumbprint: params.Thumbprint,
		Datacenter: &params.Datacenter,
		VCHID:      &params.VchID,
	}

	cert, err := h.handle(op, b, principal)
	if err != nil {
		return operations.NewGetTargetTargetDatacenterDatacenterVchVchIDCertificateDefault(errors.StatusCode(err)).WithPayload(&models.Error{Message: err.Error()})
	}

	return NewGetTargetTargetDatacenterDatacenterVchVchIDCertificateOK(cert.Pem)
}

// handle retrieves the server certificate for the VCH described by params, using the credentials from principal. If no
// certificate is configured on the VCH or the VCH itself cannot be found, a 404 is returned. If another error occurs,
// a 500 is returned.
func (h *vchCertGet) handle(op trace.Operation, params target.Params, principal interface{}) (*models.X509Data, error) {
	d, c, err := target.Validate(op, management.ActionInspectCertificates, params, principal)
	if err != nil {
		return nil, err
	}

	vchConfig, err := c.GetVCHConfig(op, d)
	if err != nil {
		return nil, err
	}

	if vchConfig.HostCertificate.IsNil() {
		return nil, errors.NewError(http.StatusNotFound, "no certificate found for VCH %s", d.ID)
	}

	return encode.AsPemCertificate(vchConfig.HostCertificate.Cert), nil
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
