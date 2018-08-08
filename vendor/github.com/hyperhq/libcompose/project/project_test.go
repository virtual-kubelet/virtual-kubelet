package project

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/hyperhq/libcompose/config"
	"github.com/hyperhq/libcompose/project/options"
	"github.com/hyperhq/libcompose/yaml"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
)

type TestServiceFactory struct {
	Counts map[string]int
}

type TestService struct {
	factory *TestServiceFactory
	name    string
	config  *config.ServiceConfig
	EmptyService
	Count int
}

func (t *TestService) Config() *config.ServiceConfig {
	return t.config
}

func (t *TestService) Name() string {
	return t.name
}

func (t *TestService) Run(ctx context.Context, commandParts []string) (int, error) {
	return 0, nil
}

func (t *TestService) Create(options options.Create) error {
	key := t.name + ".create"
	t.factory.Counts[key] = t.factory.Counts[key] + 1
	return nil
}

func (t *TestService) DependentServices() []ServiceRelationship {
	return nil
}

func (t *TestServiceFactory) Create(project *Project, name string, serviceConfig *config.ServiceConfig) (Service, error) {
	return &TestService{
		factory: t,
		config:  serviceConfig,
		name:    name,
	}, nil
}

func TestTwoCall(t *testing.T) {
	factory := &TestServiceFactory{
		Counts: map[string]int{},
	}

	p := NewProject(nil, &Context{
		ServiceFactory: factory,
	})
	p.ServiceConfigs = config.NewServiceConfigs()
	p.ServiceConfigs.Add("foo", &config.ServiceConfig{})

	if err := p.Create(options.Create{}, "foo"); err != nil {
		t.Fatal(err)
	}

	if err := p.Create(options.Create{}, "foo"); err != nil {
		t.Fatal(err)
	}

	if factory.Counts["foo.create"] != 2 {
		t.Fatal("Failed to create twice")
	}
}

func TestParseWithBadContent(t *testing.T) {
	p := NewProject(nil, &Context{
		ComposeBytes: [][]byte{
			[]byte("garbage"),
		},
	})

	err := p.Parse()
	if err == nil {
		t.Fatal("Should have failed parse")
	}

	if !strings.HasPrefix(err.Error(), "Invalid timestamp: 'garbage'") {
		t.Fatalf("Should have failed parse: %#v", err)
	}
}

func TestParseWithGoodContent(t *testing.T) {
	p := NewProject(nil, &Context{
		ComposeBytes: [][]byte{
			[]byte("not-garbage:\n  image: foo"),
		},
	})

	err := p.Parse()
	if err != nil {
		t.Fatal(err)
	}
}

type TestEnvironmentLookup struct {
}

func (t *TestEnvironmentLookup) Lookup(key, serviceName string, config *config.ServiceConfig) []string {
	return []string{fmt.Sprintf("%s=X", key)}
}

func TestEnvironmentResolve(t *testing.T) {
	factory := &TestServiceFactory{
		Counts: map[string]int{},
	}

	p := NewProject(nil, &Context{
		ServiceFactory:    factory,
		EnvironmentLookup: &TestEnvironmentLookup{},
	})
	p.ServiceConfigs = config.NewServiceConfigs()
	p.ServiceConfigs.Add("foo", &config.ServiceConfig{
		Environment: yaml.MaporEqualSlice([]string{
			"A",
			"A=",
			"A=B",
		}),
	})

	service, err := p.CreateService("foo")
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(service.Config().Environment, yaml.MaporEqualSlice{"A=X", "A=X", "A=B"}) {
		t.Fatal("Invalid environment", service.Config().Environment)
	}
}

func TestParseWithMultipleComposeFiles(t *testing.T) {
	/*
			configOne := []byte(`
		  multiple:
		    image: tianon/true
		    ports:
		      - 8000`)

			configTwo := []byte(`
		  multiple:
		    image: busybox
		    container_name: multi
		    ports:
		      - 9000`)

			configThree := []byte(`
		  multiple:
		    image: busybox
		    mem_limit: 40000000
		    ports:
		      - 10000`)
	*/
	configOne := []byte(`
  multiple:
    image: tianon/true`)

	configTwo := []byte(`
  multiple:
    image: busybox
    container_name: multi`)

	configThree := []byte(`
  multiple:
    image: busybox
    size: xxs`)

	p := NewProject(nil, &Context{
		ComposeBytes: [][]byte{configOne, configTwo},
	})

	err := p.Parse()

	assert.Nil(t, err)

	multipleConfig, _ := p.ServiceConfigs.Get("multiple")
	assert.Equal(t, "busybox", multipleConfig.Image)
	assert.Equal(t, "multi", multipleConfig.ContainerName)
	//assert.Equal(t, []string{"8000", "9000"}, multipleConfig.Ports)

	p = NewProject(nil, &Context{
		ComposeBytes: [][]byte{configTwo, configOne},
	})

	err = p.Parse()

	assert.Nil(t, err)

	multipleConfig, _ = p.ServiceConfigs.Get("multiple")
	assert.Equal(t, "tianon/true", multipleConfig.Image)
	assert.Equal(t, "multi", multipleConfig.ContainerName)
	//assert.Equal(t, []string{"9000", "8000"}, multipleConfig.Ports)

	p = NewProject(nil, &Context{
		ComposeBytes: [][]byte{configOne, configTwo, configThree},
	})

	err = p.Parse()

	assert.Nil(t, err)

	multipleConfig, _ = p.ServiceConfigs.Get("multiple")
	assert.Equal(t, "busybox", multipleConfig.Image)
	assert.Equal(t, "multi", multipleConfig.ContainerName)
	//assert.Equal(t, []string{"8000", "9000", "10000"}, multipleConfig.Ports)
	//assert.Equal(t, int64(40000000), multipleConfig.MemLimit)
}
