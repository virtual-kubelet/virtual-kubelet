// Copyright © 2017 The virtual-kubelet authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package manager_test

import (
	"testing"

	"github.com/virtual-kubelet/virtual-kubelet/internal/manager"
	testutil "github.com/virtual-kubelet/virtual-kubelet/internal/test/util"
	"gotest.tools/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

// TestGetPods verifies that the resource manager acts as a passthrough to a pod lister.
func TestGetPods(t *testing.T) {
	var (
		lsPods = []*v1.Pod{
			testutil.FakePodWithSingleContainer("namespace-0", "name-0", "image-0"),
			testutil.FakePodWithSingleContainer("namespace-1", "name-1", "image-1"),
		}
	)

	// Create a pod lister that will list the pods defined above.
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	for _, pod := range lsPods {
		assert.NilError(t, indexer.Add(pod))
	}
	podLister := corev1listers.NewPodLister(indexer)

	// Create a new instance of the resource manager based on the pod lister.
	rm, err := manager.NewResourceManager(podLister, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Check that the resource manager returns two pods in the call to "GetPods".
	rmPods := rm.GetPods()
	if len(rmPods) != len(lsPods) {
		t.Fatalf("expected %d pods, found %d", len(lsPods), len(rmPods))
	}
}

// TestGetSecret verifies that the resource manager acts as a passthrough to a secret lister.
func TestGetSecret(t *testing.T) {
	var (
		lsSecrets = []*v1.Secret{
			testutil.FakeSecret("namespace-0", "name-0", map[string]string{"key-0": "val-0"}),
			testutil.FakeSecret("namespace-1", "name-1", map[string]string{"key-1": "val-1"}),
		}
	)

	// Create a secret lister that will list the secrets defined above.
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	for _, secret := range lsSecrets {
		assert.NilError(t, indexer.Add(secret))
	}
	secretLister := corev1listers.NewSecretLister(indexer)

	// Create a new instance of the resource manager based on the secret lister.
	rm, err := manager.NewResourceManager(nil, secretLister, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Get the secret with coordinates "namespace-0/name-0".
	secret, err := rm.GetSecret("name-0", "namespace-0")
	if err != nil {
		t.Fatal(err)
	}
	value := secret.Data["key-0"]
	if string(value) != "val-0" {
		t.Fatal("got unexpected value", string(value))
	}

	// Try to get a secret that does not exist, and make sure we've got a "not found" error as a response.
	_, err = rm.GetSecret("name-X", "namespace-X")
	if err == nil || !errors.IsNotFound(err) {
		t.Fatalf("expected a 'not found' error, got %v", err)
	}
}

// TestGetConfigMap verifies that the resource manager acts as a passthrough to a config map lister.
func TestGetConfigMap(t *testing.T) {
	var (
		lsConfigMaps = []*v1.ConfigMap{
			testutil.FakeConfigMap("namespace-0", "name-0", map[string]string{"key-0": "val-0"}),
			testutil.FakeConfigMap("namespace-1", "name-1", map[string]string{"key-1": "val-1"}),
		}
	)

	// Create a config map lister that will list the config maps defined above.
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	for _, secret := range lsConfigMaps {
		assert.NilError(t, indexer.Add(secret))
	}
	configMapLister := corev1listers.NewConfigMapLister(indexer)

	// Create a new instance of the resource manager based on the config map lister.
	rm, err := manager.NewResourceManager(nil, nil, configMapLister, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Get the config map with coordinates "namespace-0/name-0".
	configMap, err := rm.GetConfigMap("name-0", "namespace-0")
	if err != nil {
		t.Fatal(err)
	}
	value := configMap.Data["key-0"]
	if value != "val-0" {
		t.Fatal("got unexpected value", value)
	}

	// Try to get a configmap that does not exist, and make sure we've got a "not found" error as a response.
	_, err = rm.GetConfigMap("name-X", "namespace-X")
	if err == nil || !errors.IsNotFound(err) {
		t.Fatalf("expected a 'not found' error, got %v", err)
	}
}

// TestListServices verifies that the resource manager acts as a passthrough to a service lister.
func TestListServices(t *testing.T) {
	var (
		lsServices = []*v1.Service{
			testutil.FakeService("namespace-0", "service-0", "1.2.3.1", "TCP", 8081),
			testutil.FakeService("namespace-1", "service-1", "1.2.3.2", "TCP", 8082),
		}
	)

	// Create a pod lister that will list the pods defined above.
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	for _, service := range lsServices {
		assert.NilError(t, indexer.Add(service))
	}
	serviceLister := corev1listers.NewServiceLister(indexer)

	// Create a new instance of the resource manager based on the pod lister.
	rm, err := manager.NewResourceManager(nil, nil, nil, serviceLister)
	if err != nil {
		t.Fatal(err)
	}

	// Check that the resource manager returns two pods in the call to "GetPods".
	services, err := rm.ListServices()
	if err != nil {
		t.Fatal(err)
	}
	if len(lsServices) != len(services) {
		t.Fatalf("expected %d services, found %d", len(lsServices), len(services))
	}
}
