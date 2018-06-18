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

package kv

import (
	"errors"
	"fmt"

	"github.com/vmware/vic/lib/apiservers/portlayer/client"
	ckv "github.com/vmware/vic/lib/apiservers/portlayer/client/kv"
	"github.com/vmware/vic/lib/apiservers/portlayer/models"
	"github.com/vmware/vic/pkg/trace"

	"context"

	log "github.com/Sirupsen/logrus"
)

const (

	// defaultNamespace is the first part of the
	// k/v store key (i.e. docker.stuff)
	defaultNamespace = "docker"
	defaultSeparator = "."
)

var (
	ErrKeyNotFound = errors.New("key not found")
)

// Get will call to the portlayer for the value of the specified key
// The key argument is prefixed w/the defaultName space for the docker
// persona. i.e. docker.{key}
//
// If the key doesn't exist an ErrKeyNotFound will be returned
func Get(client *client.PortLayer, key string) (string, error) {
	defer trace.End(trace.Begin(key))
	var val string
	resp, err := client.Kv.GetValue(ckv.NewGetValueParamsWithContext(
		context.Background()).WithKey(createNameSpacedKey(key)))
	if err != nil {
		switch err.(type) {
		case *ckv.GetValueNotFound:
			return val, ErrKeyNotFound
		default:
			log.Errorf("Error Getting Key/Value: %s", err.Error())
			return val, err
		}
	}
	val = resp.Payload.Value
	// return the value
	return val, nil

}

// Put will put the key / value in the portlayer k/v store
func Put(client *client.PortLayer, key string, val string) error {
	defer trace.End(trace.Begin(key))

	fullKey := createNameSpacedKey(key)
	keyval := &models.KeyValue{
		Key:   fullKey,
		Value: val,
	}

	_, err := client.Kv.PutValue(ckv.NewPutValueParamsWithContext(
		context.Background()).WithKey(fullKey).WithKeyValue(keyval))
	if err != nil {
		log.Errorf("Error Putting Key/Value: %s", err)
		return err
	}

	return nil
}

// Delete will remove the key / value from the store
func Delete(client *client.PortLayer, key string) error {
	defer trace.End(trace.Begin(key))

	_, err := client.Kv.DeleteValue(ckv.NewDeleteValueParamsWithContext(
		context.Background()).WithKey(createNameSpacedKey(key)))
	if err != nil {
		log.Errorf("Error Deleting Key/Value: %s", err)
		return err
	}

	return nil
}

func createNameSpacedKey(key string) string {
	return fmt.Sprintf("%s%s%s", defaultNamespace, defaultSeparator, key)
}
