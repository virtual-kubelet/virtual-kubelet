package azure

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/Azure/go-autorest/autorest/adal"
)

// Client represents authentication details and cloud specific parameters for
// Azure Resource Manager clients.
type Client struct {
	Authentication   *Authentication
	BaseURI          string
	HTTPClient       *http.Client
	BearerAuthorizer *BearerAuthorizer
}

// BearerAuthorizer implements the bearer authorization.
type BearerAuthorizer struct {
	tokenProvider adal.OAuthTokenProvider
}

type userAgentTransport struct {
	userAgent []string
	base      http.RoundTripper
	client    *Client
}

// NewClient creates a new Azure API client from an Authentication struct and BaseURI.
func NewClient(auth *Authentication, baseURI string, userAgent []string) (*Client, error) {
	resource, err := getResourceForToken(auth, baseURI)
	if err != nil {
		return nil, fmt.Errorf("Getting resource for token failed: %v", err)
	}
	client := &Client{
		Authentication: auth,
		BaseURI:        resource,
	}

	config, err := adal.NewOAuthConfig(auth.ActiveDirectoryEndpoint, auth.TenantID)
	if err != nil {
		return nil, fmt.Errorf("Creating new OAuth config for active directory failed: %v", err)
	}

	tp, err := adal.NewServicePrincipalToken(*config, auth.ClientID, auth.ClientSecret, resource)
	if err != nil {
		return nil, fmt.Errorf("Creating new service principal token failed: %v", err)
	}

	client.BearerAuthorizer = &BearerAuthorizer{tokenProvider: tp}

	nonEmptyUserAgent := userAgent[:0]
	for _, ua := range userAgent {
		if ua != "" {
			nonEmptyUserAgent = append(nonEmptyUserAgent, ua)
		}
	}

	uat := userAgentTransport{
		base:      http.DefaultTransport,
		userAgent: nonEmptyUserAgent,
		client:    client,
	}

	client.HTTPClient = &http.Client{
		Transport: uat,
	}

	return client, nil
}

func (t userAgentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.base == nil {
		return nil, errors.New("RoundTrip: no Transport specified")
	}

	newReq := *req
	newReq.Header = make(http.Header)
	for k, vv := range req.Header {
		newReq.Header[k] = vv
	}

	// Add the user agent header.
	newReq.Header["User-Agent"] = []string{strings.Join(t.userAgent, " ")}

	// Add the content-type header.
	newReq.Header["Content-Type"] = []string{"application/json"}

	// Refresh the token if necessary
	// TODO: don't refresh the token everytime
	refresher, ok := t.client.BearerAuthorizer.tokenProvider.(adal.Refresher)
	if ok {
		if err := refresher.EnsureFresh(); err != nil {
			return nil, fmt.Errorf("Failed to refresh the authorization token for request to %s: %v", newReq.URL, err)
		}
	}

	// Add the authorization header.
	newReq.Header["Authorization"] = []string{fmt.Sprintf("Bearer %s", t.client.BearerAuthorizer.tokenProvider.OAuthToken())}

	return t.base.RoundTrip(&newReq)
}

func getResourceForToken(auth *Authentication, baseURI string) (string, error) {
	// Compare dafault base URI from the SDK to the endpoints from the public cloud
	// Base URI and token resource are the same string. This func finds the authentication
	// file field that matches the SDK base URI. The SDK defines the public cloud
	// endpoint as its default base URI
	if !strings.HasSuffix(baseURI, "/") {
		baseURI += "/"
	}
	switch baseURI {
	case PublicCloud.ServiceManagementEndpoint:
		return auth.ManagementEndpoint, nil
	case PublicCloud.ResourceManagerEndpoint:
		return auth.ResourceManagerEndpoint, nil
	case PublicCloud.ActiveDirectoryEndpoint:
		return auth.ActiveDirectoryEndpoint, nil
	case PublicCloud.GalleryEndpoint:
		return auth.GalleryEndpoint, nil
	case PublicCloud.GraphEndpoint:
		return auth.GraphResourceID, nil
	}
	return "", fmt.Errorf("baseURI provided %q not found in endpoints", baseURI)
}
