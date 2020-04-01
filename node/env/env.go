package env

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/elotl/virtual-kubelet/internal/manager"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	apivalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/tools/record"
	podshelper "k8s.io/kubernetes/pkg/apis/core/pods"
	v1helper "k8s.io/kubernetes/pkg/apis/core/v1/helper"
	"k8s.io/kubernetes/pkg/fieldpath"
	"k8s.io/kubernetes/pkg/kubelet/envvars"
	"k8s.io/kubernetes/third_party/forked/golang/expansion"
)

const (
	// ReasonOptionalConfigMapNotFound is the reason used in events emitted when an optional configmap is not found.
	ReasonOptionalConfigMapNotFound = "OptionalConfigMapNotFound"
	// ReasonOptionalConfigMapKeyNotFound is the reason used in events emitted when an optional configmap key is not found.
	ReasonOptionalConfigMapKeyNotFound = "OptionalConfigMapKeyNotFound"

	// ReasonOptionalSecretNotFound is the reason used in events emitted when an optional secret is not found.
	ReasonOptionalSecretNotFound = "OptionalSecretNotFound"
	// ReasonOptionalSecretKeyNotFound is the reason used in events emitted when an optional secret key is not found.
	ReasonOptionalSecretKeyNotFound = "OptionalSecretKeyNotFound"

	// ReasonMandatoryConfigMapNotFound is the reason used in events emitted when a mandatory configmap is not found.
	ReasonMandatoryConfigMapNotFound = "MandatoryConfigMapNotFound"
	// ReasonMandatoryConfigMapKeyNotFound is the reason used in events emitted when a mandatory configmap key is not found.
	ReasonMandatoryConfigMapKeyNotFound = "MandatoryConfigMapKeyNotFound"
	// ReasonFailedToReadMandatoryConfigMap is the reason used in events emitted when a mandatory configmap could not be read.
	ReasonFailedToReadMandatoryConfigMap = "FailedToReadMandatoryConfigMap"

	// ReasonMandatorySecretNotFound is the reason used in events emitted when a mandatory secret is not found.
	ReasonMandatorySecretNotFound = "MandatorySecretNotFound"
	// ReasonMandatorySecretKeyNotFound is the reason used in events emitted when a mandatory secret key is not found.
	ReasonMandatorySecretKeyNotFound = "MandatorySecretKeyNotFound"
	// ReasonFailedToReadMandatorySecret is the reason used in events emitted when a mandatory secret could not be read.
	ReasonFailedToReadMandatorySecret = "FailedToReadMandatorySecret"

	// ReasonInvalidEnvironmentVariableNames is the reason used in events emitted when a configmap/secret referenced in a ".spec.containers[*].envFrom" field contains invalid environment variable names.
	ReasonInvalidEnvironmentVariableNames = "InvalidEnvironmentVariableNames"
)

var masterServices = sets.NewString("kubernetes")

func populateEnvironmentVariables(ctx context.Context, pod *corev1.Pod, rm *manager.ResourceManager, recorder record.EventRecorder) error {
	err := ResolveConfigMapRefs(ctx, pod, rm, recorder)
	if err != nil {
		return err
	}
	err = ResolveSecretRefs(ctx, pod, rm, recorder)
	if err != nil {
		return err
	}
	err = InsertServiceEnvVars(ctx, pod, rm)
	if err != nil {
		return err
	}
	err = ResolveFieldRefs(pod)
	if err != nil {
		return err
	}
	ResolveEnvVarExpansions(pod)
	Uniqify(pod)
	RemoveUnresolvedVars(pod)
	return nil
}

func ResolveConfigMapRefs(ctx context.Context, pod *corev1.Pod, rm *manager.ResourceManager, recorder record.EventRecorder) error {
	configMaps := make(map[string]*corev1.ConfigMap)

	for i := range pod.Spec.InitContainers {
		if err := resolveContainerConfigMapRefs(ctx, pod, &pod.Spec.InitContainers[i], rm, recorder, configMaps); err != nil {
			return err
		}
	}
	// Populate each container's environment.
	for i := range pod.Spec.Containers {
		if err := resolveContainerConfigMapRefs(ctx, pod, &pod.Spec.Containers[i], rm, recorder, configMaps); err != nil {
			return err
		}
	}
	return nil
}

