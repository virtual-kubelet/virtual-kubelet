package nodeutil

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
)

// WithTLSConfig returns a NodeOpt which creates a base TLSConfig with the default cipher suites and tls min verions.
// The tls config can be modified through functional options.
func WithTLSConfig(opts ...func(*tls.Config) error) NodeOpt {
	return func(cfg *NodeConfig) error {
		tlsCfg := &tls.Config{
			MinVersion:               tls.VersionTLS12,
			PreferServerCipherSuites: true,
			CipherSuites:             DefaultServerCiphers(),
			ClientAuth:               tls.RequestClientCert,
		}
		for _, o := range opts {
			if err := o(tlsCfg); err != nil {
				return err
			}
		}

		cfg.TLSConfig = tlsCfg
		return nil
	}
}

// WithCAFromPath makes a TLS config option to set up client auth using the path to a PEM encoded CA cert.
func WithCAFromPath(p string) func(*tls.Config) error {
	return func(cfg *tls.Config) error {
		pem, err := os.ReadFile(p)
		if err != nil {
			return fmt.Errorf("error reading ca cert pem: %w", err)
		}
		cfg.ClientAuth = tls.RequireAndVerifyClientCert
		return WithCACert(pem)(cfg)
	}
}

// WithKeyPairFromPath make sa TLS config option which loads the key pair paths from disk and appends them to the tls config.
func WithKeyPairFromPath(cert, key string) func(*tls.Config) error {
	return func(cfg *tls.Config) error {
		cert, err := tls.LoadX509KeyPair(cert, key)
		if err != nil {
			return err
		}
		cfg.Certificates = append(cfg.Certificates, cert)
		return nil
	}
}

// WithCACert makes a TLS config opotion which appends the provided PEM encoded bytes the tls config's cert pool.
// If a cert pool is not defined on the tls config an empty one will be created.
func WithCACert(pem []byte) func(*tls.Config) error {
	return func(cfg *tls.Config) error {
		if cfg.ClientCAs == nil {
			cfg.ClientCAs = x509.NewCertPool()
		}
		if !cfg.ClientCAs.AppendCertsFromPEM(pem) {
			return fmt.Errorf("could not parse ca cert pem")
		}
		return nil
	}
}

// DefaultServerCiphers is the list of accepted TLS ciphers, with known weak ciphers elided
// Note this list should be a moving target.
func DefaultServerCiphers() []uint16 {
	return []uint16{
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
		tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
		tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,

		tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	}
}
