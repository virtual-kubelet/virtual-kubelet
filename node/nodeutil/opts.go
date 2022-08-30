package nodeutil

import (
	"fmt"
	"net/http"

	"github.com/virtual-kubelet/virtual-kubelet/errdefs"
	"github.com/virtual-kubelet/virtual-kubelet/node/api"
	"k8s.io/apiserver/pkg/server/dynamiccertificates"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// WithNodeConfig returns a NodeOpt which replaces the NodeConfig with the passed in value.
func WithNodeConfig(c NodeConfig) NodeOpt {
	return func(orig *NodeConfig) error {
		*orig = c
		return nil
	}
}

// WithClient return a NodeOpt that sets the client that will be used to create/manage the node.
func WithClient(c kubernetes.Interface) NodeOpt {
	return func(cfg *NodeConfig) error {
		cfg.Client = c
		return nil
	}
}

// BootstrapConfig is the configuration for bootstrapping a node.
type BootstrapConfig struct {
	// ClientCAConfigMapSpec is the config map information for getting the CA for authenticating clients
	// If not set, the CA in the rest config will be used.
	ClientCAConfigMapSpec *ConfigMapCASpec
	// A set of options to pass when creating the webhook auth.
	WebhookAuthOpts []WebhookAuthOption
	// RestConfig is the rest config to use for the client
	// If it is not provided a default one from the environment will be created.
	RestConfig *rest.Config
}

// ConfigMapCASpec is the spec for a config map that contains a CA.
type ConfigMapCASpec struct {
	Namespace string
	Name      string
	Key       string
}

// BootstrapOpt is a functional option used to configure node bootstrapping
type BootstrapOpt func(*BootstrapConfig) error

// WithBootstrapFromRestConfig takes a reset config (or a default one from the environment) and returns a NodeOpt that will:
// 1. Create a client from the rest config (if no client is set)
// 2. Create webhook authn/authz from the rest config
// 3. Configure the TLS config for the HTTP server from the certs in the rest config.
func WithBootstrapFromRestConfig(opts ...BootstrapOpt) NodeOpt {
	return func(cfg *NodeConfig) error {
		var bOpts BootstrapConfig
		if rCfg, err := RestConfigFromEnv(cfg.KubeconfigPath); err == nil {
			bOpts.RestConfig = rCfg
		}

		for _, o := range opts {
			if err := o(&bOpts); err != nil {
				return err
			}
		}

		if bOpts.RestConfig == nil {
			return fmt.Errorf("no rest config provided")
		}

		if cfg.Client == nil {
			client, err := kubernetes.NewForConfig(bOpts.RestConfig)
			if err != nil {
				return err
			}
			cfg.Client = client
		}

		if err := WithTLSConfig(tlsFromRestConfig(bOpts.RestConfig))(cfg); err != nil {
			return err
		}

		if err := configureWebhookCA(cfg, &bOpts); err != nil {
			return fmt.Errorf("error configure webhook auth: %w", err)
		}

		var mux *http.ServeMux
		if cfg.Handler == nil {
			mux = http.NewServeMux()
			cfg.Handler = mux
		}
		if cfg.routeAttacher == nil && mux != nil {
			if err := AttachProviderRoutes(mux)(cfg); err != nil {
				return err
			}
		}

		auth, err := WebhookAuth(cfg.Client, cfg.NodeSpec.Name, bOpts.WebhookAuthOpts...)
		if err != nil {
			return err
		}

		cfg.Handler = api.InstrumentHandler(WithAuth(auth, cfg.Handler))
		return nil
	}
}

func configureWebhookCA(cfg *NodeConfig, bCfg *BootstrapConfig) error {
	if bCfg.ClientCAConfigMapSpec != nil {
		bCfg.WebhookAuthOpts = append(bCfg.WebhookAuthOpts, func(auth *WebhookAuthConfig) error {
			cmCA, err := dynamiccertificates.NewDynamicCAFromConfigMapController("client-ca", bCfg.ClientCAConfigMapSpec.Namespace, bCfg.ClientCAConfigMapSpec.Name, bCfg.ClientCAConfigMapSpec.Key, cfg.Client)
			if err != nil {
				return fmt.Errorf("error loading dynamic CA from config map: %w", err)
			}
			auth.AuthnConfig.ClientCertificateCAContentProvider = cmCA
			cfg.caController = cmCA
			return nil
		})
		return nil
	}

	if bCfg.RestConfig.CAFile != "" {
		bCfg.WebhookAuthOpts = append(bCfg.WebhookAuthOpts, func(auth *WebhookAuthConfig) error {
			caFile, err := dynamiccertificates.NewDynamicCAContentFromFile("ca-file", bCfg.RestConfig.CAFile)
			if err != nil {
				return fmt.Errorf("error loading dynamic CA file from rest config: %w", err)
			}
			auth.AuthnConfig.ClientCertificateCAContentProvider = caFile
			cfg.caController = caFile
			return nil
		})
		return nil
	}

	if bCfg.RestConfig.CAData != nil {
		bCfg.WebhookAuthOpts = append(bCfg.WebhookAuthOpts, func(auth *WebhookAuthConfig) error {
			caData, err := dynamiccertificates.NewStaticCAContent("ca-data", bCfg.RestConfig.CAData)
			if err != nil {
				return fmt.Errorf("error loading static ca from rest config: %w", err)
			}
			auth.AuthnConfig.ClientCertificateCAContentProvider = caData
			return nil
		})
		return nil
	}

	return errdefs.InvalidInput("no client CA found")
}