func resolveContainerConfigMapRefs(ctx context.Context, pod *corev1.Pod, container *corev1.Container, rm *manager.ResourceManager, recorder record.EventRecorder, configMaps map[string]*v1.ConfigMap) error {
	envFromVals := make([]corev1.EnvVar, 0)
	for i := range container.EnvFrom {
		if container.EnvFrom[i].ConfigMapRef != nil {
			envFrom := container.EnvFrom[i]
			cm := envFrom.ConfigMapRef
			name := cm.Name
			configMap, ok := configMaps[name]
			if !ok {
				var err error
				optional := cm.Optional != nil && *cm.Optional
				configMap, err = rm.GetConfigMap(name, pod.Namespace)
				if err != nil {
					if errors.IsNotFound(err) && optional {
						recorder.Eventf(pod, corev1.EventTypeWarning, ReasonOptionalConfigMapNotFound, "configmap %q not found", name)
						continue
					}
					if errors.IsNotFound(err) {
						recorder.Eventf(pod, corev1.EventTypeWarning, ReasonMandatoryConfigMapNotFound, "configmap %q not found", name)
						return fmt.Errorf("configmap %q not found", name)
					}
					recorder.Eventf(pod, corev1.EventTypeWarning, ReasonFailedToReadMandatoryConfigMap, "failed to read configmap %q", name)
					return fmt.Errorf("failed to fetch configmap %q: %v", name, err)
				}
				configMaps[name] = configMap
			}

			invalidKeys := []string{}
			for k, v := range configMap.Data {
				if len(envFrom.Prefix) > 0 {
					k = envFrom.Prefix + k
				}
				if errMsgs := apivalidation.IsEnvVarName(k); len(errMsgs) != 0 {
					invalidKeys = append(invalidKeys, k)
					continue
				}
				envFromVals = append(envFromVals, corev1.EnvVar{
					Name:  k,
					Value: v,
				})
			}
			if len(invalidKeys) > 0 {
				sort.Strings(invalidKeys)
				recorder.Eventf(pod, v1.EventTypeWarning, ReasonInvalidEnvironmentVariableNames, "Keys [%s] from the EnvFrom configMap %s/%s were skipped since they are considered invalid environment variable names.", strings.Join(invalidKeys, ", "), pod.Namespace, name)
			}
		}
	}

	for i := range container.Env {
		if container.Env[i].ValueFrom != nil && container.Env[i].ValueFrom.ConfigMapKeyRef != nil {
			envVar := container.Env[i]
			cm := envVar.ValueFrom.ConfigMapKeyRef
			name := cm.Name
			key := cm.Key
			optional := cm.Optional != nil && *cm.Optional
			configMap, ok := configMaps[name]
			if !ok {
				var err error
				configMap, err = rm.GetConfigMap(name, pod.Namespace)
				if err != nil {
					if errors.IsNotFound(err) && optional {
						// ignore error when marked optional
						recorder.Eventf(pod, corev1.EventTypeWarning, ReasonOptionalConfigMapNotFound, "skipping optional envvar %q: configmap %q not found", envVar.Name, name)
						continue
					}
					if errors.IsNotFound(err) {
						recorder.Eventf(pod, corev1.EventTypeWarning, ReasonMandatoryConfigMapNotFound, "configmap %q not found", name)
						return fmt.Errorf("configmap %q not found", name)
					}
					recorder.Eventf(pod, corev1.EventTypeWarning, ReasonFailedToReadMandatoryConfigMap, "failed to read configmap %q", name)
					return fmt.Errorf("failed to read configmap %q: %v", name, err)
				}
				configMaps[name] = configMap
			}
			runtimeVal, ok := configMap.Data[key]
			if !ok {
				if optional {
					recorder.Eventf(pod, corev1.EventTypeWarning, ReasonOptionalConfigMapKeyNotFound, "skipping optional envvar %q: key %q does not exist in configmap %q", envVar.Name, key, name)
					continue
				}
				recorder.Eventf(pod, corev1.EventTypeWarning, ReasonMandatoryConfigMapKeyNotFound, "key %q does not exist in configmap %q", key, name)
				return fmt.Errorf("configmap %q doesn't contain the %q key required by pod %s", name, key, pod.Name)
			}
			container.Env[i].Value = runtimeVal
			container.Env[i].ValueFrom = nil
		}
	}

	container.Env = append(envFromVals, container.Env...)
	return nil
}

func ResolveSecretRefs(ctx context.Context, pod *corev1.Pod, rm *manager.ResourceManager, recorder record.EventRecorder) error {
	secrets := make(map[string]*corev1.Secret)

	for i := range pod.Spec.InitContainers {
		if err := resolveContainerSecretRefs(ctx, pod, &pod.Spec.InitContainers[i], rm, recorder, secrets); err != nil {
			return err
		}
	}
	for i := range pod.Spec.Containers {
		if err := resolveContainerSecretRefs(ctx, pod, &pod.Spec.Containers[i], rm, recorder, secrets); err != nil {
			return err
		}
	}
	return nil
}

