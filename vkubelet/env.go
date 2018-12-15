package vkubelet

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"

	"github.com/virtual-kubelet/virtual-kubelet/log"
)

const (
	// ReasonOptionalConfigMapNotFound is the reason used in events emitted when an optional config map is not found.
	ReasonOptionalConfigMapNotFound = "OptionalConfigMapNotFound"
	// ReasonOptionalConfigMapKeyNotFound is the reason used in events emitted when an optional config map key is not found.
	ReasonOptionalConfigMapKeyNotFound = "OptionalConfigMapKeyNotFound"
	// ReasonFailedToReadOptionalConfigMap is the reason used in events emitted when an optional config map could not be read.
	ReasonFailedToReadOptionalConfigMap = "FailedToReadOptionalConfigMap"

	// ReasonOptionalSecretNotFound is the reason used in events emitted when an optional secret is not found.
	ReasonOptionalSecretNotFound = "OptionalSecretNotFound"
	// ReasonOptionalSecretKeyNotFound is the reason used in events emitted when an optional secret key is not found.
	ReasonOptionalSecretKeyNotFound = "OptionalSecretKeyNotFound"
	// ReasonFailedToReadOptionalSecret is the reason used in events emitted when an optional secret could not be read.
	ReasonFailedToReadOptionalSecret = "FailedToReadOptionalSecret"

	// ReasonMandatoryConfigMapNotFound is the reason used in events emitted when an mandatory config map is not found.
	ReasonMandatoryConfigMapNotFound = "MandatoryConfigMapNotFound"
	// ReasonMandatoryConfigMapKeyNotFound is the reason used in events emitted when an mandatory config map key is not found.
	ReasonMandatoryConfigMapKeyNotFound = "MandatoryConfigMapKeyNotFound"
	// ReasonFailedToReadMandatoryConfigMap is the reason used in events emitted when an mandatory config map could not be read.
	ReasonFailedToReadMandatoryConfigMap = "FailedToReadMandatoryConfigMap"

	// ReasonMandatorySecretNotFound is the reason used in events emitted when an mandatory secret is not found.
	ReasonMandatorySecretNotFound = "MandatorySecretNotFound"
	// ReasonMandatorySecretKeyNotFound is the reason used in events emitted when an mandatory secret key is not found.
	ReasonMandatorySecretKeyNotFound = "MandatorySecretKeyNotFound"
	// ReasonFailedToReadMandatorySecret is the reason used in events emitted when an mandatory secret could not be read.
	ReasonFailedToReadMandatorySecret = "FailedToReadMandatorySecret"
)

