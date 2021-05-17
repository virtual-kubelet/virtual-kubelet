/*
Copyright 2014 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package podutils

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"
)

// ConvertDownwardAPIFieldLabel converts the specified downward API field label
// and its value in the pod of the specified version to the internal version,
// and returns the converted label and value. This function returns an error if
// the conversion fails.
func ConvertDownwardAPIFieldLabel(version, label, value string) (string, string, error) {
	if version != "v1" {
		return "", "", fmt.Errorf("unsupported pod version: %s", version)
	}

	if path, _, ok := SplitMaybeSubscriptedPath(label); ok {
		switch path {
		case "metadata.annotations", "metadata.labels":
			return label, value, nil
		default:
			return "", "", fmt.Errorf("field label does not support subscript: %s", label)
		}
	}

	switch label {
	case "metadata.annotations",
		"metadata.labels",
		"metadata.name",
		"metadata.namespace",
		"metadata.uid",
		"spec.nodeName",
		"spec.restartPolicy",
		"spec.serviceAccountName",
		"spec.schedulerName",
		"status.phase",
		"status.hostIP",
		"status.podIP",
		"status.podIPs":
		return label, value, nil
	// This is for backwards compatibility with old v1 clients which send spec.host
	case "spec.host":
		return "spec.nodeName", value, nil
	default:
		return "", "", fmt.Errorf("field label not supported: %s", label)
	}
}

// ExtractFieldPathAsString extracts the field from the given object
// and returns it as a string.  The object must be a pointer to an
// API type.
func ExtractFieldPathAsString(obj interface{}, fieldPath string) (string, error) {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return "", err
	}

	if path, subscript, ok := SplitMaybeSubscriptedPath(fieldPath); ok {
		switch path {
		case "metadata.annotations":
			if errs := validation.IsQualifiedName(strings.ToLower(subscript)); len(errs) != 0 {
				return "", fmt.Errorf("invalid key subscript in %s: %s", fieldPath, strings.Join(errs, ";"))
			}
			return accessor.GetAnnotations()[subscript], nil
		case "metadata.labels":
			if errs := validation.IsQualifiedName(subscript); len(errs) != 0 {
				return "", fmt.Errorf("invalid key subscript in %s: %s", fieldPath, strings.Join(errs, ";"))
			}
			return accessor.GetLabels()[subscript], nil
		default:
			return "", fmt.Errorf("fieldPath %q does not support subscript", fieldPath)
		}
	}

	switch fieldPath {
	case "metadata.annotations":
		return FormatMap(accessor.GetAnnotations()), nil
	case "metadata.labels":
		return FormatMap(accessor.GetLabels()), nil
	case "metadata.name":
		return accessor.GetName(), nil
	case "metadata.namespace":
		return accessor.GetNamespace(), nil
	case "metadata.uid":
		return string(accessor.GetUID()), nil
	}

	return "", fmt.Errorf("unsupported fieldPath: %v", fieldPath)
}

// SplitMaybeSubscriptedPath checks whether the specified fieldPath is
// subscripted, and
//  - if yes, this function splits the fieldPath into path and subscript, and
//    returns (path, subscript, true).
//  - if no, this function returns (fieldPath, "", false).
//
// Example inputs and outputs:
//  - "metadata.annotations['myKey']" --> ("metadata.annotations", "myKey", true)
//  - "metadata.annotations['a[b]c']" --> ("metadata.annotations", "a[b]c", true)
//  - "metadata.labels['']"           --> ("metadata.labels", "", true)
//  - "metadata.labels"               --> ("metadata.labels", "", false)
func SplitMaybeSubscriptedPath(fieldPath string) (string, string, bool) {
	if !strings.HasSuffix(fieldPath, "']") {
		return fieldPath, "", false
	}
	s := strings.TrimSuffix(fieldPath, "']")
	parts := strings.SplitN(s, "['", 2)
	if len(parts) < 2 {
		return fieldPath, "", false
	}
	if len(parts[0]) == 0 {
		return fieldPath, "", false
	}
	return parts[0], parts[1], true
}

// FormatMap formats map[string]string to a string.
func FormatMap(m map[string]string) (fmtStr string) {
	// output with keys in sorted order to provide stable output
	keys := sets.NewString()
	for key := range m {
		keys.Insert(key)
	}
	for _, key := range keys.List() {
		fmtStr += fmt.Sprintf("%v=%q\n", key, m[key])
	}
	fmtStr = strings.TrimSuffix(fmtStr, "\n")

	return
}

// IsServiceIPSet aims to check if the service's ClusterIP is set or not the objective is not to perform validation here
func IsServiceIPSet(service *corev1.Service) bool {
	return service.Spec.ClusterIP != corev1.ClusterIPNone && service.Spec.ClusterIP != ""
}
