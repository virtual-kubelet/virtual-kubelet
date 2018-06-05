package stack

import (
	"golang.org/x/net/context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/cli/command"
)

func deployBundle(ctx context.Context, dockerCli *command.DockerCli, opts deployOptions) error {
	bundle, err := loadBundlefile(dockerCli.Err(), opts.namespace, opts.bundlefile)
	if err != nil {
		return err
	}

	if err := checkDaemonIsSwarmManager(ctx, dockerCli); err != nil {
		return err
	}

	namespace := namespace{name: opts.namespace}

	networks := make(map[string]types.NetworkCreate)
	for _, service := range bundle.Services {
		for _, networkName := range service.Networks {
			networks[networkName] = types.NetworkCreate{
				Labels: getStackLabels(namespace.name, nil),
			}
		}
	}

	services := make(map[string]swarm.ServiceSpec)
	for internalName, service := range bundle.Services {
		name := namespace.scope(internalName)

		var ports []swarm.PortConfig
		for _, portSpec := range service.Ports {
			ports = append(ports, swarm.PortConfig{
				Protocol:   swarm.PortConfigProtocol(portSpec.Protocol),
				TargetPort: portSpec.Port,
			})
		}

		nets := []swarm.NetworkAttachmentConfig{}
		for _, networkName := range service.Networks {
			nets = append(nets, swarm.NetworkAttachmentConfig{
				Target:  namespace.scope(networkName),
				Aliases: []string{networkName},
			})
		}

		serviceSpec := swarm.ServiceSpec{
			Annotations: swarm.Annotations{
				Name:   name,
				Labels: getStackLabels(namespace.name, service.Labels),
			},
			TaskTemplate: swarm.TaskSpec{
				ContainerSpec: swarm.ContainerSpec{
					Image:   service.Image,
					Command: service.Command,
					Args:    service.Args,
					Env:     service.Env,
					// Service Labels will not be copied to Containers
					// automatically during the deployment so we apply
					// it here.
					Labels: getStackLabels(namespace.name, nil),
				},
			},
			EndpointSpec: &swarm.EndpointSpec{
				Ports: ports,
			},
			Networks: nets,
		}

		services[internalName] = serviceSpec
	}

	if err := createNetworks(ctx, dockerCli, namespace, networks); err != nil {
		return err
	}
	return deployServices(ctx, dockerCli, services, namespace, opts.sendRegistryAuth)
}