func resolveContainerSecretRefs(ctx context.Context, pod *corev1.Pod, container *corev1.Container, rm *manager.ResourceManager, recorder record.EventRecorder, secrets map[string]*v1.Secret) error {
	envFromVals := make([]corev1.EnvVar, 0)
	for i := range container.EnvFrom {
		if container.EnvFrom[i].SecretRef != nil {
			envFrom := container.EnvFrom[i]
			s := envFrom.SecretRef
			name := s.Name
			secret, ok := secrets[name]
			if !ok {
				var err error
				optional := s.Optional != nil && *s.Optional
				secret, err = rm.GetSecret(name, pod.Namespace)
				if err != nil {
					if errors.IsNotFound(err) && optional {
						// ignore error when marked optional
						recorder.Eventf(pod, corev1.EventTypeWarning, ReasonOptionalSecretNotFound, "secret %q not found", name)
						continue
					}
					if errors.IsNotFound(err) {
						recorder.Eventf(pod, corev1.EventTypeWarning, ReasonMandatorySecretNotFound, "secret %q not found", name)
						return fmt.Errorf("secret %q not found", name)
					}
					recorder.Eventf(pod, corev1.EventTypeWarning, ReasonFailedToReadMandatorySecret, "failed to read secret %q", name)
					return fmt.Errorf("failed to fetch secret %q: %v", name, err)
				}
				secrets[name] = secret
			}

			invalidKeys := []string{}
			for k, v := range secret.Data {
				if len(envFrom.Prefix) > 0 {
					k = envFrom.Prefix + k
				}
				if errMsgs := apivalidation.IsEnvVarName(k); len(errMsgs) != 0 {
					invalidKeys = append(invalidKeys, k)
					continue
				}
				envFromVals = append(envFromVals, corev1.EnvVar{
					Name:  k,
					Value: string(v),
				})
			}
			if len(invalidKeys) > 0 {
				sort.Strings(invalidKeys)
				recorder.Eventf(pod, v1.EventTypeWarning, ReasonInvalidEnvironmentVariableNames, "Keys [%s] from the EnvFrom secret %s/%s were skipped since they are considered invalid environment variable names.", strings.Join(invalidKeys, ", "), pod.Namespace, name)
			}
		}
	}

	for i := range container.Env {
		if container.Env[i].ValueFrom != nil && container.Env[i].ValueFrom.SecretKeyRef != nil {
			envVar := container.Env[i]
			s := envVar.ValueFrom.SecretKeyRef
			name := s.Name
			key := s.Key
			optional := s.Optional != nil && *s.Optional
			secret, ok := secrets[name]
			if !ok {
				var err error
				secret, err = rm.GetSecret(name, pod.Namespace)
				if err != nil {
					if errors.IsNotFound(err) && optional {
						// ignore error when marked optional
						recorder.Eventf(pod, corev1.EventTypeWarning, ReasonOptionalSecretNotFound, "skipping optional envvar %q: secret %q not found", envVar.Name, name)
						continue
					}
					if errors.IsNotFound(err) {
						recorder.Eventf(pod, corev1.EventTypeWarning, ReasonMandatorySecretNotFound, "secret %q not found", name)
						return fmt.Errorf("secret %q not found", name)
					}
					recorder.Eventf(pod, corev1.EventTypeWarning, ReasonFailedToReadMandatorySecret, "failed to read secret %q", name)
					return fmt.Errorf("failed to read secret %q: %v", name, err)
				}
				secrets[name] = secret
			}
			runtimeValBytes, ok := secret.Data[key]
			if !ok {
				if optional {
					recorder.Eventf(pod, corev1.EventTypeWarning, ReasonOptionalSecretKeyNotFound, "skipping optional envvar %q: key %q does not exist in secret %q", envVar.Name, key, name)
					continue
				}
				recorder.Eventf(pod, corev1.EventTypeWarning, ReasonMandatorySecretKeyNotFound, "key %q does not exist in secret %q", key, name)
				return fmt.Errorf("couldn't find key %v in Secret %v/%v", key, pod.Namespace, name)
			}
			container.Env[i].Value = string(runtimeValBytes)
			container.Env[i].ValueFrom = nil
		}
	}
	container.Env = append(envFromVals, container.Env...)
	return nil
}

