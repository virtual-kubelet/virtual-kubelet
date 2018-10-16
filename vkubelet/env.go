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
					cm, err := s.resourceManager.GetConfigMap(vf.Name, pod.Namespace)
					if vf.Optional != nil && !*vf.Optional && errors.IsNotFound(err) {
						return fmt.Errorf("ConfigMap %s is required by Pod %s and does not exist", vf.Name, pod.Name)
					}

					if err != nil {
						return fmt.Errorf("Error retrieving ConfigMap %s required by Pod %s: %s", vf.Name, pod.Name, err)
					}

					var ok bool
					if c.Env[i].Value, ok = cm.Data[vf.Key]; !ok {
						return fmt.Errorf("ConfigMap %s key %s is required by Pod %s and does not exist", vf.Name, vf.Key, pod.Name)
					}
					continue
				}

				// Populate Secrets to Env
				if e.ValueFrom.SecretKeyRef != nil {
					vf := e.ValueFrom.SecretKeyRef
					sec, err := s.resourceManager.GetSecret(vf.Name, pod.Namespace)
					if vf.Optional != nil && !*vf.Optional && errors.IsNotFound(err) {
						return fmt.Errorf("Secret %s is required by Pod %s and does not exist", vf.Name, pod.Name)
					}
					v, ok := sec.Data[vf.Key]
					if !ok {
						return fmt.Errorf("Secret %s key %s is required by Pod %s and does not exist", vf.Name, vf.Key, pod.Name)
					}
					c.Env[i].Value = string(v)
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
