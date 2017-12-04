package client

import (
	"errors"
	"fmt"

	"golang.org/x/net/context"

	"github.com/docker/engine-api/types"
	Cli "github.com/hyperhq/hypercli/cli"
	"github.com/hyperhq/hypercli/pkg/jsonmessage"
	flag "github.com/hyperhq/hypercli/pkg/mflag"
	"github.com/hyperhq/hypercli/reference"
	"github.com/hyperhq/hypercli/registry"
)

// CmdPull pulls an image or a repository from the registry.
//
// Usage: docker pull [OPTIONS] IMAGENAME[:TAG|@DIGEST]
func (cli *DockerCli) CmdPull(args ...string) error {
	cmd := Cli.Subcmd("pull", []string{"NAME[:TAG|@DIGEST]"}, Cli.DockerCommands["pull"].Description, true)
	allTags := cmd.Bool([]string{}, false, "Download all tagged images in the repository")
	addTrustedFlags(cmd, true)
	cmd.Require(flag.Exact, 1)

	cmd.ParseFlags(args, true)
	remote := cmd.Arg(0)

	distributionRef, err := reference.ParseNamed(remote)
	if err != nil {
		return err
	}
	if *allTags && !reference.IsNameOnly(distributionRef) {
		return errors.New("tag can't be used with --all-tags/-a")
	}

	if err = cli.checkCloudConfig(); err != nil {
		return err
	}

	if !*allTags && reference.IsNameOnly(distributionRef) {
		distributionRef = reference.WithDefaultTag(distributionRef)
		fmt.Fprintf(cli.out, "Using default tag: %s\n", reference.DefaultTag)
	}

	var tag string
	switch x := distributionRef.(type) {
	case reference.Canonical:
		tag = x.Digest().String()
	case reference.NamedTagged:
		tag = x.Tag()
	}

	ref := registry.ParseReference(tag)

	// Resolve the Repository name from fqn to RepositoryInfo
	repoInfo, err := registry.ParseRepositoryInfo(distributionRef)
	if err != nil {
		return err
	}

	ctx := context.Background()

	authConfig := cli.resolveAuthConfig(ctx, cli.configFile.AuthConfigs, repoInfo.Index)
	requestPrivilege := cli.registryAuthenticationPrivilegedFunc(repoInfo.Index, "pull")

	if isTrusted() && !ref.HasDigest() {
		// Check if tag is digest
		return cli.trustedPull(ctx, repoInfo, ref, authConfig, requestPrivilege)
	}

	return cli.imagePullPrivileged(ctx, authConfig, distributionRef.String(), requestPrivilege, *allTags)
}

func (cli *DockerCli) imagePullPrivileged(ctx context.Context, authConfig types.AuthConfig, ref string, requestPrivilege types.RequestPrivilegeFunc, all bool) error {

	encodedAuth, err := encodeAuthToBase64(authConfig)
	if err != nil {
		return err
	}
	options := types.ImagePullOptions{
		PrivilegeFunc: requestPrivilege,
		RegistryAuth:  encodedAuth,
		All:           all,
	}

	responseBody, err := cli.client.ImagePull(context.Background(), ref, options)
	if err != nil {
		return err
	}
	defer responseBody.Close()

	return jsonmessage.DisplayJSONMessagesStream(responseBody, cli.out, cli.outFd, cli.isTerminalOut, nil)
}
