package config

import "testing"

type NullLookup struct {
}

func (n *NullLookup) Lookup(file, relativeTo string) ([]byte, string, error) {
	return nil, "", nil
}

func (n *NullLookup) ResolvePath(path, inFile string) string {
	return ""
}

func TestExtendsInheritImage(t *testing.T) {
	config, err := MergeServices(NewConfigs(), nil, &NullLookup{}, "", []byte(`
parent:
  image: foo
child:
  extends:
    service: parent
`))

	if err != nil {
		t.Fatal(err)
	}

	parent := config["parent"]
	child := config["child"]

	if parent.Image != "foo" {
		t.Fatal("Invalid image", parent.Image)
	}

	if child.Build != "" {
		t.Fatal("Invalid build", child.Build)
	}

	if child.Image != "foo" {
		t.Fatal("Invalid image", child.Image)
	}
}

func TestExtendsInheritBuild(t *testing.T) {
	config, err := MergeServices(NewConfigs(), nil, &NullLookup{}, "", []byte(`
parent:
  build: .
child:
  extends:
    service: parent
`))

	if err != nil {
		t.Fatal(err)
	}

	parent := config["parent"]
	child := config["child"]

	if parent.Build != "." {
		t.Fatal("Invalid build", parent.Build)
	}

	if child.Build != "." {
		t.Fatal("Invalid build", child.Build)
	}

	if child.Image != "" {
		t.Fatal("Invalid image", child.Image)
	}
}

func TestExtendBuildOverImage(t *testing.T) {
	config, err := MergeServices(NewConfigs(), nil, &NullLookup{}, "", []byte(`
parent:
  image: foo
child:
  build: .
  extends:
    service: parent
`))

	if err != nil {
		t.Fatal(err)
	}

	parent := config["parent"]
	child := config["child"]

	if parent.Image != "foo" {
		t.Fatal("Invalid image", parent.Image)
	}

	if child.Build != "." {
		t.Fatal("Invalid build", child.Build)
	}

	if child.Image != "" {
		t.Fatal("Invalid image", child.Image)
	}
}

func TestExtendImageOverBuild(t *testing.T) {
	config, err := MergeServices(NewConfigs(), nil, &NullLookup{}, "", []byte(`
parent:
  build: .
child:
  image: foo
  extends:
    service: parent
`))

	if err != nil {
		t.Fatal(err)
	}

	parent := config["parent"]
	child := config["child"]

	if parent.Image != "" {
		t.Fatal("Invalid image", parent.Image)
	}

	if parent.Build != "." {
		t.Fatal("Invalid build", parent.Build)
	}

	if child.Build != "" {
		t.Fatal("Invalid build", child.Build)
	}

	if child.Image != "foo" {
		t.Fatal("Invalid image", child.Image)
	}
}

func TestRestartNo(t *testing.T) {
	config, err := MergeServices(NewConfigs(), nil, &NullLookup{}, "", []byte(`
test:
  restart: "no"
  image: foo
`))

	if err != nil {
		t.Fatal(err)
	}

	test := config["test"]

	if test.Restart != "no" {
		t.Fatal("Invalid restart policy", test.Restart)
	}
}

func TestRestartAlways(t *testing.T) {
	config, err := MergeServices(NewConfigs(), nil, &NullLookup{}, "", []byte(`
test:
  restart: always
  image: foo
`))

	if err != nil {
		t.Fatal(err)
	}

	test := config["test"]

	if test.Restart != "always" {
		t.Fatal("Invalid restart policy", test.Restart)
	}
}
