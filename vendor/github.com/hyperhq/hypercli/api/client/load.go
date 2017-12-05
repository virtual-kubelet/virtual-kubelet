package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/engine-api/types"
	Cli "github.com/hyperhq/hypercli/cli"
	"github.com/hyperhq/hypercli/image"
	"github.com/hyperhq/hypercli/pkg/archive"
	"github.com/hyperhq/hypercli/pkg/jsonmessage"
	flag "github.com/hyperhq/hypercli/pkg/mflag"
	"github.com/hyperhq/hypercli/pkg/progress"
	"github.com/hyperhq/hypercli/pkg/streamformatter"
	"github.com/hyperhq/hypercli/pkg/symlink"
	"golang.org/x/net/context"
)

const (
	unsupported = "Unsupported image version"
)

type manifestItem struct {
	Config   string
	RepoTags []string
	Layers   []string
}

type readCloser struct {
	io.Reader
	NeedClose io.ReadCloser
}

func (rc readCloser) Close() error {
	return rc.NeedClose.Close()
}

func safePath(base, path string) (string, error) {
	return symlink.FollowSymlinkInScope(filepath.Join(base, path), base)
}

func removeExistLayers(tmpDir string, existLayers []string, layerPaths []string) error {
	keepLayerPaths := make(map[string]bool)
	for _, _layerPath := range layerPaths[len(existLayers):] {
		layerPath := filepath.Join(tmpDir, _layerPath)
		info, err := os.Lstat(layerPath)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink == os.ModeSymlink {
			if _realPath, err := filepath.EvalSymlinks(layerPath); err == nil {
				realPath := filepath.Join(filepath.Base(filepath.Dir(_realPath)), "layer.tar")
				keepLayerPaths[realPath] = true
			}
		}
	}
	for idx := range existLayers {
		layerPath := layerPaths[idx]
		if _, ok := keepLayerPaths[layerPath]; !ok {
			if err := os.Remove(filepath.Join(tmpDir, layerPath)); err != nil {
				continue
			}
		}
	}
	return nil
}

func (cli *DockerCli) getExistLayers(ctx context.Context, tmpDir string) ([]string, []string, error) {
	manifestPath, err := safePath(tmpDir, "manifest.json")
	if err != nil {
		return nil, nil, err
	}

	manifestFile, err := os.Open(manifestPath)
	if err != nil {
		return nil, nil, err
	}
	defer manifestFile.Close()

	var manifest []manifestItem
	if err := json.NewDecoder(manifestFile).Decode(&manifest); err != nil {
		return nil, nil, err
	}

	allLayers := make([][]string, 0)
	repoTags := make([][]string, 0)
	layerPaths := make([]string, 0)
	for _, m := range manifest {
		configPath, err := safePath(tmpDir, m.Config)
		if err != nil {
			return nil, nil, err
		}
		config, err := ioutil.ReadFile(configPath)
		if err != nil {
			return nil, nil, err
		}
		img, err := image.NewFromJSON(config)
		if err != nil {
			return nil, nil, err
		}

		if expected, actual := len(m.Layers), len(img.RootFS.DiffIDs); expected != actual {
			return nil, nil, errors.New(unsupported)
		}

		layerPaths = append(layerPaths, m.Layers...)

		layers := make([]string, 0)

		for _, diffID := range img.RootFS.DiffIDs {
			layers = append(layers, string(diffID))
		}

		allLayers = append(allLayers, layers)
		repoTags = append(repoTags, m.RepoTags)
	}
	diffRet, err := cli.client.ImageDiff(ctx, allLayers, repoTags)
	if err != nil {
		return nil, nil, err
	}
	return diffRet.ExistLayers, layerPaths, nil
}

