package cli

import flag "github.com/hyperhq/hypercli/pkg/mflag"

// ClientFlags represents flags for the docker client.
type ClientFlags struct {
	FlagSet   *flag.FlagSet
	Common    *CommonFlags
	PostParse func()

	ConfigDir string
}
