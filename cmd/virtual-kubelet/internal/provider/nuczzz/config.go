package nuczzz

import (
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

type Capacity struct {
	CPU    string `yaml:"cpu"`
	Memory string `yaml:"memory"`
	Pods   string `yaml:"pods"`
}

type Config struct {
	Kubeconfig string   `yaml:"kubeconfig"`
	Capacity   Capacity `yaml:"capacity"`
	Namespace  string   `yaml:"namespace"`
}

func parseConfig(file string, obj interface{}) error {
	if file == "" {
		return errors.Errorf("config file is null")
	}

	v := viper.New()
	v.SetConfigFile(file)
	v.SetConfigType("yaml")
	if err := v.ReadInConfig(); err != nil {
		return errors.Wrap(err, "ReadInConfig error")
	}

	return v.Unmarshal(obj)
}
