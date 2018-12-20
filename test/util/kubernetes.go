package util

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
)

// FakeConfigMap returns a configmap with the specified namespace, name and data.
func FakeConfigMap(namespace, name string, data map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Data: data,
	}
}

// FakeEventRecorder returns an event recorder that can be used to capture events.
func FakeEventRecorder(bufferSize int) *record.FakeRecorder {
	return record.NewFakeRecorder(bufferSize)
}

// FakePodWithSingleContainer returns a pod with the specified namespace and name, and having a single container with the specified image.
func FakePodWithSingleContainer(namespace, name, image string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  name,
					Image: image,
				},
			},
		},
	}
}

// FakeSecret returns a secret with the specified namespace, name and data.
func FakeSecret(namespace, name string, data map[string]string) *corev1.Secret {
	res := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Data: make(map[string][]byte),
	}
	for key, val := range data {
		res.Data[key] = []byte(val)
	}
	return res
}
