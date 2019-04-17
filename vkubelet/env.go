package vkubelet

import (
	"context"
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	apivalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/tools/record"
	podshelper "k8s.io/kubernetes/pkg/apis/core/pods"
	v1helper "k8s.io/kubernetes/pkg/apis/core/v1/helper"
	fieldpath "k8s.io/kubernetes/pkg/fieldpath"
	"k8s.io/kubernetes/pkg/kubelet/envvars"

	"github.com/virtual-kubelet/virtual-kubelet/log"
	"github.com/virtual-kubelet/virtual-kubelet/manager"
)

const (
	// ReasonOptionalConfigMapNotFound is the reason used in events emitted when an optional configmap is not found.
	ReasonOptionalConfigMapNotFound = "OptionalConfigMapNotFound"
	// ReasonOptionalConfigMapKeyNotFound is the reason used in events emitted when an optional configmap key is not found.
	ReasonOptionalConfigMapKeyNotFound = "OptionalConfigMapKeyNotFound"
	// ReasonFailedToReadOptionalConfigMap is the reason used in events emitted when an optional configmap could not be read.
	ReasonFailedToReadOptionalConfigMap = "FailedToReadOptionalConfigMap"

	// ReasonOptionalSecretNotFound is the reason used in events emitted when an optional secret is not found.
	ReasonOptionalSecretNotFound = "OptionalSecretNotFound"
	// ReasonOptionalSecretKeyNotFound is the reason used in events emitted when an optional secret key is not found.
	ReasonOptionalSecretKeyNotFound = "OptionalSecretKeyNotFound"
	// ReasonFailedToReadOptionalSecret is the reason used in events emitted when an optional secret could not be read.
	ReasonFailedToReadOptionalSecret = "FailedToReadOptionalSecret"

	// ReasonMandatoryConfigMapNotFound is the reason used in events emitted when an mandatory configmap is not found.
	ReasonMandatoryConfigMapNotFound = "MandatoryConfigMapNotFound"
	// ReasonMandatoryConfigMapKeyNotFound is the reason used in events emitted when an mandatory configmap key is not found.
	ReasonMandatoryConfigMapKeyNotFound = "MandatoryConfigMapKeyNotFound"
	// ReasonFailedToReadMandatoryConfigMap is the reason used in events emitted when an mandatory configmap could not be read.
	ReasonFailedToReadMandatoryConfigMap = "FailedToReadMandatoryConfigMap"

	// ReasonMandatorySecretNotFound is the reason used in events emitted when an mandatory secret is not found.
	ReasonMandatorySecretNotFound = "MandatorySecretNotFound"
	// ReasonMandatorySecretKeyNotFound is the reason used in events emitted when an mandatory secret key is not found.
	ReasonMandatorySecretKeyNotFound = "MandatorySecretKeyNotFound"
	// ReasonFailedToReadMandatorySecret is the reason used in events emitted when an mandatory secret could not be read.
	ReasonFailedToReadMandatorySecret = "FailedToReadMandatorySecret"

	// ReasonInvalidEnvironmentVariableNames is the reason used in events emitted when a configmap/secret referenced in a ".spec.containers[*].envFrom" field contains invalid environment variable names.
	ReasonInvalidEnvironmentVariableNames = "InvalidEnvironmentVariableNames"
)

var masterServices = sets.NewString("kubernetes")

// populateEnvironmentVariables populates the environment of each container (and init container) in the specified pod.
// TODO Make this the single exported function of a "pkg/environment" package in the future.
func populateEnvironmentVariables(ctx context.Context, pod *corev1.Pod, rm *manager.ResourceManager, recorder record.EventRecorder) error {

	// Populate each init container's environment.
	for idx := range pod.Spec.InitContainers {
		if err := populateContainerEnvironment(ctx, pod, &pod.Spec.InitContainers[idx], rm, recorder); err != nil {
			return err
		}
	}
	// Populate each container's environment.
	for idx := range pod.Spec.Containers {
		if err := populateContainerEnvironment(ctx, pod, &pod.Spec.Containers[idx], rm, recorder); err != nil {
			return err
		}
	}
	return nil
}