// populateEnvironmentVariables populates Secrets and ConfigMap into environment variables
func (s *Server) populateEnvironmentVariables(ctx context.Context, pod *corev1.Pod, recorder record.EventRecorder) error {
	for _, c := range pod.Spec.Containers {
		for i, e := range c.Env {
			if e.ValueFrom != nil {
				// Populate ConfigMaps to Env
				if e.ValueFrom.ConfigMapKeyRef != nil {
					vf := e.ValueFrom.ConfigMapKeyRef
					// Check whether the key reference is optional.
					// This will control whether we fail when unable to read the requested key.
					optional := vf != nil && *vf.Optional
					// Try to grab the referenced config map.
					cm, err := s.resourceManager.GetConfigMap(vf.Name, pod.Namespace)
					if err != nil {
						// We couldn't fetch the config map.
						// However, if the key reference is optional we should not fail.
						if optional {
							if errors.IsNotFound(err) {
								recorder.Eventf(pod, corev1.EventTypeWarning, ReasonOptionalConfigMapNotFound, "skipping optional envvar %q: configmap %q not found", e.Name, vf.Name)
							} else {
								log.G(ctx).Warnf("failed to read configmap %q: %v", vf.Name, err)
								recorder.Eventf(pod, corev1.EventTypeWarning, ReasonFailedToReadOptionalConfigMap, "skipping optional envvar %q: failed to read configmap %q", e.Name, vf.Name)
							}
							// Continue on to the next reference.
							continue
						}
						// At this point we know the key reference is mandatory.
						// Hence, we should return a meaningful error.
						if errors.IsNotFound(err) {
							recorder.Eventf(pod, corev1.EventTypeWarning, ReasonMandatoryConfigMapNotFound, "configmap %q not found", vf.Name)
							return fmt.Errorf("required configmap %q not found", vf.Name)
						}
						recorder.Eventf(pod, corev1.EventTypeWarning, ReasonFailedToReadMandatoryConfigMap, "failed to read configmap %q", vf.Name)
						return fmt.Errorf("failed to read required configmap %q: %v", vf.Name, err)
					}
					// At this point we have successfully fetched the target config map.
					// We must now try to grab the requested key.
					var (
						keyExists bool
						keyValue  string
					)
					if keyValue, keyExists = cm.Data[vf.Key]; !keyExists {
						// The requested key does not exist.
						// However, we should not fail if the key reference is optional.
						if optional {
							// Continue on to the next reference.
							recorder.Eventf(pod, corev1.EventTypeWarning, ReasonOptionalConfigMapKeyNotFound, "skipping optional envvar %q: key %q does not exist in configmap %q", e.Name, vf.Key, vf.Name)
							continue
						}
						// At this point we know the key reference is mandatory.
						// Hence, we should fail.
						recorder.Eventf(pod, corev1.EventTypeWarning, ReasonMandatoryConfigMapKeyNotFound, "key %q does not exist in configmap %q", vf.Key, vf.Name)
						return fmt.Errorf("configmap %q doesn't contain the %q key required by pod %s", vf.Name, vf.Key, pod.Name)
					}
					// Populate the environment variable and continue on to the next reference.
					c.Env[i].Value = keyValue
					continue
				}
				// Populate Secrets to Env
				if e.ValueFrom.SecretKeyRef != nil {
					vf := e.ValueFrom.SecretKeyRef
					// Check whether the key reference is optional.
					// This will control whether we fail when unable to read the requested key.
					optional := vf != nil && *vf.Optional
					// Try to grab the referenced secret.
					cm, err := s.resourceManager.GetSecret(vf.Name, pod.Namespace)
					if err != nil {
						// We couldn't fetch the secret.
						// However, if the key reference is optional we should not fail.
						if optional {
							if errors.IsNotFound(err) {
								recorder.Eventf(pod, corev1.EventTypeWarning, ReasonOptionalSecretNotFound, "skipping optional envvar %q: secret %q not found", e.Name, vf.Name)
							} else {
								log.G(ctx).Warnf("failed to read secret %q: %v", vf.Name, err)
								recorder.Eventf(pod, corev1.EventTypeWarning, ReasonFailedToReadOptionalSecret, "skipping optional envvar %q: failed to read secret %q", e.Name, vf.Name)
							}
							// Continue on to the next reference.
							continue
						}
						// At this point we know the key reference is mandatory.
						// Hence, we should return a meaningful error.
						if errors.IsNotFound(err) {
							recorder.Eventf(pod, corev1.EventTypeWarning, ReasonMandatorySecretNotFound, "secret %q not found", vf.Name)
							return fmt.Errorf("required secret %q not found", vf.Name)
						}
						recorder.Eventf(pod, corev1.EventTypeWarning, ReasonFailedToReadMandatorySecret, "failed to read secret %q", vf.Name)
						return fmt.Errorf("failed to read secret %q: %v", vf.Name, err)
					}
					// At this point we have successfully fetched the target secret.
					// We must now try to grab the requested key.
					var (
						keyExists bool
						keyValue  []byte
					)
					if keyValue, keyExists = cm.Data[vf.Key]; !keyExists {
						// The requested key does not exist.
						// However, we should not fail if the key reference is optional.
						if optional {
							// Continue on to the next reference.
							recorder.Eventf(pod, corev1.EventTypeWarning, ReasonOptionalSecretKeyNotFound, "skipping optional envvar %q: key %q does not exist in secret %q", e.Name, vf.Key, vf.Name)
							continue
						}
						// At this point we know the key reference is mandatory.
						// Hence, we should fail.
						recorder.Eventf(pod, corev1.EventTypeWarning, ReasonMandatorySecretKeyNotFound, "key %q does not exist in secret %q", vf.Key, vf.Name)
						return fmt.Errorf("secret %q doesn't contain the %q key required by pod %s", vf.Name, vf.Key, pod.Name)
					}
					// Populate the environment variable and continue on to the next reference.
					c.Env[i].Value = string(keyValue)
					continue
				}

				// TODO: Populate Downward API to Env
				if e.ValueFrom.FieldRef != nil {
					continue
				}

				// TODO: Populate resource requests
				if e.ValueFrom.ResourceFieldRef != nil {
					continue
				}
			}
		}
	}
	return nil
}
