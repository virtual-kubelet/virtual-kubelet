package nodeutil

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/virtual-kubelet/virtual-kubelet/log"
	"github.com/virtual-kubelet/virtual-kubelet/trace"
	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/apiserver/pkg/authentication/authenticatorfactory"
	"k8s.io/apiserver/pkg/authentication/request/anonymous"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/authorization/authorizerfactory"
	"k8s.io/apiserver/pkg/server/options"
	"k8s.io/client-go/kubernetes"
)

// Auth is the interface used to implement authn/authz for http requests
type Auth interface {
	authenticator.Request
	authorizer.RequestAttributesGetter
	authorizer.Authorizer
}

type authWrapper struct {
	authenticator.Request
	authorizer.RequestAttributesGetter
	authorizer.Authorizer
}

// InstrumentAuth wraps the provided Auth in a new instrumented Auth
//
// Note: You would only need this if you rolled your own auth.
// The Auth implementations defined in this package are already instrumented.
func InstrumentAuth(auth Auth) Auth {
	if _, ok := auth.(*authWrapper); ok {
		// This is already instrumented
		return auth
	}
	return &authWrapper{
		Request:                 auth,
		RequestAttributesGetter: auth,
		Authorizer:              auth,
	}
}

// NoAuth creates an Auth which allows anonymous access to all resouorces
func NoAuth() Auth {
	return &authWrapper{
		Request:                 anonymous.NewAuthenticator(),
		RequestAttributesGetter: &NodeRequestAttr{},
		Authorizer:              authorizerfactory.NewAlwaysAllowAuthorizer(),
	}
}

// WithAuth makes a new http handler which wraps the provided handler with authn/authz.
func WithAuth(auth Auth, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleAuth(auth, w, r, h)
	})
}

func handleAuth(auth Auth, w http.ResponseWriter, r *http.Request, next http.Handler) {
	ctx := r.Context()
	ctx, span := trace.StartSpan(ctx, "vk.handleAuth")
	defer span.End()
	r = r.WithContext(ctx)

	info, ok, err := auth.AuthenticateRequest(r)
	if err != nil || !ok {
		log.G(r.Context()).WithError(err).Error("Authorization error")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	logger := log.G(ctx).WithFields(log.Fields{
		"user-name": info.User.GetName(),
		"user-id":   info.User.GetUID(),
	})

	ctx = log.WithLogger(ctx, logger)
	r = r.WithContext(ctx)

	attrs := auth.GetRequestAttributes(info.User, r)

	decision, _, err := auth.Authorize(ctx, attrs)
	if err != nil {
		log.G(r.Context()).WithError(err).Error("Authorization error")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if decision != authorizer.DecisionAllow {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	next.ServeHTTP(w, r)
}

// WebhookAuthOption is used as a functional argument to configure webhook auth.
type WebhookAuthOption func(*WebhookAuthConfig) error

// WebhookAuthConfig stores the configurations for authn/authz and is used by WebhookAuthOption to expose to callers.
type WebhookAuthConfig struct {
	AuthnConfig authenticatorfactory.DelegatingAuthenticatorConfig
	AuthzConfig authorizerfactory.DelegatingAuthorizerConfig
}

// WebhookAuth creates an Auth suitable to use with kubelet webhook auth.
// You must provide a CA provider to the authentication config, otherwise mTLS is disabled.
func WebhookAuth(client kubernetes.Interface, nodeName string, opts ...WebhookAuthOption) (Auth, error) {
	cfg := WebhookAuthConfig{
		AuthnConfig: authenticatorfactory.DelegatingAuthenticatorConfig{
			CacheTTL:            2 * time.Minute, // default taken from k8s.io/kubernetes/pkg/kubelet/apis/config/v1beta1
			WebhookRetryBackoff: options.DefaultAuthWebhookRetryBackoff(),
		},
		AuthzConfig: authorizerfactory.DelegatingAuthorizerConfig{
			AllowCacheTTL:       5 * time.Minute,  // default taken from k8s.io/kubernetes/pkg/kubelet/apis/config/v1beta1
			DenyCacheTTL:        30 * time.Second, // default taken from k8s.io/kubernetes/pkg/kubelet/apis/config/v1beta1
			WebhookRetryBackoff: options.DefaultAuthWebhookRetryBackoff(),
		},
	}

	for _, o := range opts {
		if err := o(&cfg); err != nil {
			return nil, err
		}
	}

	cfg.AuthnConfig.TokenAccessReviewClient = client.AuthenticationV1()
	cfg.AuthzConfig.SubjectAccessReviewClient = client.AuthorizationV1()

	authn, _, err := cfg.AuthnConfig.New()
	if err != nil {
		return nil, err
	}

	authz, err := cfg.AuthzConfig.New()
	if err != nil {
		return nil, err
	}
	return &authWrapper{
		Request:                 authn,
		RequestAttributesGetter: NodeRequestAttr{nodeName},
		Authorizer:              authz,
	}, nil
}

func (w *authWrapper) AuthenticateRequest(r *http.Request) (*authenticator.Response, bool, error) {
	ctx, span := trace.StartSpan(r.Context(), "AuthenticateRequest")
	defer span.End()
	return w.Request.AuthenticateRequest(r.WithContext(ctx))
}

func (w *authWrapper) Authorize(ctx context.Context, a authorizer.Attributes) (authorizer.Decision, string, error) {
	ctx, span := trace.StartSpan(ctx, "Authorize")
	defer span.End()
	return w.Authorizer.Authorize(ctx, a)
}

// NodeRequestAttr is a authorizor.RequeestAttributesGetter which can be used in the Auth interface.
type NodeRequestAttr struct {
	NodeName string
}

// GetRequestAttributes satisfies the authorizer.RequestAttributesGetter interface for use with an `Auth`.
func (a NodeRequestAttr) GetRequestAttributes(u user.Info, r *http.Request) authorizer.Attributes {
	return authorizer.AttributesRecord{
		User:            u,
		Verb:            getAPIVerb(r),
		Namespace:       "",
		APIGroup:        "",
		APIVersion:      "v1",
		Resource:        "nodes",
		Name:            a.NodeName,
		ResourceRequest: true,
		Path:            r.URL.Path,
		Subresource:     getSubresource(r),
	}
}

func getAPIVerb(r *http.Request) string {
	switch r.Method {
	case http.MethodPost:
		return "create"
	case http.MethodGet:
		return "get"
	case http.MethodPut:
		return "update"
	case http.MethodPatch:
		return "patch"
	case http.MethodDelete:
		return "delete"
	}
	return ""
}

func isSubpath(subpath, path string) bool {
	// Taken from k8s.io/kubernetes/pkg/kubelet/server/auth.go
	return subpath == path || (strings.HasPrefix(subpath, path) && subpath[len(path)] == '/')
}

func getSubresource(r *http.Request) string {
	if isSubpath(r.URL.Path, "/stats") {
		return "stats"
	}
	if isSubpath(r.URL.Path, "/metrics") {
		return "metrics"
	}
	if isSubpath(r.URL.Path, "/logs") {
		// yes, "log", not "logs"
		// per kubelet code: "log" to match other log subresources (pods/log, etc)
		return "log"
	}

	return "proxy"
}