// populateContainerEnvironment populates the environment of a single container in the specified pod.
func populateContainerEnvironment(ctx context.Context, pod *corev1.Pod, container *corev1.Container, rm *manager.ResourceManager, recorder record.EventRecorder) error {
	// Create an "environment map" based on the value of the specified container's ".envFrom" field.
	envFrom, err := makeEnvironmentMapBasedOnEnvFrom(ctx, pod, container, rm, recorder)
	if err != nil {
		return err
	}
	// Create an "environment map" based on the value of the specified container's ".env" field.
	env, err := makeEnvironmentMapBasedOnEnv(ctx, pod, container, rm, recorder)
	if err != nil {
		return err
	}
	// Empty the container's ".envFrom" field and replace its ".env" field with the final, merged environment.
	// Values in "env" (sourced from ".env") will override any values with the same key defined in "envFrom" (sourced from ".envFrom").
	// This is in accordance with what the Kubelet itself does.
	// https://github.com/kubernetes/kubernetes/blob/v1.13.1/pkg/kubelet/kubelet_pods.go#L557-L558
	container.EnvFrom = []corev1.EnvFromSource{}
	container.Env = mergeEnvironments(envFrom, env)
	return nil
}

// getServiceEnvVarMap makes a map[string]string of env vars for services a
// pod in namespace ns should see.
// Based on getServiceEnvVarMap in kubelet_pods.go.
func getServiceEnvVarMap(rm *manager.ResourceManager, ns string, enableServiceLinks bool) (map[string]string, error) {
	var (
		serviceMap = make(map[string]*corev1.Service)
		m          = make(map[string]string)
	)

	services, err := rm.ListServices()
	if err != nil {
		return nil, err
	}

	// project the services in namespace ns onto the master services
	for i := range services {
		service := services[i]
		// ignore services where ClusterIP is "None" or empty
		if !v1helper.IsServiceIPSet(service) {
			continue
		}
		serviceName := service.Name

		// We always want to add environment variables for master kubernetes service
		// from the default namespace, even if enableServiceLinks is false.
		// We also add environment variables for other services in the same
		// namespace, if enableServiceLinks is true.
		if service.Namespace == metav1.NamespaceDefault && masterServices.Has(serviceName) {
			if _, exists := serviceMap[serviceName]; !exists {
				serviceMap[serviceName] = service
			}
		} else if service.Namespace == ns && enableServiceLinks {
			serviceMap[serviceName] = service
		}
	}

	mappedServices := make([]*corev1.Service, 0, len(serviceMap))
	for key := range serviceMap {
		mappedServices = append(mappedServices, serviceMap[key])
	}

	for _, e := range envvars.FromServices(mappedServices) {
		m[e.Name] = e.Value
	}
	return m, nil
}

