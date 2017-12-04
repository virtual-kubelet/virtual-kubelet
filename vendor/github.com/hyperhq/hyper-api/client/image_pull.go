package client

import (
	"io"
	"net/http"
	"net/url"

	"context"

	"github.com/hyperhq/hyper-api/types"
	"github.com/hyperhq/hyper-api/types/reference"
)

// ImagePull requests the docker host to pull an image from a remote registry.
// It executes the privileged function if the operation is unauthorized
// and it tries one more time.
// It's up to the caller to handle the io.ReadCloser and close it properly.
//
// FIXME(vdemeester): there is currently used in a few way in docker/docker
// - if not in trusted content, ref is used to pass the whole reference, and tag is empty
// - if in trusted content, ref is used to pass the reference name, and tag for the digest
func (cli *Client) ImagePull(ctx context.Context, ref string, options types.ImagePullOptions) (io.ReadCloser, error) {
	repository, tag, err := reference.Parse(ref)
	if err != nil {
		return nil, err
	}

	query := url.Values{}
	query.Set("fromImage", repository)
	if tag != "" && !options.All {
		query.Set("tag", tag)
	}

	resp, err := cli.tryImageCreate(ctx, query, options.RegistryAuth)
	if resp.statusCode == http.StatusProxyAuthRequired {
		newAuthHeader, privilegeErr := options.PrivilegeFunc()
		if privilegeErr != nil {
			return nil, privilegeErr
		}
		resp, err = cli.tryImageCreate(ctx, query, newAuthHeader)
	}
	if err != nil {
		return nil, err
	}
	return resp.body, nil
}
