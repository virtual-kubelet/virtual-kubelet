package exec

import (
	"testing"
	"k8s.io/api/core/v1"
	"strings"
)

var p = ExecProvider{}

var containerTests = []struct {
	command  	[]string
	args 		[]string
	pathSuffix 	string
	argsLength	int
}{
	{[]string{"bash", "-c"}, []string{"echo 1"}, "/bash", 3},
	{[]string{"bash"}, []string{"/tmp/some-script.sh", "arg1", "arg2"}, "/bash", 4},
	{[]string{"echo", "1"}, []string{}, "/echo", 2},
	{[]string{"/tmp/some-binary"}, []string{}, "/tmp/some-binary", 1},

}

func TestCreateCommand(t *testing.T) {
	for _, ct := range containerTests {
		container := v1.Container{
			Command: ct.command,
			Args:    ct.args,
		}
		cmd := p.createCommand(container)

		if ! strings.HasSuffix(cmd.Path, ct.pathSuffix) {
			t.Errorf("Expecting cmd.Path to be resolved and end with %s, received: %s\n", ct.pathSuffix, cmd.Path)
		}
		if len(cmd.Args) != ct.argsLength {
			t.Errorf("Expecting cmd.Args to contains %d entry, received: %v\n", ct.argsLength, cmd.Args)
		}
	}
}
