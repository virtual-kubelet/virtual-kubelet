package selfupdate

import (
	"fmt"
	"io"
	"net/http"
)

// Requester interface allows developers to customize the method in which
// requests are made to retrieve the version and binary
type Requester interface {
	Fetch(url string) (io.ReadCloser, error)
}

// HTTPRequester is the normal requester that is used and does an HTTP
// to the url location requested to retrieve the specified data.
type HTTPRequester struct {
}

// Fetch will return an HTTP request to the specified url and return
// the body of the result. An error will occur for a non 200 status code.
func (httpRequester *HTTPRequester) Fetch(url string) (io.ReadCloser, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("bad http status from %s: %v", url, resp.Status)
	}

	return resp.Body, nil
}

// mockRequester used for some mock testing to ensure the requester contract
// works as specified.
type mockRequester struct {
	currentIndex int
	fetches      []func(string) (io.ReadCloser, error)
}

func (mr *mockRequester) handleRequest(requestHandler func(string) (io.ReadCloser, error)) {
	if mr.fetches == nil {
		mr.fetches = []func(string) (io.ReadCloser, error){}
	}
	mr.fetches = append(mr.fetches, requestHandler)
}

func (mr *mockRequester) Fetch(url string) (io.ReadCloser, error) {
	if len(mr.fetches) <= mr.currentIndex {
		return nil, fmt.Errorf("No for currentIndex %d to mock", mr.currentIndex)
	}
	current := mr.fetches[mr.currentIndex]
	mr.currentIndex++

	return current(url)
}
