package vkubelet

import (
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
)

// populateEnvironmentVariables populates Secrets and ConfigMap into environment variables
func (s *Server) populateEnvironmentVariables(pod *corev1.Pod) error {
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
							// TODO: Emit an event (the contents of which possibly depend on whether the error is "NotFound", for clarity).
							continue
						}
						// At this point we know the key reference is mandatory.
						// Hence, we should return a meaningful error.
						if errors.IsNotFound(err) {
							return fmt.Errorf("required configmap %q not found", vf.Name)
						}
						return fmt.Errorf("failed to fetch required configmap %q: %v", vf.Name, err)
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
							// TODO: Emit an event.
							continue
						}
						// At this point we know the key reference is mandatory.
						// Hence, we should fail.
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
							// TODO: Emit an event (the contents of which possibly depend on whether the error is "NotFound", for clarity).
							continue
						}
						// At this point we know the key reference is mandatory.
						// Hence, we should return a meaningful error.
						if errors.IsNotFound(err) {
							return fmt.Errorf("required secret %q not found", vf.Name)
						}
						return fmt.Errorf("failed to fetch required secret %q: %v", vf.Name, err)
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
							// TODO: Emit an event.
							continue
						}
						// At this point we know the key reference is mandatory.
						// Hence, we should fail.
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
