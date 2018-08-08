package main

import (
	"path/filepath"

	"github.com/hyperhq/hypercli/cli"
	"github.com/hyperhq/hypercli/cliconfig"
	flag "github.com/hyperhq/hypercli/pkg/mflag"
	"github.com/hyperhq/hypercli/utils"
)

var clientFlags = &cli.ClientFlags{FlagSet: new(flag.FlagSet), Common: commonFlags}

func init() {
	client := clientFlags.FlagSet
	client.StringVar(&clientFlags.ConfigDir, []string{"-config"}, cliconfig.ConfigDir(), "Location of client config files")

	clientFlags.PostParse = func() {
		clientFlags.Common.PostParse()

		if clientFlags.ConfigDir != "" {
			cliconfig.SetConfigDir(clientFlags.ConfigDir)
		}

		if clientFlags.Common.TrustKey == "" {
			clientFlags.Common.TrustKey = filepath.Join(cliconfig.ConfigDir(), defaultTrustKeyFile)
		}

		if clientFlags.Common.Debug {
			utils.EnableDebug()
		}
	}
}