func (cli *DockerCli) ImageLoadFromTar(ctx context.Context, tr io.Reader, quiet bool) (*types.ImageLoadResponse, error) {
	tmpDir, err := ioutil.TempDir("", "hyper-pull-local-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	if err := archive.Untar(tr, tmpDir, &archive.TarOptions{NoLchown: true}); err != nil {
		return nil, err
	}

	if !quiet {
		fmt.Fprintln(cli.out, "Diffing local image with remote image...")
	}

	existLayers, layerPaths, err := cli.getExistLayers(ctx, tmpDir)
	if err != nil {
		return nil, err
	}

	if err := removeExistLayers(tmpDir, existLayers, layerPaths); err != nil {
		return nil, err
	}

	fs, err := archive.Tar(tmpDir, archive.Gzip)
	if err != nil {
		return nil, err
	}
	defer fs.Close()

	hasNewLayers := len(existLayers) != len(layerPaths)
	if hasNewLayers {
		if !quiet {
			fmt.Fprintln(cli.out, "Preparing to upload image...")
		}
	}

	tarTmpDir, err := ioutil.TempDir("", "hyper-pull-local-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tarTmpDir)

	tarPath := filepath.Join(tarTmpDir, "image.tar")

	tf, err := os.Create(tarPath)
	if err != nil {
		return nil, err
	}
	defer tf.Close()

	_, err = io.Copy(tf, fs)
	if err != nil {
		return nil, err
	}
	os.RemoveAll(tmpDir)

	info, err := tf.Stat()
	if err != nil {
		return nil, err
	}
	tf, err = os.Open(tarPath)
	if err != nil {
		return nil, err
	}

	resp, err := cli.client.ImageLoadLocal(ctx, quiet, info.Size())
	if err != nil {
		return nil, err
	}

	if !hasNewLayers || quiet {
		go func() {
			_, err := io.Copy(resp.Conn, tf)
			if err != nil {
				fmt.Fprintln(cli.out, err.Error())
				resp.Conn.Close()
				return
			}
			tf.Close()
		}()
		return &types.ImageLoadResponse{
			Body: resp.Conn,
			JSON: true,
		}, nil
	}

	pr, pw := io.Pipe()
	progressOutput := streamformatter.NewJSONStreamFormatter().NewProgressOutput(pw, false)
	progressReader := progress.NewProgressReader(tf, progressOutput, info.Size(), "", "Uploading image")

	go func() {
		_, err := io.Copy(resp.Conn, progressReader)
		if err != nil {
			fmt.Fprintln(cli.out, err.Error())
			resp.Conn.Close()
			return
		}
		pw.CloseWithError(io.EOF)
	}()

	return &types.ImageLoadResponse{
		Body: readCloser{io.MultiReader(pr, resp.Conn), resp.Conn},
		JSON: true,
	}, nil
}

// ImageDiff diff an image layers with local and imaged
func (cli *DockerCli) ImageLoadFromDaemon(ctx context.Context, name string, quiet bool) (*types.ImageLoadResponse, error) {
	if !quiet {
		fmt.Fprintln(cli.out, "Loading image from local docker daemon...")
	}

	tr, err := cli.client.ImageSaveTarFromDaemon(ctx, []string{name})
	if err != nil {
		return nil, err
	}
	defer tr.Close()

	return cli.ImageLoadFromTar(ctx, tr, quiet)
}

// CmdLoad load a local image or a tar file
//
// The tar archive is read from STDIN by default, or from a tar archive file.
//
// Usage: docker load [OPTIONS]
func (cli *DockerCli) CmdLoad(args ...string) error {
	cmd := Cli.Subcmd("load", nil, "Load a local image or a tar file", true)
	local := cmd.String([]string{"l", "-local"}, "", "Read from a local image")
	infile := cmd.String([]string{"i", "-input"}, "", "Read from a local or remote archive file compressed with gzip, bzip, or xz, instead of STDIN")
	quiet := cmd.Bool([]string{"q", "-quiet"}, false, "Do not show load process")
	cmd.Require(flag.Exact, 0)
	cmd.ParseFlags(args, true)

	*infile = strings.TrimSpace(*infile)
	*local = strings.TrimSpace(*local)

	var stdin io.Reader = cli.in

	if *infile == "" && *local == "" && stdin == nil {
		return errors.New("source image must be specified via --input, --local or STDIN")
	}

	var response *types.ImageLoadResponse
	var err error

	if *local != "" {
		// Load from local docker daemon
		response, err = cli.ImageLoadFromDaemon(context.Background(), *local, *quiet)
	} else if *infile != "" {
		if strings.HasPrefix(*infile, "http://") ||
			strings.HasPrefix(*infile, "https://") ||
			strings.HasPrefix(*infile, "ftp://") {
			var input struct {
				FromSrc string `json:"fromSrc"`
				Quiet   bool   `json:"quiet"`
			}
			input.FromSrc = *infile
			input.Quiet = *quiet
			// Load from remote URL
			response, err = cli.client.ImageLoad(context.Background(), input)
		} else {
			// Load from local tar
			var af *os.File
			af, err = os.Open(*infile)
			if err != nil {
				return err
			}
			defer af.Close()
			response, err = cli.ImageLoadFromTar(context.Background(), af, *quiet)
		}
	} else if stdin != nil {
		// Load from STDIN
		response, err = cli.ImageLoadFromTar(context.Background(), stdin, *quiet)
	}

	if err != nil {
		return err
	}

	if response == nil {
		return nil
	}

	defer response.Body.Close()

	if response.JSON {
		return jsonmessage.DisplayJSONMessagesStream(response.Body, cli.out, cli.outFd, cli.isTerminalOut, nil)
	}

	_, err = io.Copy(cli.out, response.Body)
	return err
}
