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

package handlers

import (
	"github.com/go-openapi/runtime/middleware"

	"github.com/vmware/vic/lib/apiservers/portlayer/models"
	"github.com/vmware/vic/lib/apiservers/portlayer/restapi/operations"
	"github.com/vmware/vic/lib/apiservers/portlayer/restapi/operations/logging"
	"github.com/vmware/vic/lib/portlayer/exec"
	portlayer "github.com/vmware/vic/lib/portlayer/logging"
	"github.com/vmware/vic/pkg/trace"
)

// LoggingHandlersImpl is the receiver for all of the logging handler methods
type LoggingHandlersImpl struct {
}

// Configure initializes the handler
func (i *LoggingHandlersImpl) Configure(api *operations.PortLayerAPI, _ *HandlerContext) {
	api.LoggingLoggingJoinHandler = logging.LoggingJoinHandlerFunc(i.JoinHandler)
	api.LoggingLoggingBindHandler = logging.LoggingBindHandlerFunc(i.BindHandler)
	api.LoggingLoggingUnbindHandler = logging.LoggingUnbindHandlerFunc(i.UnbindHandler)
}

// JoinHandler calls the Join
func (i *LoggingHandlersImpl) JoinHandler(params logging.LoggingJoinParams) middleware.Responder {
	defer trace.End(trace.Begin(""))

	handle := exec.HandleFromInterface(params.Config.Handle)
	if handle == nil {
		err := &models.Error{Message: "Failed to get the Handle"}
		return logging.NewLoggingJoinInternalServerError().WithPayload(err)
	}

	handleprime, err := portlayer.Join(handle)
	if err != nil {
		return logging.NewLoggingJoinInternalServerError().WithPayload(
			&models.Error{Message: err.Error()},
		)
	}

	res := &models.LoggingJoinResponse{
		Handle: exec.ReferenceFromHandle(handleprime),
	}
	return logging.NewLoggingJoinOK().WithPayload(res)
}

// BindHandler calls the Bind
func (i *LoggingHandlersImpl) BindHandler(params logging.LoggingBindParams) middleware.Responder {
	defer trace.End(trace.Begin(""))

	handle := exec.HandleFromInterface(params.Config.Handle)
	if handle == nil {
		err := &models.Error{Message: "Failed to get the Handle"}
		return logging.NewLoggingBindInternalServerError().WithPayload(err)
	}

	handleprime, err := portlayer.Bind(handle)
	if err != nil {
		return logging.NewLoggingBindInternalServerError().WithPayload(
			&models.Error{Message: err.Error()},
		)
	}

	res := &models.LoggingBindResponse{
		Handle: exec.ReferenceFromHandle(handleprime),
	}
	return logging.NewLoggingBindOK().WithPayload(res)
}

// UnbindHandler calls the Unbind
func (i *LoggingHandlersImpl) UnbindHandler(params logging.LoggingUnbindParams) middleware.Responder {
	defer trace.End(trace.Begin(""))

	handle := exec.HandleFromInterface(params.Config.Handle)
	if handle == nil {
		err := &models.Error{Message: "Failed to get the Handle"}
		return logging.NewLoggingUnbindInternalServerError().WithPayload(err)
	}

	handleprime, err := portlayer.Unbind(handle)
	if err != nil {
		return logging.NewLoggingUnbindInternalServerError().WithPayload(
			&models.Error{Message: err.Error()},
		)
	}

	res := &models.LoggingUnbindResponse{
		Handle: exec.ReferenceFromHandle(handleprime),
	}
	return logging.NewLoggingUnbindOK().WithPayload(res)
}
