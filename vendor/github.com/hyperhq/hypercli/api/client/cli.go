package client

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"runtime"

	"github.com/docker/engine-api/client"
	"github.com/docker/go-connections/sockets"
	"github.com/docker/go-connections/tlsconfig"
	"github.com/hyperhq/hypercli/api"
	"github.com/hyperhq/hypercli/cli"
	"github.com/hyperhq/hypercli/cliconfig"
	"github.com/hyperhq/hypercli/dockerversion"
	"github.com/hyperhq/hypercli/opts"
	"github.com/hyperhq/hypercli/pkg/term"
)

// DockerCli represents the docker command line client.
// Instances of the client can be returned from NewDockerCli.
type DockerCli struct {
	// initializing closure
	init func() error

	// configFile has the client configuration file
	configFile *cliconfig.ConfigFile
	// in holds the input stream and closer (io.ReadCloser) for the client.
	in io.ReadCloser
	// out holds the output stream (io.Writer) for the client.
	out io.Writer
	// err holds the error stream (io.Writer) for the client.
	err io.Writer
	// keyFile holds the key file as a string.
	keyFile string
	// inFd holds the file descriptor of the client's STDIN (if valid).
	inFd uintptr
	// outFd holds file descriptor of the client's STDOUT (if valid).
	outFd uintptr
	// isTerminalIn indicates whether the client's STDIN is a TTY
	isTerminalIn bool
	// isTerminalOut indicates whether the client's STDOUT is a TTY
	isTerminalOut bool
	// client is the http client that performs all API operations
	client client.APIClient
	// state holds the terminal state
	state  *term.State
	region string
	host   string
}

// Initialize calls the init function that will setup the configuration for the client
// such as the TLS, tcp and other parameters used to run the client.
func (cli *DockerCli) Initialize() error {
	if cli.init == nil {
		return nil
	}
	return cli.init()
}

// CheckTtyInput checks if we are trying to attach to a container tty
// from a non-tty client input stream, and if so, returns an error.
func (cli *DockerCli) CheckTtyInput(attachStdin, ttyMode bool) error {
	// In order to attach to a container tty, input stream for the client must
	// be a tty itself: redirecting or piping the client standard input is
	// incompatible with `docker run -t`, `docker exec -t` or `docker attach`.
	if ttyMode && attachStdin && !cli.isTerminalIn {
		return errors.New("cannot enable tty mode on non tty input")
	}
	return nil
}

// PsFormat returns the format string specified in the configuration.
// String contains columns and format specification, for example {{ID}}\t{{Name}}.
func (cli *DockerCli) PsFormat() string {
	return cli.configFile.PsFormat
}

// ImagesFormat returns the format string specified in the configuration.
// String contains columns and format specification, for example {{ID}}\t{{Name}}.
func (cli *DockerCli) ImagesFormat() string {
	return cli.configFile.ImagesFormat
}

// VolumesFormat returns the format string specified in the configuration.
// String contains columns and format specification, for example {{ID}}\t{{Name}}.
func (cli *DockerCli) VolumesFormat() string {
	return cli.configFile.VolumesFormat
}

func (cli *DockerCli) setRawTerminal() error {
	if cli.isTerminalIn && os.Getenv("NORAW") == "" {
		state, err := term.SetRawTerminal(cli.inFd)
		if err != nil {
			return err
		}
		cli.state = state
	}
	return nil
}

func (cli *DockerCli) restoreTerminal(in io.Closer) error {
	if cli.state != nil {
		term.RestoreTerminal(cli.inFd, cli.state)
	}
	// WARNING: DO NOT REMOVE THE OS CHECK !!!
	// For some reason this Close call blocks on darwin..
	// As the client exists right after, simply discard the close
	// until we find a better solution.
	if in != nil && runtime.GOOS != "darwin" {
		return in.Close()
	}
	return nil
}

