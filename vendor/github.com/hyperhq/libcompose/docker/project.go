package docker

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Sirupsen/logrus"
	"github.com/hyperhq/libcompose/config"
	"github.com/hyperhq/libcompose/lookup"
	"github.com/hyperhq/libcompose/project"
)

// ComposeVersion is name of docker-compose.yml file syntax supported version
const ComposeVersion = "1.5.0"

// NewProject creates a Project with the specified context.
func NewProject(context *Context) (project.APIProject, error) {
	if context.ResourceLookup == nil {
		context.ResourceLookup = &lookup.FileConfigLookup{}
	}

	if context.EnvironmentLookup == nil {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		context.EnvironmentLookup = &lookup.ComposableEnvLookup{
			Lookups: []config.EnvironmentLookup{
				&lookup.EnvfileLookup{
					Path: filepath.Join(cwd, ".env"),
				},
				&lookup.OsEnvLookup{},
			},
		}
	}

	if context.AuthLookup == nil {
		context.AuthLookup = &ConfigAuthLookup{context}
	}

	if context.ServiceFactory == nil {
		context.ServiceFactory = &ServiceFactory{
			context: context,
		}
	}

	if context.ClientFactory == nil {
		return nil, fmt.Errorf("please provide the client to operate the Hyper.sh")
	}

	// FIXME(vdemeester) Remove the context duplication ?
	p := project.NewProject(context.ClientFactory, &context.Context)

	err := p.Parse()
	if err != nil {
		return nil, err
	}

	if err = context.open(); err != nil {
		logrus.Errorf("Failed to open project %s: %v", p.Name, err)
		return nil, err
	}

	return p, err
}
