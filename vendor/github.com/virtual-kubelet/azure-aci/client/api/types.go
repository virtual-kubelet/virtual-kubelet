package api

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// ResponseMetadata is embedded in each response and contains the HTTP response code and headers from the server.
type ResponseMetadata struct {
	// HTTPStatusCode is the server's response status code.
	HTTPStatusCode int
	// Header contains the response header fields from the server.
	Header http.Header
}

// JSONTime assumes the time format is RFC3339.
type JSONTime time.Time

const AzureTimeFormat = "2006-01-02T15:04:05Z"

// MarshalJSON ensures that the time is serialized as RFC3339.
func (t JSONTime) MarshalJSON() ([]byte, error) {
	// Serialize the JSON as RFC3339.
	stamp := fmt.Sprintf("\"%s\"", time.Time(t).Format(AzureTimeFormat))
	return []byte(stamp), nil
}

// UnmarshalJSON ensures that the time is deserialized as RFC3339.
func (t *JSONTime) UnmarshalJSON(data []byte) error {
	if t == nil {
		return errors.New("api.JSONTime: UnmarshalJSON on nil pointer")
	}

	parsed, err := time.Parse(AzureTimeFormat, string(bytes.Trim(data, "\"")))
	if err != nil {
		return fmt.Errorf("api.JSONTime: UnmarshalJSON failed: %v", err)
	}
	*t = JSONTime(parsed)
	return nil
}