// makeEnvironmentMapBasedOnEnvFrom returns a map representing the resolved environment of the specified container after being populated from the entries in the ".envFrom" field.
func makeEnvironmentMapBasedOnEnvFrom(ctx context.Context, pod *corev1.Pod, container *corev1.Container, rm *manager.ResourceManager, recorder record.EventRecorder) (map[string]string, error) {
	// Create a map to hold the resulting environment.
	res := make(map[string]string, 0)
	// Iterate over "envFrom" references in order to populate the environment.
loop:
	for _, envFrom := range container.EnvFrom {
		switch {
		// Handle population from a configmap.
		case envFrom.ConfigMapRef != nil:
			ef := envFrom.ConfigMapRef
			// Check whether the configmap reference is optional.
			// This will control whether we fail when unable to read the configmap.
			optional := ef.Optional != nil && *ef.Optional
			// Try to grab the referenced configmap.
			m, err := rm.GetConfigMap(ef.Name, pod.Namespace)
			if err != nil {
				// We couldn't fetch the configmap.
				// However, if the configmap reference is optional we should not fail.
				if optional {
					if errors.IsNotFound(err) {
						recorder.Eventf(pod, corev1.EventTypeWarning, ReasonOptionalConfigMapNotFound, "configmap %q not found", ef.Name)
					} else {
						log.G(ctx).Warnf("failed to read configmap %q: %v", ef.Name, err)
						recorder.Eventf(pod, corev1.EventTypeWarning, ReasonFailedToReadOptionalConfigMap, "failed to read configmap %q", ef.Name)
					}
					// Continue on to the next reference.
					continue loop
				}
				// At this point we know the configmap reference is mandatory.
				// Hence, we should return a meaningful error.
				if errors.IsNotFound(err) {
					recorder.Eventf(pod, corev1.EventTypeWarning, ReasonMandatoryConfigMapNotFound, "configmap %q not found", ef.Name)
					return nil, fmt.Errorf("configmap %q not found", ef.Name)
				}
				recorder.Eventf(pod, corev1.EventTypeWarning, ReasonFailedToReadMandatoryConfigMap, "failed to read configmap %q", ef.Name)
				return nil, fmt.Errorf("failed to fetch configmap %q: %v", ef.Name, err)
			}
			// At this point we have successfully fetched the target configmap.
			// Iterate over the keys defined in the configmap and populate the environment accordingly.
			// https://github.com/kubernetes/kubernetes/blob/v1.13.1/pkg/kubelet/kubelet_pods.go#L581-L595
			invalidKeys := make([]string, 0)
		mKeys:
			for key, val := range m.Data {
				// If a prefix has been defined, prepend it to the environment variable's name.
				if len(envFrom.Prefix) > 0 {
					key = envFrom.Prefix + key
				}
				// Make sure that the resulting key is a valid environment variable name.
				// If it isn't, it should be appended to the list of invalid keys and skipped.
				if errMsgs := apivalidation.IsEnvVarName(key); len(errMsgs) != 0 {
					invalidKeys = append(invalidKeys, key)
					continue mKeys
				}
				// Add the key and its value to the environment.
				res[key] = val
			}
			// Report any invalid keys.
			if len(invalidKeys) > 0 {
				sort.Strings(invalidKeys)
				recorder.Eventf(pod, corev1.EventTypeWarning, ReasonInvalidEnvironmentVariableNames, "keys [%s] from configmap %s/%s were skipped since they are invalid as environment variable names", strings.Join(invalidKeys, ", "), m.Namespace, m.Name)
			}
		// Handle population from a secret.
		case envFrom.SecretRef != nil:
			ef := envFrom.SecretRef
			// Check whether the secret reference is optional.
			// This will control whether we fail when unable to read the secret.
			optional := ef.Optional != nil && *ef.Optional
			// Try to grab the referenced secret.
			s, err := rm.GetSecret(ef.Name, pod.Namespace)
			if err != nil {
				// We couldn't fetch the secret.
				// However, if the secret reference is optional we should not fail.
				if optional {
					if errors.IsNotFound(err) {
						recorder.Eventf(pod, corev1.EventTypeWarning, ReasonOptionalSecretNotFound, "secret %q not found", ef.Name)
					} else {
						log.G(ctx).Warnf("failed to read secret %q: %v", ef.Name, err)
						recorder.Eventf(pod, corev1.EventTypeWarning, ReasonFailedToReadOptionalSecret, "failed to read secret %q", ef.Name)
					}
					// Continue on to the next reference.
					continue loop
				}
				// At this point we know the secret reference is mandatory.
				// Hence, we should return a meaningful error.
				if errors.IsNotFound(err) {
					recorder.Eventf(pod, corev1.EventTypeWarning, ReasonMandatorySecretNotFound, "secret %q not found", ef.Name)
					return nil, fmt.Errorf("secret %q not found", ef.Name)
				}
				recorder.Eventf(pod, corev1.EventTypeWarning, ReasonFailedToReadMandatorySecret, "failed to read secret %q", ef.Name)
				return nil, fmt.Errorf("failed to fetch secret %q: %v", ef.Name, err)
			}
			// At this point we have successfully fetched the target secret.
			// Iterate over the keys defined in the secret and populate the environment accordingly.
			// https://github.com/kubernetes/kubernetes/blob/v1.13.1/pkg/kubelet/kubelet_pods.go#L581-L595
			invalidKeys := make([]string, 0)
		sKeys:
			for key, val := range s.Data {
				// If a prefix has been defined, prepend it to the environment variable's name.
				if len(envFrom.Prefix) > 0 {
					key = envFrom.Prefix + key
				}
				// Make sure that the resulting key is a valid environment variable name.
				// If it isn't, it should be appended to the list of invalid keys and skipped.
				if errMsgs := apivalidation.IsEnvVarName(key); len(errMsgs) != 0 {
					invalidKeys = append(invalidKeys, key)
					continue sKeys
				}
				// Add the key and its value to the environment.
				res[key] = string(val)
			}
			// Report any invalid keys.
			if len(invalidKeys) > 0 {
				sort.Strings(invalidKeys)
				recorder.Eventf(pod, corev1.EventTypeWarning, ReasonInvalidEnvironmentVariableNames, "keys [%s] from secret %s/%s were skipped since they are invalid as environment variable names", strings.Join(invalidKeys, ", "), s.Namespace, s.Name)
			}
		}
	}
	// Return the populated environment.
	return res, nil
}