// NewDockerCli returns a DockerCli instance with IO output and error streams set by in, out and err.
// The key file, protocol (i.e. unix) and address are passed in as strings, along with the tls.Config. If the tls.Config
// is set the client scheme will be set to https.
// The client will be given a 32-second timeout (see https://github.com/hyperhq/hypercli/pull/8035).
func NewDockerCli(in io.ReadCloser, out, err io.Writer, clientFlags *cli.ClientFlags) *DockerCli {
	cli := &DockerCli{
		in:      in,
		out:     out,
		err:     err,
		keyFile: clientFlags.Common.TrustKey,
	}

	cli.init = func() error {
		clientFlags.PostParse()
		configFile, e := cliconfig.Load(cliconfig.ConfigDir())
		if e != nil {
			fmt.Fprintf(cli.err, "WARNING: Error loading config file:%v\n", e)
		}
		cli.configFile = configFile

		host, dft, err := cli.getServerHost(clientFlags.Common.Region, clientFlags.Common.TLSOptions)
		if err != nil {
			return err
		}

		customHeaders := cli.configFile.HTTPHeaders
		if customHeaders == nil {
			customHeaders = map[string]string{}
		}
		customHeaders["User-Agent"] = "Docker-Client/" + dockerversion.Version + " (" + runtime.GOOS + ")"

		verStr := api.DefaultVersion.String()
		if tmpStr := os.Getenv("HYPER_API_VERSION"); tmpStr != "" {
			verStr = tmpStr
		}

		httpClient, err := newHTTPClient(host, clientFlags.Common.TLSOptions)
		if err != nil {
			return err
		}
		var cloudConfig cliconfig.CloudConfig
		cc, ok := configFile.CloudConfig[cliconfig.DefaultHyperFormat]
		if ok && dft {
			cloudConfig.AccessKey = cc.AccessKey
			cloudConfig.SecretKey = cc.SecretKey
		} else {
			cc, ok = configFile.CloudConfig[host]
			if ok {
				cloudConfig.AccessKey = cc.AccessKey
				cloudConfig.SecretKey = cc.SecretKey
			} else {
				cloudConfig.AccessKey = os.Getenv("HYPER_ACCESS")
				cloudConfig.SecretKey = os.Getenv("HYPER_SECRET")
			}
		}
		if cloudConfig.AccessKey == "" || cloudConfig.SecretKey == "" {
			fmt.Fprintf(cli.err, "WARNING: null cloud config\n")
		}
		cli.region = clientFlags.Common.Region
		if cli.region == "" {
			if cli.region = cc.Region; cli.region == "" {
				cli.region = cli.getDefaultRegion()
			}
		}
		if !dft {
			if cli.region = cc.Region; cli.region == "" {
				cli.region = cliconfig.DefaultHyperRegion
			}
		}

		client, err := client.NewClient(host, verStr, httpClient, customHeaders, cloudConfig.AccessKey, cloudConfig.SecretKey, cli.region)
		if err != nil {
			return err
		}
		cli.client = client
		cli.host = host
		if cli.in != nil {
			cli.inFd, cli.isTerminalIn = term.GetFdInfo(cli.in)
		}
		if cli.out != nil {
			cli.outFd, cli.isTerminalOut = term.GetFdInfo(cli.out)
		}

		return nil
	}

	return cli
}

func (cli *DockerCli) getServerHost(region string, tlsOptions *tlsconfig.Options) (host string, dft bool, err error) {
	dft = false
	host = region
	if host == "" {
		host = os.Getenv("HYPER_DEFAULT_REGION")
		region = cli.getDefaultRegion()
	}
	if _, err := url.ParseRequestURI(host); err != nil {
		host = "tcp://" + region + "." + cliconfig.DefaultHyperEndpoint
		dft = true
	}

	host, err = opts.ParseHost(tlsOptions != nil, host)
	return
}

func newHTTPClient(host string, tlsOptions *tlsconfig.Options) (*http.Client, error) {
	if tlsOptions == nil {
		// let the api client configure the default transport.
		return nil, nil
	}

	config, err := tlsconfig.Client(*tlsOptions)
	if err != nil {
		return nil, err
	}
	tr := &http.Transport{
		TLSClientConfig: config,
	}
	proto, addr, _, err := client.ParseHost(host)
	if err != nil {
		return nil, err
	}

	sockets.ConfigureTransport(tr, proto, addr)

	return &http.Client{
		Transport: tr,
	}, nil
}
