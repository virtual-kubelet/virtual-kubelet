package cmd

import (
	"os"

	"github.com/cpuguy83/strongerrors"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
)

// Default taint values
const (
	DefaultTaintEffect = corev1.TaintEffectNoSchedule
	DefaultTaintKey    = "virtual-kubelet.io/provider"
)

func getEnv(key, defaultValue string) string {
	value, found := os.LookupEnv(key)
	if found {
		return value
	}
	return defaultValue
}

// getTaint creates a taint using the provided key/value.
// Taint effect is read from the environment
// The taint key/value may be overwritten by the environment.
func getTaint(key, value string) (*corev1.Taint, error) {
	if key == "" {
		key = DefaultTaintKey
		value = provider
	}

	key = getEnv("VKUBELET_TAINT_KEY", key)
	value = getEnv("VKUBELET_TAINT_VALUE", value)
	effectEnv := getEnv("VKUBELET_TAINT_EFFECT", string(DefaultTaintEffect))

	var effect corev1.TaintEffect
	switch effectEnv {
	case "NoSchedule":
		effect = corev1.TaintEffectNoSchedule
	case "NoExecute":
		effect = corev1.TaintEffectNoExecute
	case "PreferNoSchedule":
		effect = corev1.TaintEffectPreferNoSchedule
	default:
		return nil, strongerrors.InvalidArgument(errors.Errorf("taint effect %q is not supported", effectEnv))
	}

	return &corev1.Taint{
		Key:    key,
		Value:  value,
		Effect: effect,
	}, nil
}
