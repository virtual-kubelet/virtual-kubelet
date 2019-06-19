package provider

const (
	// OperatingSystemLinux is the configuration value for defining Linux.
	OperatingSystemLinux = "Linux"
	// OperatingSystemWindows is the configuration value for defining Windows.
	OperatingSystemWindows = "Windows"
)

type OperatingSystems map[string]bool

var (
	// ValidOperatingSystems defines the group of operating systems
	// that can be used as a kubelet node.
	ValidOperatingSystems = OperatingSystems{
		OperatingSystemLinux:   true,
		OperatingSystemWindows: true,
	}
)

func (o OperatingSystems) Names() []string {
	keys := make([]string, 0, len(o))
	for k := range o {
		keys = append(keys, k)
	}
	return keys
}