// makeEnvironmentMapBasedOnEnv returns a map representing the resolved environment of the specified container after being populated from the entries in the ".env" field.
func makeEnvironmentMapBasedOnEnv(ctx context.Context, pod *corev1.Pod, container *corev1.Container, rm *manager.ResourceManager, recorder record.EventRecorder) (map[string]string, error) {
	// Create a map to hold the resolved environment variables.
	res := make(map[string]string, len(container.Env))
	// Iterate over environment variables in order to populate the map.
loop:
	for _, env := range container.Env {
		switch {
		// Handle values that have been directly provided.
		case env.Value != "":
			res[env.Name] = env.Value
			continue loop
		// Handle population from a configmap key.
		case env.ValueFrom != nil && env.ValueFrom.ConfigMapKeyRef != nil:
			// The environment variable must be set from a configmap.
			vf := env.ValueFrom.ConfigMapKeyRef
			// Check whether the key reference is optional.
			// This will control whether we fail when unable to read the requested key.
			optional := vf != nil && vf.Optional != nil && *vf.Optional
			// Try to grab the referenced configmap.
			m, err := rm.GetConfigMap(vf.Name, pod.Namespace)
			if err != nil {
				// We couldn't fetch the configmap.
				// However, if the key reference is optional we should not fail.
				if optional {
					if errors.IsNotFound(err) {
						recorder.Eventf(pod, corev1.EventTypeWarning, ReasonOptionalConfigMapNotFound, "skipping optional envvar %q: configmap %q not found", env.Name, vf.Name)
					} else {
						log.G(ctx).Warnf("failed to read configmap %q: %v", vf.Name, err)
						recorder.Eventf(pod, corev1.EventTypeWarning, ReasonFailedToReadOptionalConfigMap, "skipping optional envvar %q: failed to read configmap %q", env.Name, vf.Name)
					}
					// Continue on to the next reference.
					continue loop
				}
				// At this point we know the key reference is mandatory.
				// Hence, we should return a meaningful error.
				if errors.IsNotFound(err) {
					recorder.Eventf(pod, corev1.EventTypeWarning, ReasonMandatoryConfigMapNotFound, "configmap %q not found", vf.Name)
					return nil, fmt.Errorf("configmap %q not found", vf.Name)
				}
				recorder.Eventf(pod, corev1.EventTypeWarning, ReasonFailedToReadMandatoryConfigMap, "failed to read configmap %q", vf.Name)
				return nil, fmt.Errorf("failed to read configmap %q: %v", vf.Name, err)
			}
			// At this point we have successfully fetched the target configmap.
			// We must now try to grab the requested key.
			var (
				keyExists bool
				keyValue  string
			)
			if keyValue, keyExists = m.Data[vf.Key]; !keyExists {
				// The requested key does not exist.
				// However, we should not fail if the key reference is optional.
				if optional {
					// Continue on to the next reference.
					recorder.Eventf(pod, corev1.EventTypeWarning, ReasonOptionalConfigMapKeyNotFound, "skipping optional envvar %q: key %q does not exist in configmap %q", env.Name, vf.Key, vf.Name)
					continue loop
				}
				// At this point we know the key reference is mandatory.
				// Hence, we should fail.
				recorder.Eventf(pod, corev1.EventTypeWarning, ReasonMandatoryConfigMapKeyNotFound, "key %q does not exist in configmap %q", vf.Key, vf.Name)
				return nil, fmt.Errorf("configmap %q doesn't contain the %q key required by pod %s", vf.Name, vf.Key, pod.Name)
			}
			// Populate the environment variable and continue on to the next reference.
			res[env.Name] = keyValue
			continue loop
		// Handle population from a secret key.
		case env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil:
			vf := env.ValueFrom.SecretKeyRef
			// Check whether the key reference is optional.
			// This will control whether we fail when unable to read the requested key.
			optional := vf != nil && vf.Optional != nil && *vf.Optional
			// Try to grab the referenced secret.
			s, err := rm.GetSecret(vf.Name, pod.Namespace)
			if err != nil {
				// We couldn't fetch the secret.
				// However, if the key reference is optional we should not fail.
				if optional {
					if errors.IsNotFound(err) {
						recorder.Eventf(pod, corev1.EventTypeWarning, ReasonOptionalSecretNotFound, "skipping optional envvar %q: secret %q not found", env.Name, vf.Name)
					} else {
						log.G(ctx).Warnf("failed to read secret %q: %v", vf.Name, err)
						recorder.Eventf(pod, corev1.EventTypeWarning, ReasonFailedToReadOptionalSecret, "skipping optional envvar %q: failed to read secret %q", env.Name, vf.Name)
					}
					// Continue on to the next reference.
					continue loop
				}
				// At this point we know the key reference is mandatory.
				// Hence, we should return a meaningful error.
				if errors.IsNotFound(err) {
					recorder.Eventf(pod, corev1.EventTypeWarning, ReasonMandatorySecretNotFound, "secret %q not found", vf.Name)
					return nil, fmt.Errorf("secret %q not found", vf.Name)
				}
				recorder.Eventf(pod, corev1.EventTypeWarning, ReasonFailedToReadMandatorySecret, "failed to read secret %q", vf.Name)
				return nil, fmt.Errorf("failed to read secret %q: %v", vf.Name, err)
			}
			// At this point we have successfully fetched the target secret.
			// We must now try to grab the requested key.
			var (
				keyExists bool
				keyValue  []byte
			)
			if keyValue, keyExists = s.Data[vf.Key]; !keyExists {
				// The requested key does not exist.
				// However, we should not fail if the key reference is optional.
				if optional {
					// Continue on to the next reference.
					recorder.Eventf(pod, corev1.EventTypeWarning, ReasonOptionalSecretKeyNotFound, "skipping optional envvar %q: key %q does not exist in secret %q", env.Name, vf.Key, vf.Name)
					continue loop
				}
				// At this point we know the key reference is mandatory.
				// Hence, we should fail.
				recorder.Eventf(pod, corev1.EventTypeWarning, ReasonMandatorySecretKeyNotFound, "key %q does not exist in secret %q", vf.Key, vf.Name)
				return nil, fmt.Errorf("secret %q doesn't contain the %q key required by pod %s", vf.Name, vf.Key, pod.Name)
			}
			// Populate the environment variable and continue on to the next reference.
			res[env.Name] = string(keyValue)
			continue loop
		// Handle population from a field (downward API).
		case env.ValueFrom != nil && env.ValueFrom.FieldRef != nil:
			// https://github.com/virtual-kubelet/virtual-kubelet/issues/123
			vf := env.ValueFrom.FieldRef

			runtimeVal, err := podFieldSelectorRuntimeValue(vf, pod)
			if err != nil {
				return res, err
			}

			res[env.Name] = runtimeVal

			continue loop
		// Handle population from a resource request/limit.
		case env.ValueFrom != nil && env.ValueFrom.ResourceFieldRef != nil:
			// TODO Implement populating resource requests.
			continue loop
		}
	}

	// TODO If pod.Spec.EnableServiceLinks is nil then fail as per 1.14 kubelet.
	enableServiceLinks := corev1.DefaultEnableServiceLinks
	if pod.Spec.EnableServiceLinks != nil {
		enableServiceLinks = *pod.Spec.EnableServiceLinks
	}

	// Note that there is a race between Kubelet seeing the pod and kubelet seeing the service.
	// To avoid this users can: (1) wait between starting a service and starting; or (2) detect
	// missing service env var and exit and be restarted; or (3) use DNS instead of env vars
	// and keep trying to resolve the DNS name of the service (recommended).
	svcEnv, err := getServiceEnvVarMap(rm, pod.Namespace, enableServiceLinks)
	if err != nil {
		return nil, err
	}

	// Append service env vars.
	for k, v := range svcEnv {
		if _, present := res[k]; !present {
			res[k] = v
		}
	}

	// Return the populated environment.
	return res, nil
}

