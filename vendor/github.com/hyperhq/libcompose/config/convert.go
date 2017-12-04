package config

import "github.com/hyperhq/libcompose/utils"

// ConvertV1toV2 converts a v1 service config to a v2 service config
func ConvertV1toV2(v1Services map[string]*ServiceConfigV1, environmentLookup EnvironmentLookup, resourceLookup ResourceLookup) (map[string]*ServiceConfig, error) {
	v2Services := make(map[string]*ServiceConfig)

	/*
		builds := make(map[string]Build)
		logs := make(map[string]Log)

		for name, service := range v1Services {
			builds[name] = Build{
				Context:    service.Build,
				Dockerfile: service.Dockerfile,
			}

			v1Services[name].Build = ""
			v1Services[name].Dockerfile = ""

			logs[name] = Log{
				Driver:  service.LogDriver,
				Options: service.LogOpt,
			}

			v1Services[name].LogDriver = ""
			v1Services[name].LogOpt = nil
		}
	*/

	if err := utils.Convert(v1Services, &v2Services); err != nil {
		return nil, err
	}

	/*
		for name := range v2Services {
			v2Services[name].Build = builds[name]
		}
	*/

	return v2Services, nil
}
