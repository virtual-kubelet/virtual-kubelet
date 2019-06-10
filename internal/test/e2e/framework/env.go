package framework

import (
	corev1 "k8s.io/api/core/v1"
)

var (
	bFalse = false
	bTrue  = true
)

// CreatePodObjectWithMandatoryConfigMapKey creates a pod object that references the "key_0" key from the "config-map-0" config map as mandatory.
func (f *Framework) CreatePodObjectWithMandatoryConfigMapKey(testName string) *corev1.Pod {
	return f.CreatePodObjectWithEnv(testName, []corev1.EnvVar{
		{
			Name: "CONFIG_MAP_0_KEY_0",
			ValueFrom: &corev1.EnvVarSource{
				ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: "config-map-0"},
					Key:                  "key_0",
					Optional:             &bFalse,
				},
			},
		},
	})
}

// CreatePodObjectWithOptionalConfigMapKey creates a pod object that references the "key_0" key from the "config-map-0" config map as optional.
func (f *Framework) CreatePodObjectWithOptionalConfigMapKey(testName string) *corev1.Pod {
	return f.CreatePodObjectWithEnv(testName, []corev1.EnvVar{
		{
			Name: "CONFIG_MAP_0_KEY_0",
			ValueFrom: &corev1.EnvVarSource{
				ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: "config-map-0"},
					Key:                  "key_0",
					Optional:             &bTrue,
				},
			},
		},
	})
}

// CreatePodObjectWithMandatorySecretKey creates a pod object that references the "key_0" key from the "secret-0" config map as mandatory.
func (f *Framework) CreatePodObjectWithMandatorySecretKey(testName string) *corev1.Pod {
	return f.CreatePodObjectWithEnv(testName, []corev1.EnvVar{
		{
			Name: "SECRET_0_KEY_0",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: "secret-0"},
					Key:                  "key_0",
					Optional:             &bFalse,
				},
			},
		},
	})
}

// CreatePodObjectWithOptionalSecretKey creates a pod object that references the "key_0" key from the "secret-0" config map as optional.
func (f *Framework) CreatePodObjectWithOptionalSecretKey(testName string) *corev1.Pod {
	return f.CreatePodObjectWithEnv(testName, []corev1.EnvVar{
		{
			Name: "SECRET_0_KEY_0",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: "secret-0"},
					Key:                  "key_0",
					Optional:             &bTrue,
				},
			},
		},
	})
}

// CreatePodObjectWithEnv creates a pod object whose name starts with "env-test-" and that uses the specified environment configuration for its first container.
func (f *Framework) CreatePodObjectWithEnv(testName string, env []corev1.EnvVar) *corev1.Pod {
	pod := f.CreateDummyPodObjectWithPrefix(testName, "env-test-", "foo")
	pod.Spec.Containers[0].Env = env
	return pod
}
