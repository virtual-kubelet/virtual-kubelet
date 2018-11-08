// Package api contains the common code shared by all Azure API libraries.
package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

// Error contains an error response from the server.
type Error struct {
	// StatusCode is the HTTP response status code and will always be populated.
	StatusCode int `json:"statusCode"`
	// Code is the API error code that is given in the error message.
	Code string `json:"code"`
	// Message is the server response message and is only populated when
	// explicitly referenced by the JSON server response.
	Message string `json:"message"`
	// Body is the raw response returned by the server.
	// It is often but not always JSON, depending on how the request fails.
	Body string
	// Header contains the response header fields from the server.
	Header http.Header
	// URL is the URL of the original HTTP request and will always be populated.
	URL string
}

// Error converts the Error type to a readable string.
func (e *Error) Error() string {
	// If the message is empty return early.
	if e.Message == "" {
		return fmt.Sprintf("api call to %s: got HTTP response status code %d error code %q with body: %v", e.URL, e.StatusCode, e.Code, e.Body)
	}

	return fmt.Sprintf("api call to %s: got HTTP response status code %d error code %q: %s", e.URL, e.StatusCode, e.Code, e.Message)
}

type errorReply struct {
	Error *Error `json:"error"`
}

// CheckResponse returns an error (of type *Error) if the response
// status code is not 2xx.
func CheckResponse(res *http.Response) error {
	if res.StatusCode >= 200 && res.StatusCode <= 299 {
		return nil
	}

	slurp, err := ioutil.ReadAll(res.Body)
	if err == nil {
		jerr := new(errorReply)
		err = json.Unmarshal(slurp, jerr)
		if err == nil && jerr.Error != nil {
			if jerr.Error.StatusCode == 0 {
				jerr.Error.StatusCode = res.StatusCode
			}
			jerr.Error.Body = string(slurp)
			jerr.Error.URL = res.Request.URL.String()
			return jerr.Error
		}
	}

	return &Error{
		StatusCode: res.StatusCode,
		Body:       res.Status,
		Header:     res.Header,
	}
}
