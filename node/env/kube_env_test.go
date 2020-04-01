package env

import (
	"context"
	"sort"
	"testing"

	testutil "github.com/elotl/virtual-kubelet/internal/test/util"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	// TODO: remove this import if
	// api.Registry.GroupOrDie(v1.GroupName).GroupVersions[0].String() is changed
	// to "v1"?
)

type envs []v1.EnvVar

func (e envs) Len() int {
	return len(e)
}

func (e envs) Swap(i, j int) { e[i], e[j] = e[j], e[i] }

func (e envs) Less(i, j int) bool { return e[i].Name < e[j].Name }

func buildService(name, namespace, clusterIP, protocol string, port int) *v1.Service {
	return &v1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{{
				Protocol: v1.Protocol(protocol),
				Port:     int32(port),
			}},
			ClusterIP: clusterIP,
		},
	}
}

func TestMakeEnvironmentVariables(t *testing.T) {
	services := []*v1.Service{
		buildService("kubernetes", metav1.NamespaceDefault, "1.2.3.1", "TCP", 8081),
		buildService("test", "test1", "1.2.3.3", "TCP", 8083),
		buildService("test", "test2", "1.2.3.5", "TCP", 8085),
	}

	trueValue := true
	falseValue := false
	testCases := []struct {
		name               string        // the name of the test case
		ns                 string        // the namespace to generate environment for
		enableServiceLinks *bool         // enabling service links
		container          *v1.Container // the container to use
		masterServiceNs    string        // the namespace to read master service info from
		nilLister          bool          // whether the lister should be nil
		configMap          *v1.ConfigMap // an optional ConfigMap to pull from
		secret             *v1.Secret    // an optional Secret to pull from
		expectedEnvs       []v1.EnvVar   // a set of expected environment vars
		expectedError      bool          // does the test fail
		expectedEvent      string        // does the test emit an event
	}{
		{
			name:               "api server = Y, kubelet = Y",
			ns:                 "test1",
			enableServiceLinks: &falseValue,
			container: &v1.Container{
				Env: []v1.EnvVar{
					{Name: "FOO", Value: "BAR"},
					{Name: "TEST_SERVICE_HOST", Value: "1.2.3.3"},
					{Name: "TEST_SERVICE_PORT", Value: "8083"},
					{Name: "TEST_PORT", Value: "tcp://1.2.3.3:8083"},
					{Name: "TEST_PORT_8083_TCP", Value: "tcp://1.2.3.3:8083"},
					{Name: "TEST_PORT_8083_TCP_PROTO", Value: "tcp"},
					{Name: "TEST_PORT_8083_TCP_PORT", Value: "8083"},
					{Name: "TEST_PORT_8083_TCP_ADDR", Value: "1.2.3.3"},
				},
			},
			masterServiceNs: metav1.NamespaceDefault,
			nilLister:       false,
			expectedEnvs: []v1.EnvVar{
				{Name: "FOO", Value: "BAR"},
				{Name: "TEST_SERVICE_HOST", Value: "1.2.3.3"},
				{Name: "TEST_SERVICE_PORT", Value: "8083"},
				{Name: "TEST_PORT", Value: "tcp://1.2.3.3:8083"},
				{Name: "TEST_PORT_8083_TCP", Value: "tcp://1.2.3.3:8083"},
				{Name: "TEST_PORT_8083_TCP_PROTO", Value: "tcp"},
				{Name: "TEST_PORT_8083_TCP_PORT", Value: "8083"},
				{Name: "TEST_PORT_8083_TCP_ADDR", Value: "1.2.3.3"},
				{Name: "KUBERNETES_SERVICE_PORT", Value: "8081"},
				{Name: "KUBERNETES_SERVICE_HOST", Value: "1.2.3.1"},
				{Name: "KUBERNETES_PORT", Value: "tcp://1.2.3.1:8081"},
				{Name: "KUBERNETES_PORT_8081_TCP", Value: "tcp://1.2.3.1:8081"},
				{Name: "KUBERNETES_PORT_8081_TCP_PROTO", Value: "tcp"},
				{Name: "KUBERNETES_PORT_8081_TCP_PORT", Value: "8081"},
				{Name: "KUBERNETES_PORT_8081_TCP_ADDR", Value: "1.2.3.1"},
			},
		},
		{
			name:               "api server = Y, kubelet = N",
			ns:                 "test1",
			enableServiceLinks: &falseValue,
			container: &v1.Container{
				Env: []v1.EnvVar{
					{Name: "FOO", Value: "BAR"},
					{Name: "TEST_SERVICE_HOST", Value: "1.2.3.3"},
					{Name: "TEST_SERVICE_PORT", Value: "8083"},
					{Name: "TEST_PORT", Value: "tcp://1.2.3.3:8083"},
					{Name: "TEST_PORT_8083_TCP", Value: "tcp://1.2.3.3:8083"},
					{Name: "TEST_PORT_8083_TCP_PROTO", Value: "tcp"},
					{Name: "TEST_PORT_8083_TCP_PORT", Value: "8083"},
					{Name: "TEST_PORT_8083_TCP_ADDR", Value: "1.2.3.3"},
				},
			},
			masterServiceNs: metav1.NamespaceDefault,
			nilLister:       true,
			expectedEnvs: []v1.EnvVar{
				{Name: "FOO", Value: "BAR"},
				{Name: "TEST_SERVICE_HOST", Value: "1.2.3.3"},
				{Name: "TEST_SERVICE_PORT", Value: "8083"},
				{Name: "TEST_PORT", Value: "tcp://1.2.3.3:8083"},
				{Name: "TEST_PORT_8083_TCP", Value: "tcp://1.2.3.3:8083"},
				{Name: "TEST_PORT_8083_TCP_PROTO", Value: "tcp"},
				{Name: "TEST_PORT_8083_TCP_PORT", Value: "8083"},
				{Name: "TEST_PORT_8083_TCP_ADDR", Value: "1.2.3.3"},
			},
		},
		{
			name:               "api server = N; kubelet = Y",
			ns:                 "test1",
			enableServiceLinks: &falseValue,
			container: &v1.Container{
				Env: []v1.EnvVar{
					{Name: "FOO", Value: "BAZ"},
				},
			},
			masterServiceNs: metav1.NamespaceDefault,
			nilLister:       false,
			expectedEnvs: []v1.EnvVar{
				{Name: "FOO", Value: "BAZ"},
				{Name: "KUBERNETES_SERVICE_HOST", Value: "1.2.3.1"},
				{Name: "KUBERNETES_SERVICE_PORT", Value: "8081"},
				{Name: "KUBERNETES_PORT", Value: "tcp://1.2.3.1:8081"},
				{Name: "KUBERNETES_PORT_8081_TCP", Value: "tcp://1.2.3.1:8081"},
				{Name: "KUBERNETES_PORT_8081_TCP_PROTO", Value: "tcp"},
				{Name: "KUBERNETES_PORT_8081_TCP_PORT", Value: "8081"},
				{Name: "KUBERNETES_PORT_8081_TCP_ADDR", Value: "1.2.3.1"},
			},
		},
		{
			name:               "api server = N; kubelet = Y; service env vars",
			ns:                 "test1",
			enableServiceLinks: &trueValue,
			container: &v1.Container{
				Env: []v1.EnvVar{
					{Name: "FOO", Value: "BAZ"},
				},
			},
			masterServiceNs: metav1.NamespaceDefault,
			nilLister:       false,
			expectedEnvs: []v1.EnvVar{
				{Name: "FOO", Value: "BAZ"},
				{Name: "TEST_SERVICE_HOST", Value: "1.2.3.3"},
				{Name: "TEST_SERVICE_PORT", Value: "8083"},
				{Name: "TEST_PORT", Value: "tcp://1.2.3.3:8083"},
				{Name: "TEST_PORT_8083_TCP", Value: "tcp://1.2.3.3:8083"},
				{Name: "TEST_PORT_8083_TCP_PROTO", Value: "tcp"},
				{Name: "TEST_PORT_8083_TCP_PORT", Value: "8083"},
				{Name: "TEST_PORT_8083_TCP_ADDR", Value: "1.2.3.3"},
				{Name: "KUBERNETES_SERVICE_HOST", Value: "1.2.3.1"},
				{Name: "KUBERNETES_SERVICE_PORT", Value: "8081"},
				{Name: "KUBERNETES_PORT", Value: "tcp://1.2.3.1:8081"},
				{Name: "KUBERNETES_PORT_8081_TCP", Value: "tcp://1.2.3.1:8081"},
				{Name: "KUBERNETES_PORT_8081_TCP_PROTO", Value: "tcp"},
				{Name: "KUBERNETES_PORT_8081_TCP_PORT", Value: "8081"},
				{Name: "KUBERNETES_PORT_8081_TCP_ADDR", Value: "1.2.3.1"},
			},
		},
		{
			name:               "downward api pod",
			ns:                 "downward-api",
			enableServiceLinks: &falseValue,
			container: &v1.Container{
				Env: []v1.EnvVar{
					{
						Name: "POD_NAME",
						ValueFrom: &v1.EnvVarSource{
							FieldRef: &v1.ObjectFieldSelector{
								APIVersion: "v1",
								FieldPath:  "metadata.name",
							},
						},
					},
					{
						Name: "POD_NAMESPACE",
						ValueFrom: &v1.EnvVarSource{
							FieldRef: &v1.ObjectFieldSelector{
								APIVersion: "v1",
								FieldPath:  "metadata.namespace",
							},
						},
					},
					{
						Name: "POD_NODE_NAME",
						ValueFrom: &v1.EnvVarSource{
							FieldRef: &v1.ObjectFieldSelector{
								APIVersion: "v1",
								FieldPath:  "spec.nodeName",
							},
						},
					},
					{
						Name: "POD_SERVICE_ACCOUNT_NAME",
						ValueFrom: &v1.EnvVarSource{
							FieldRef: &v1.ObjectFieldSelector{
								APIVersion: "v1",
								FieldPath:  "spec.serviceAccountName",
							},
						},
					},
					// {
					// 	Name: "POD_IP",
					// 	ValueFrom: &v1.EnvVarSource{
					// 		FieldRef: &v1.ObjectFieldSelector{
					// 			APIVersion: "v1",
					// 			FieldPath:  "status.podIP",
					// 		},
					// 	},
					// },
					// {
					// 	Name: "POD_IPS",
					// 	ValueFrom: &v1.EnvVarSource{
					// 		FieldRef: &v1.ObjectFieldSelector{
					// 			APIVersion: "v1",
					// 			FieldPath:  "status.podIPs",
					// 		},
					// 	},
					// },
					// {
					// 	Name: "HOST_IP",
					// 	ValueFrom: &v1.EnvVarSource{
					// 		FieldRef: &v1.ObjectFieldSelector{
					// 			APIVersion: "v1",
					// 			FieldPath:  "status.hostIP",
					// 		},
					// 	},
					// },
				},
			},
			masterServiceNs: "nothing",
			nilLister:       true,
			expectedEnvs: []v1.EnvVar{
				{Name: "POD_NAME", Value: "dapi-test-pod-name"},
				{Name: "POD_NAMESPACE", Value: "downward-api"},
				{Name: "POD_NODE_NAME", Value: "node-name"},
				{Name: "POD_SERVICE_ACCOUNT_NAME", Value: "special"},
			},
		},
		{
			name:               "env expansion",
			ns:                 "test1",
			enableServiceLinks: &falseValue,
			container: &v1.Container{
				Env: []v1.EnvVar{
					{
						Name:  "TEST_LITERAL",
						Value: "test-test-test",
					},
					{
						Name: "POD_NAME",
						ValueFrom: &v1.EnvVarSource{
							FieldRef: &v1.ObjectFieldSelector{
								APIVersion: "v1", //legacyscheme.Registry.GroupOrDie(v1.GroupName).GroupVersion.String(),
								FieldPath:  "metadata.name",
							},
						},
					},
					{
						Name:  "OUT_OF_ORDER_TEST",
						Value: "$(OUT_OF_ORDER_TARGET)",
					},
					{
						Name:  "OUT_OF_ORDER_TARGET",
						Value: "FOO",
					},
					{
						Name: "EMPTY_VAR",
					},
					{
						Name:  "EMPTY_TEST",
						Value: "foo-$(EMPTY_VAR)",
					},
					{
						Name:  "POD_NAME_TEST2",
						Value: "test2-$(POD_NAME)",
					},
					{
						Name:  "POD_NAME_TEST3",
						Value: "$(POD_NAME_TEST2)-3",
					},
					{
						Name:  "LITERAL_TEST",
						Value: "literal-$(TEST_LITERAL)",
					},
					{
						Name:  "TEST_UNDEFINED",
						Value: "$(UNDEFINED_VAR)",
					},
				},
			},
			masterServiceNs: "nothing",
			nilLister:       false,
			expectedEnvs: []v1.EnvVar{
				{
					Name:  "TEST_LITERAL",
					Value: "test-test-test",
				},
				{
					Name:  "POD_NAME",
					Value: "dapi-test-pod-name",
				},
				{
					Name:  "POD_NAME_TEST2",
					Value: "test2-dapi-test-pod-name",
				},
				{
					Name:  "POD_NAME_TEST3",
					Value: "test2-dapi-test-pod-name-3",
				},
				{
					Name:  "LITERAL_TEST",
					Value: "literal-test-test-test",
				},
				{
					Name:  "OUT_OF_ORDER_TEST",
					Value: "$(OUT_OF_ORDER_TARGET)",
				},
				{
					Name:  "OUT_OF_ORDER_TARGET",
					Value: "FOO",
				},
				{
					Name:  "TEST_UNDEFINED",
					Value: "$(UNDEFINED_VAR)",
				},
				{
					Name: "EMPTY_VAR",
				},
				{
					Name:  "EMPTY_TEST",
					Value: "foo-",
				},
			},
		},
		{
			name:               "env expansion, service env vars",
			ns:                 "test1",
			enableServiceLinks: &trueValue,
			container: &v1.Container{
				Env: []v1.EnvVar{
					{
						Name:  "TEST_LITERAL",
						Value: "test-test-test",
					},
					{
						Name: "POD_NAME",
						ValueFrom: &v1.EnvVarSource{
							FieldRef: &v1.ObjectFieldSelector{
								APIVersion: "v1",
								FieldPath:  "metadata.name",
							},
						},
					},
					{
						Name:  "OUT_OF_ORDER_TEST",
						Value: "$(OUT_OF_ORDER_TARGET)",
					},
					{
						Name:  "OUT_OF_ORDER_TARGET",
						Value: "FOO",
					},
					{
						Name: "EMPTY_VAR",
					},
					{
						Name:  "EMPTY_TEST",
						Value: "foo-$(EMPTY_VAR)",
					},
					{
						Name:  "POD_NAME_TEST2",
						Value: "test2-$(POD_NAME)",
					},
					{
						Name:  "POD_NAME_TEST3",
						Value: "$(POD_NAME_TEST2)-3",
					},
					{
						Name:  "LITERAL_TEST",
						Value: "literal-$(TEST_LITERAL)",
					},
					{
						Name:  "SERVICE_VAR_TEST",
						Value: "$(TEST_SERVICE_HOST):$(TEST_SERVICE_PORT)",
					},
					{
						Name:  "TEST_UNDEFINED",
						Value: "$(UNDEFINED_VAR)",
					},
				},
			},
			masterServiceNs: "nothing",
			nilLister:       false,
			expectedEnvs: []v1.EnvVar{
				{
					Name:  "TEST_LITERAL",
					Value: "test-test-test",
				},
				{
					Name:  "POD_NAME",
					Value: "dapi-test-pod-name",
				},
				{
					Name:  "POD_NAME_TEST2",
					Value: "test2-dapi-test-pod-name",
				},
				{
					Name:  "POD_NAME_TEST3",
					Value: "test2-dapi-test-pod-name-3",
				},
				{
					Name:  "LITERAL_TEST",
					Value: "literal-test-test-test",
				},
				{
					Name:  "TEST_SERVICE_HOST",
					Value: "1.2.3.3",
				},
				{
					Name:  "TEST_SERVICE_PORT",
					Value: "8083",
				},
				{
					Name:  "TEST_PORT",
					Value: "tcp://1.2.3.3:8083",
				},
				{
					Name:  "TEST_PORT_8083_TCP",
					Value: "tcp://1.2.3.3:8083",
				},
				{
					Name:  "TEST_PORT_8083_TCP_PROTO",
					Value: "tcp",
				},
				{
					Name:  "TEST_PORT_8083_TCP_PORT",
					Value: "8083",
				},
				{
					Name:  "TEST_PORT_8083_TCP_ADDR",
					Value: "1.2.3.3",
				},
				{
					Name:  "SERVICE_VAR_TEST",
					Value: "1.2.3.3:8083",
				},
				{
					Name:  "OUT_OF_ORDER_TEST",
					Value: "$(OUT_OF_ORDER_TARGET)",
				},
				{
					Name:  "OUT_OF_ORDER_TARGET",
					Value: "FOO",
				},
				{
					Name:  "TEST_UNDEFINED",
					Value: "$(UNDEFINED_VAR)",
				},
				{
					Name: "EMPTY_VAR",
				},
				{
					Name:  "EMPTY_TEST",
					Value: "foo-",
				},
			},
		},
		{
			name:               "configmapkeyref_missing_optional",
			ns:                 "test",
			enableServiceLinks: &falseValue,
			container: &v1.Container{
				Env: []v1.EnvVar{
					{
						Name: "POD_NAME",
						ValueFrom: &v1.EnvVarSource{
							ConfigMapKeyRef: &v1.ConfigMapKeySelector{
								LocalObjectReference: v1.LocalObjectReference{Name: "missing-config-map"},
								Key:                  "key",
								Optional:             &trueValue,
							},
						},
					},
				},
			},
			masterServiceNs: "nothing",
			expectedEnvs:    []v1.EnvVar{},
			expectedEvent:   "Warning OptionalConfigMapNotFound skipping optional envvar \"POD_NAME\": configmap \"missing-config-map\" not found",
		},
		{
			name:               "configmapkeyref_missing_key_optional",
			ns:                 "test",
			enableServiceLinks: &falseValue,
			container: &v1.Container{
				Env: []v1.EnvVar{
					{
						Name: "POD_NAME",
						ValueFrom: &v1.EnvVarSource{
							ConfigMapKeyRef: &v1.ConfigMapKeySelector{
								LocalObjectReference: v1.LocalObjectReference{Name: "test-config-map"},
								Key:                  "key",
								Optional:             &trueValue,
							},
						},
					},
				},
			},
			masterServiceNs: "nothing",
			nilLister:       true,
			configMap: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test1",
					Name:      "test-configmap",
				},
				Data: map[string]string{
					"a": "b",
				},
			},
			expectedEnvs:  []v1.EnvVar{},
			expectedEvent: "Warning OptionalConfigMapNotFound skipping optional envvar \"POD_NAME\": configmap \"test-config-map\" not found",
		},
		{
			name:               "secretkeyref_missing_optional",
			ns:                 "test",
			enableServiceLinks: &falseValue,
			container: &v1.Container{
				Env: []v1.EnvVar{
					{
						Name: "POD_NAME",
						ValueFrom: &v1.EnvVarSource{
							SecretKeyRef: &v1.SecretKeySelector{
								LocalObjectReference: v1.LocalObjectReference{Name: "missing-secret"},
								Key:                  "key",
								Optional:             &trueValue,
							},
						},
					},
				},
			},
			masterServiceNs: "nothing",
			expectedEnvs:    []v1.EnvVar{},
			expectedEvent:   "Warning OptionalSecretNotFound skipping optional envvar \"POD_NAME\": secret \"missing-secret\" not found",
		},
		{
			name:               "secretkeyref_missing_key_optional",
			ns:                 "test",
			enableServiceLinks: &falseValue,
			container: &v1.Container{
				Env: []v1.EnvVar{
					{
						Name: "POD_NAME",
						ValueFrom: &v1.EnvVarSource{
							SecretKeyRef: &v1.SecretKeySelector{
								LocalObjectReference: v1.LocalObjectReference{Name: "test-secret"},
								Key:                  "key",
								Optional:             &trueValue,
							},
						},
					},
				},
			},
			masterServiceNs: "nothing",
			nilLister:       true,
			secret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test1",
					Name:      "test-secret",
				},
				Data: map[string][]byte{
					"a": []byte("b"),
				},
			},
			expectedEnvs:  []v1.EnvVar{},
			expectedEvent: "Warning OptionalSecretNotFound skipping optional envvar \"POD_NAME\": secret \"test-secret\" not found",
		},
		{
			name:               "configmap",
			ns:                 "test1",
			enableServiceLinks: &falseValue,
			container: &v1.Container{
				EnvFrom: []v1.EnvFromSource{
					{
						ConfigMapRef: &v1.ConfigMapEnvSource{LocalObjectReference: v1.LocalObjectReference{Name: "test-config-map"}},
					},
					{
						Prefix:       "p_",
						ConfigMapRef: &v1.ConfigMapEnvSource{LocalObjectReference: v1.LocalObjectReference{Name: "test-config-map"}},
					},
				},
				Env: []v1.EnvVar{
					{
						Name:  "TEST_LITERAL",
						Value: "test-test-test",
					},
					{
						Name:  "EXPANSION_TEST",
						Value: "$(REPLACE_ME)",
					},
					{
						Name:  "DUPE_TEST",
						Value: "ENV_VAR",
					},
				},
			},
			masterServiceNs: "nothing",
			nilLister:       false,
			configMap: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test1",
					Name:      "test-config-map",
				},
				Data: map[string]string{
					"REPLACE_ME": "FROM_CONFIG_MAP",
					"DUPE_TEST":  "CONFIG_MAP",
				},
			},
			expectedEnvs: []v1.EnvVar{
				{
					Name:  "TEST_LITERAL",
					Value: "test-test-test",
				},
				{
					Name:  "REPLACE_ME",
					Value: "FROM_CONFIG_MAP",
				},
				{
					Name:  "EXPANSION_TEST",
					Value: "FROM_CONFIG_MAP",
				},
				{
					Name:  "DUPE_TEST",
					Value: "ENV_VAR",
				},
				{
					Name:  "p_REPLACE_ME",
					Value: "FROM_CONFIG_MAP",
				},
				{
					Name:  "p_DUPE_TEST",
					Value: "CONFIG_MAP",
				},
			},
		},
		{
			name:               "configmap, service env vars",
			ns:                 "test1",
			enableServiceLinks: &trueValue,
			container: &v1.Container{
				EnvFrom: []v1.EnvFromSource{
					{
						ConfigMapRef: &v1.ConfigMapEnvSource{LocalObjectReference: v1.LocalObjectReference{Name: "test-config-map"}},
					},
					{
						Prefix:       "p_",
						ConfigMapRef: &v1.ConfigMapEnvSource{LocalObjectReference: v1.LocalObjectReference{Name: "test-config-map"}},
					},
				},
				Env: []v1.EnvVar{
					{
						Name:  "TEST_LITERAL",
						Value: "test-test-test",
					},
					{
						Name:  "EXPANSION_TEST",
						Value: "$(REPLACE_ME)",
					},
					{
						Name:  "DUPE_TEST",
						Value: "ENV_VAR",
					},
				},
			},
			masterServiceNs: "nothing",
			nilLister:       false,
			configMap: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test1",
					Name:      "test-config-map",
				},
				Data: map[string]string{
					"REPLACE_ME": "FROM_CONFIG_MAP",
					"DUPE_TEST":  "CONFIG_MAP",
				},
			},
			expectedEnvs: []v1.EnvVar{
				{
					Name:  "TEST_LITERAL",
					Value: "test-test-test",
				},
				{
					Name:  "TEST_SERVICE_HOST",
					Value: "1.2.3.3",
				},
				{
					Name:  "TEST_SERVICE_PORT",
					Value: "8083",
				},
				{
					Name:  "TEST_PORT",
					Value: "tcp://1.2.3.3:8083",
				},
				{
					Name:  "TEST_PORT_8083_TCP",
					Value: "tcp://1.2.3.3:8083",
				},
				{
					Name:  "TEST_PORT_8083_TCP_PROTO",
					Value: "tcp",
				},
				{
					Name:  "TEST_PORT_8083_TCP_PORT",
					Value: "8083",
				},
				{
					Name:  "TEST_PORT_8083_TCP_ADDR",
					Value: "1.2.3.3",
				},
				{
					Name:  "REPLACE_ME",
					Value: "FROM_CONFIG_MAP",
				},
				{
					Name:  "EXPANSION_TEST",
					Value: "FROM_CONFIG_MAP",
				},
				{
					Name:  "DUPE_TEST",
					Value: "ENV_VAR",
				},
				{
					Name:  "p_REPLACE_ME",
					Value: "FROM_CONFIG_MAP",
				},
				{
					Name:  "p_DUPE_TEST",
					Value: "CONFIG_MAP",
				},
			},
		},
		{
			name:               "configmap_missing",
			ns:                 "test1",
			enableServiceLinks: &falseValue,
			container: &v1.Container{
				EnvFrom: []v1.EnvFromSource{
					{ConfigMapRef: &v1.ConfigMapEnvSource{LocalObjectReference: v1.LocalObjectReference{Name: "test-config-map"}}},
				},
			},
			masterServiceNs: "nothing",
			expectedError:   true,
			expectedEvent:   "Warning MandatoryConfigMapNotFound configmap \"test-config-map\" not found",
		},
		{
			name:               "configmap_missing_optional",
			ns:                 "test",
			enableServiceLinks: &falseValue,
			container: &v1.Container{
				EnvFrom: []v1.EnvFromSource{
					{ConfigMapRef: &v1.ConfigMapEnvSource{
						Optional:             &trueValue,
						LocalObjectReference: v1.LocalObjectReference{Name: "missing-config-map"}}},
				},
			},
			masterServiceNs: "nothing",
			expectedEnvs:    []v1.EnvVar{},
			expectedEvent:   "Warning OptionalConfigMapNotFound configmap \"missing-config-map\" not found",
		},
		{
			name:               "configmap_invalid_keys",
			ns:                 "test1",
			enableServiceLinks: &falseValue,
			container: &v1.Container{
				EnvFrom: []v1.EnvFromSource{
					{ConfigMapRef: &v1.ConfigMapEnvSource{LocalObjectReference: v1.LocalObjectReference{Name: "test-config-map"}}},
				},
			},
			masterServiceNs: "nothing",
			configMap: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test1",
					Name:      "test-config-map",
				},
				Data: map[string]string{
					"1234": "abc",
					"1z":   "abc",
					"key":  "value",
				},
			},
			expectedEnvs: []v1.EnvVar{
				{
					Name:  "key",
					Value: "value",
				},
			},
			expectedEvent: "Warning InvalidEnvironmentVariableNames Keys [1234, 1z] from the EnvFrom configMap test1/test-config-map were skipped since they are considered invalid environment variable names.",
		},
		{
			name:               "configmap_invalid_keys_valid",
			ns:                 "test1",
			enableServiceLinks: &falseValue,
			container: &v1.Container{
				EnvFrom: []v1.EnvFromSource{
					{
						Prefix:       "p_",
						ConfigMapRef: &v1.ConfigMapEnvSource{LocalObjectReference: v1.LocalObjectReference{Name: "test-config-map"}},
					},
				},
			},
			masterServiceNs: "",
			configMap: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test1",
					Name:      "test-config-map",
				},
				Data: map[string]string{
					"1234": "abc",
				},
			},
			expectedEnvs: []v1.EnvVar{
				{
					Name:  "p_1234",
					Value: "abc",
				},
			},
		},
		{
			name:               "secret",
			ns:                 "test1",
			enableServiceLinks: &falseValue,
			container: &v1.Container{
				EnvFrom: []v1.EnvFromSource{
					{
						SecretRef: &v1.SecretEnvSource{LocalObjectReference: v1.LocalObjectReference{Name: "test-secret"}},
					},
					{
						Prefix:    "p_",
						SecretRef: &v1.SecretEnvSource{LocalObjectReference: v1.LocalObjectReference{Name: "test-secret"}},
					},
				},
				Env: []v1.EnvVar{
					{
						Name:  "TEST_LITERAL",
						Value: "test-test-test",
					},
					{
						Name:  "EXPANSION_TEST",
						Value: "$(REPLACE_ME)",
					},
					{
						Name:  "DUPE_TEST",
						Value: "ENV_VAR",
					},
				},
			},
			masterServiceNs: "nothing",
			nilLister:       false,
			secret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test1",
					Name:      "test-secret",
				},
				Data: map[string][]byte{
					"REPLACE_ME": []byte("FROM_SECRET"),
					"DUPE_TEST":  []byte("SECRET"),
				},
			},
			expectedEnvs: []v1.EnvVar{
				{
					Name:  "TEST_LITERAL",
					Value: "test-test-test",
				},
				{
					Name:  "REPLACE_ME",
					Value: "FROM_SECRET",
				},
				{
					Name:  "EXPANSION_TEST",
					Value: "FROM_SECRET",
				},
				{
					Name:  "DUPE_TEST",
					Value: "ENV_VAR",
				},
				{
					Name:  "p_REPLACE_ME",
					Value: "FROM_SECRET",
				},
				{
					Name:  "p_DUPE_TEST",
					Value: "SECRET",
				},
			},
		},
		{
			name:               "secret, service env vars",
			ns:                 "test1",
			enableServiceLinks: &trueValue,
			container: &v1.Container{
				EnvFrom: []v1.EnvFromSource{
					{
						SecretRef: &v1.SecretEnvSource{LocalObjectReference: v1.LocalObjectReference{Name: "test-secret"}},
					},
					{
						Prefix:    "p_",
						SecretRef: &v1.SecretEnvSource{LocalObjectReference: v1.LocalObjectReference{Name: "test-secret"}},
					},
				},
				Env: []v1.EnvVar{
					{
						Name:  "TEST_LITERAL",
						Value: "test-test-test",
					},
					{
						Name:  "EXPANSION_TEST",
						Value: "$(REPLACE_ME)",
					},
					{
						Name:  "DUPE_TEST",
						Value: "ENV_VAR",
					},
				},
			},
			masterServiceNs: "nothing",
			nilLister:       false,
			secret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test1",
					Name:      "test-secret",
				},
				Data: map[string][]byte{
					"REPLACE_ME": []byte("FROM_SECRET"),
					"DUPE_TEST":  []byte("SECRET"),
				},
			},
			expectedEnvs: []v1.EnvVar{
				{
					Name:  "TEST_LITERAL",
					Value: "test-test-test",
				},
				{
					Name:  "TEST_SERVICE_HOST",
					Value: "1.2.3.3",
				},
				{
					Name:  "TEST_SERVICE_PORT",
					Value: "8083",
				},
				{
					Name:  "TEST_PORT",
					Value: "tcp://1.2.3.3:8083",
				},
				{
					Name:  "TEST_PORT_8083_TCP",
					Value: "tcp://1.2.3.3:8083",
				},
				{
					Name:  "TEST_PORT_8083_TCP_PROTO",
					Value: "tcp",
				},
				{
					Name:  "TEST_PORT_8083_TCP_PORT",
					Value: "8083",
				},
				{
					Name:  "TEST_PORT_8083_TCP_ADDR",
					Value: "1.2.3.3",
				},
				{
					Name:  "REPLACE_ME",
					Value: "FROM_SECRET",
				},
				{
					Name:  "EXPANSION_TEST",
					Value: "FROM_SECRET",
				},
				{
					Name:  "DUPE_TEST",
					Value: "ENV_VAR",
				},
				{
					Name:  "p_REPLACE_ME",
					Value: "FROM_SECRET",
				},
				{
					Name:  "p_DUPE_TEST",
					Value: "SECRET",
				},
			},
		},
		{
			name:               "secret_missing",
			ns:                 "test1",
			enableServiceLinks: &falseValue,
			container: &v1.Container{
				EnvFrom: []v1.EnvFromSource{
					{SecretRef: &v1.SecretEnvSource{LocalObjectReference: v1.LocalObjectReference{Name: "test-secret"}}},
				},
			},
			masterServiceNs: "nothing",
			expectedError:   true,
			expectedEvent:   "Warning MandatorySecretNotFound secret \"test-secret\" not found",
		},
		{
			name:               "secret_missing_optional",
			ns:                 "test",
			enableServiceLinks: &falseValue,
			container: &v1.Container{
				EnvFrom: []v1.EnvFromSource{
					{SecretRef: &v1.SecretEnvSource{
						LocalObjectReference: v1.LocalObjectReference{Name: "missing-secret"},
						Optional:             &trueValue}},
				},
			},
			masterServiceNs: "nothing",
			expectedEnvs:    []v1.EnvVar{},
			expectedEvent:   "Warning OptionalSecretNotFound secret \"missing-secret\" not found",
		},
		{
			name:               "secret_invalid_keys",
			ns:                 "test1",
			enableServiceLinks: &falseValue,
			container: &v1.Container{
				EnvFrom: []v1.EnvFromSource{
					{SecretRef: &v1.SecretEnvSource{LocalObjectReference: v1.LocalObjectReference{Name: "test-secret"}}},
				},
			},
			masterServiceNs: "nothing",
			secret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test1",
					Name:      "test-secret",
				},
				Data: map[string][]byte{
					"1234":  []byte("abc"),
					"1z":    []byte("abc"),
					"key.1": []byte("value"),
				},
			},
			expectedEnvs: []v1.EnvVar{
				{
					Name:  "key.1",
					Value: "value",
				},
			},
			expectedEvent: "Warning InvalidEnvironmentVariableNames Keys [1234, 1z] from the EnvFrom secret test1/test-secret were skipped since they are considered invalid environment variable names.",
		},
		{
			name:               "secret_invalid_keys_valid",
			ns:                 "test1",
			enableServiceLinks: &falseValue,
			container: &v1.Container{
				EnvFrom: []v1.EnvFromSource{
					{
						Prefix:    "p_",
						SecretRef: &v1.SecretEnvSource{LocalObjectReference: v1.LocalObjectReference{Name: "test-secret"}},
					},
				},
			},
			masterServiceNs: "",
			secret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test1",
					Name:      "test-secret",
				},
				Data: map[string][]byte{
					"1234.name": []byte("abc"),
				},
			},
			expectedEnvs: []v1.EnvVar{
				{
					Name:  "p_1234.name",
					Value: "abc",
				},
			},
		},
		{
			name:               "nil_enableServiceLinks",
			ns:                 "test",
			enableServiceLinks: nil,
			container: &v1.Container{
				EnvFrom: []v1.EnvFromSource{
					{
						Prefix:    "p_",
						SecretRef: &v1.SecretEnvSource{LocalObjectReference: v1.LocalObjectReference{Name: "test-secret"}},
					},
				},
			},
			masterServiceNs: "",
			secret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test1",
					Name:      "test-secret",
				},
				Data: map[string][]byte{
					"1234.name": []byte("abc"),
				},
			},
			expectedError: true,
			expectedEvent: "Warning MandatorySecretNotFound secret \"test-secret\" not found",
		},
	}

	for _, tc := range testCases {
		objs := []runtime.Object{}
		if !tc.nilLister {
			for i := range services {
				if services[i].Name == "kubernetes" &&
					tc.masterServiceNs != metav1.NamespaceDefault {
					continue
				}
				objs = append(objs, services[i])
			}
		}
		if tc.configMap != nil {
			objs = append(objs, tc.configMap)
		}
		if tc.secret != nil {
			objs = append(objs, tc.secret)
		}
		rm := testutil.FakeResourceManager(objs...)
		er := testutil.FakeEventRecorder(defaultEventRecorderBufferSize)

		testPod := &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: tc.ns,
				Name:      "dapi-test-pod-name",
			},
			Spec: v1.PodSpec{
				ServiceAccountName: "special",
				NodeName:           "node-name",
				EnableServiceLinks: tc.enableServiceLinks,
				Containers: []corev1.Container{
					*tc.container,
				},
			},
		}

		err := PopulateEnvironmentVariables(context.Background(), testPod, rm, er)

		select {
		case e := <-er.Events:
			assert.Equal(t, tc.expectedEvent, e, tc.name)
		default:
			assert.Equal(t, "", tc.expectedEvent)
		}
		if tc.expectedError {
			assert.Assert(t, err != nil, tc.name)
		} else {
			assert.NilError(t, err, "[%s]", tc.name)

			sort.Sort(envs(testPod.Spec.Containers[0].Env))
			sort.Sort(envs(tc.expectedEnvs))
			assert.Check(t, is.DeepEqual(tc.expectedEnvs, testPod.Spec.Containers[0].Env), "[%s] env entries", tc.name)
		}
	}
}
