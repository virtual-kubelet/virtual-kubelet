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
	"context"
	"net/http"

	log "github.com/Sirupsen/logrus"
	"github.com/go-openapi/runtime/middleware"

	"github.com/vmware/vic/lib/apiservers/portlayer/models"
	"github.com/vmware/vic/lib/apiservers/portlayer/restapi/operations"
	"github.com/vmware/vic/lib/apiservers/portlayer/restapi/operations/kv"
	"github.com/vmware/vic/lib/portlayer/store"
	"github.com/vmware/vic/pkg/kvstore"
	"github.com/vmware/vic/pkg/trace"
)

type KvHandlersImpl struct {
	defaultStore kvstore.KeyValueStore
}

func (handler *KvHandlersImpl) Configure(api *operations.PortLayerAPI, handlerCtx *HandlerContext) {
	api.KvGetValueHandler = kv.GetValueHandlerFunc(handler.GetValueHandler)
	api.KvPutValueHandler = kv.PutValueHandlerFunc(handler.PutValueHandler)
	api.KvDeleteValueHandler = kv.DeleteValueHandlerFunc(handler.DeleteValueHandler)

	// Get the APIKV store -- it should always be present since it's
	// initialized when the portlayer starts
	// #nosec: Errors unhandled.
	s, _ := store.Store(store.APIKV)
	handler.defaultStore = s
}

func (handler *KvHandlersImpl) GetValueHandler(params kv.GetValueParams) middleware.Responder {
	defer trace.End(trace.Begin(params.Key))

	val, err := handler.defaultStore.Get(params.Key)
	if err != nil {
		switch err {
		case kvstore.ErrKeyNotFound:
			return kv.NewGetValueNotFound()
		default:
			log.Errorf("Error Getting Key/Value: %s", err.Error())
			return kv.NewGetValueInternalServerError().WithPayload(&models.Error{
				Code:    http.StatusInternalServerError,
				Message: err.Error(),
			})
		}
	}
	return kv.NewGetValueOK().WithPayload(&models.KeyValue{Key: params.Key, Value: string(val)})
}

func (handler *KvHandlersImpl) PutValueHandler(params kv.PutValueParams) middleware.Responder {
	defer trace.End(trace.Begin(params.KeyValue.Key))

	err := handler.defaultStore.Put(
		context.Background(),
		params.KeyValue.Key,
		[]byte(params.KeyValue.Value))
	if err != nil {
		log.Errorf("Error Setting Key/Value: %s", err.Error())
		return kv.NewGetValueInternalServerError().WithPayload(&models.Error{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
	}
	return kv.NewPutValueOK()
}

func (handler *KvHandlersImpl) DeleteValueHandler(params kv.DeleteValueParams) middleware.Responder {
	defer trace.End(trace.Begin(params.Key))

	err := handler.defaultStore.Delete(trace.NewOperation(context.Background(), "DeleteValue"), params.Key)
	if err != nil {
		switch err {
		case kvstore.ErrKeyNotFound:
			return kv.NewDeleteValueNotFound()
		default:
			log.Errorf("Error deleting Key/Value: %s", err.Error())
			return kv.NewGetValueInternalServerError().WithPayload(&models.Error{
				Code:    http.StatusInternalServerError,
				Message: err.Error(),
			})
		}
	}
	return kv.NewDeleteValueOK()
}
