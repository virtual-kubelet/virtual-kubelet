package client

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	distreference "github.com/docker/distribution/reference"
	"github.com/hyperhq/hyper-api/types"
	"github.com/hyperhq/hyper-api/types/reference"
)

// ImageTag tags an image in the docker host
func (cli *Client) ImageTag(ctx context.Context, imageID, ref string, options types.ImageTagOptions) error {
	distributionRef, err := distreference.ParseNamed(ref)
	if err != nil {
		return fmt.Errorf("Error parsing reference: %q is not a valid repository/tag", ref)
	}

	if _, isCanonical := distributionRef.(distreference.Canonical); isCanonical {
		return errors.New("refusing to create a tag with a digest reference")
	}

	tag := reference.GetTagFromNamedRef(distributionRef)

	query := url.Values{}
	query.Set("repo", distributionRef.Name())
	query.Set("tag", tag)
	if options.Force {
		query.Set("force", "1")
	}

	resp, err := cli.post(ctx, "/images/"+imageID+"/tag", query, nil, nil)
	ensureReaderClosed(resp)
	return err
}