func InsertServiceEnvVars(ctx context.Context, pod *corev1.Pod, rm *manager.ResourceManager) error {
	// TODO If pod.Spec.EnableServiceLinks is nil then fail as per 1.14 kubelet.
	enableServiceLinks := corev1.DefaultEnableServiceLinks
	if pod.Spec.EnableServiceLinks != nil {
		enableServiceLinks = *pod.Spec.EnableServiceLinks
	}

	svcEnv, err := getServiceEnvVarSlice(rm, pod.Namespace, enableServiceLinks)
	if err != nil {
		return err
	}

	for i := range pod.Spec.InitContainers {
		envCopy := make([]corev1.EnvVar, 0, len(svcEnv)+len(pod.Spec.InitContainers[i].Env))
		envCopy = append(envCopy, svcEnv...)
		pod.Spec.InitContainers[i].Env = append(envCopy, pod.Spec.InitContainers[i].Env...)
	}
	for i := range pod.Spec.Containers {
		envCopy := make([]corev1.EnvVar, 0, len(svcEnv)+len(pod.Spec.Containers[i].Env))
		envCopy = append(envCopy, svcEnv...)
		pod.Spec.Containers[i].Env = append(envCopy, pod.Spec.Containers[i].Env...)
	}
	return nil
}

// getServiceEnvVarMap makes a map[string]string of env vars for services a
// pod in namespace ns should see.
// Based on getServiceEnvVarMap in kubelet_pods.go.
func getServiceEnvVarSlice(rm *manager.ResourceManager, ns string, enableServiceLinks bool) ([]corev1.EnvVar, error) {
	var (
		serviceMap = make(map[string]*corev1.Service)
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

	return envvars.FromServices(mappedServices), nil

}

func ResolveFieldRefs(pod *corev1.Pod) error {
	for i := range pod.Spec.InitContainers {
		err := resolveContainerFieldRefs(pod, &pod.Spec.InitContainers[i])
		if err != nil {
			return err
		}
	}
	for i := range pod.Spec.Containers {
		err := resolveContainerFieldRefs(pod, &pod.Spec.Containers[i])
		if err != nil {
			return err
		}

	}
	return nil
}

func resolveContainerFieldRefs(pod *corev1.Pod, container *corev1.Container) error {
	for i := range container.Env {
		if container.Env[i].ValueFrom != nil && container.Env[i].ValueFrom.FieldRef != nil {
			val, err := podFieldSelectorRuntimeValue(container.Env[i].ValueFrom.FieldRef, pod)
			if err != nil {
				return err
			}
			container.Env[i].Value = val
			container.Env[i].ValueFrom = nil
		}
	}
	return nil
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

func ResolveResourceRefs(pod *corev1.Pod, node *corev1.Node) error {
	return nil
}

func ResolveEnvVarExpansions(pod *corev1.Pod) {
	for i := range pod.Spec.InitContainers {
		resolveContainerEnvVarExpansions(&pod.Spec.InitContainers[i])
	}
	for i := range pod.Spec.Containers {
		resolveContainerEnvVarExpansions(&pod.Spec.Containers[i])
	}
}

func resolveContainerEnvVarExpansions(container *corev1.Container) {
	res := make(map[string]string)
	mappingFunc := expansion.MappingFuncFor(res)
	for i := range container.Env {
		container.Env[i].Value = expansion.Expand(container.Env[i].Value, mappingFunc)
		res[container.Env[i].Name] = container.Env[i].Value
	}
}

func Uniqify(pod *corev1.Pod) {
	for i := range pod.Spec.InitContainers {
		uniqifyContainer(&pod.Spec.InitContainers[i])
	}
	for i := range pod.Spec.Containers {
		uniqifyContainer(&pod.Spec.Containers[i])
	}
}

func uniqifyContainer(container *corev1.Container) {
	seenVars := sets.NewString()
	keepVars := make([]corev1.EnvVar, 0, len(container.Env))
	for i := len(container.Env) - 1; i >= 0; i-- {
		if !seenVars.Has(container.Env[i].Name) {
			keepVars = append(keepVars, container.Env[i])
			seenVars.Insert(container.Env[i].Name)
		}
	}
	container.Env = keepVars
}

func RemoveUnresolvedVars(pod *corev1.Pod) {
	for i := range pod.Spec.InitContainers {
		removeContainerUnresolvedVars(&pod.Spec.InitContainers[i])
	}
	for i := range pod.Spec.Containers {
		removeContainerUnresolvedVars(&pod.Spec.Containers[i])
	}
}

func removeContainerUnresolvedVars(container *corev1.Container) {
	keepVars := make([]corev1.EnvVar, 0, len(container.Env))
	for i := range container.Env {
		if container.Env[i].Value == "" && container.Env[i].ValueFrom != nil {
			continue
		}
		container.Env[i].ValueFrom = nil
		keepVars = append(keepVars, container.Env[i])
	}
	container.Env = keepVars
}
