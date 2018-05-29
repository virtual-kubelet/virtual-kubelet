package azure

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"time"

	"github.com/Azure/go-autorest/autorest/adal"
)

// AADMock implements a AAD mock server .
type AADMock struct {
	server         *httptest.Server
	OnAcquireToken func(http.ResponseWriter, *http.Request)
}

// NewAADMock creates a new AAD server mocker.
func NewAADMock() *AADMock {
	aadServer := new(AADMock)
	aadServer.start()

	return aadServer
}

// Start the AAD server mocker.
func (mock *AADMock) start() {
	if mock.server != nil {
		return
	}

	mock.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if mock.OnAcquireToken != nil {
			mock.OnAcquireToken(w, r)
			return
		}

		w.WriteHeader(http.StatusOK)
		token := adal.Token{
			AccessToken: "Test Token",
			NotBefore:   strconv.FormatInt(time.Now().UnixNano(), 10),
			ExpiresIn:   strconv.FormatInt(int64(time.Minute), 10),
		}

		b := new(bytes.Buffer)
		json.NewEncoder(b).Encode(token)
		w.Write(b.Bytes())
	}))
}

// GetServerURL returns the mock server URL.
func (mock *AADMock) GetServerURL() string {
	if mock.server != nil {
		return mock.server.URL
	}

	panic("Mock server is not initialized.")
}

// Close terminates the AAD server mocker.
func (mock *AADMock) Close() {
	if mock.server != nil {
		mock.server.Close()
		mock.server = nil
	}
}