// podFieldSelectorRuntimeValue returns the runtime value of the given
// selector for a pod.
func podFieldSelectorRuntimeValue(fs *corev1.ObjectFieldSelector, pod *corev1.Pod) (string, error) {
	internalFieldPath, _, err := podshelper.ConvertDownwardAPIFieldLabel(fs.APIVersion, fs.FieldPath, "")
	if err != nil {
		return "", err
	}
	switch internalFieldPath {
	case "spec.nodeName":
		return pod.Spec.NodeName, nil
	case "spec.serviceAccountName":
		return pod.Spec.ServiceAccountName, nil

	}
	return fieldpath.ExtractFieldPathAsString(pod, internalFieldPath)
}

// mergeEnvironments creates the final environment for a container by merging "envFrom" and "env".
// Values in "env" override any values with the same key defined in "envFrom".
// This is in accordance with what the Kubelet itself does.
// https://github.com/kubernetes/kubernetes/blob/v1.13.1/pkg/kubelet/kubelet_pods.go#L557-L558
func mergeEnvironments(envFrom map[string]string, env map[string]string) []corev1.EnvVar {
	tmp := make(map[string]string, 0)
	res := make([]corev1.EnvVar, 0)
	for key, val := range envFrom {
		tmp[key] = val
	}
	for key, val := range env {
		tmp[key] = val
	}
	for key, val := range tmp {
		res = append(res, corev1.EnvVar{
			Name:  key,
			Value: val,
		})
	}
	return res
}
