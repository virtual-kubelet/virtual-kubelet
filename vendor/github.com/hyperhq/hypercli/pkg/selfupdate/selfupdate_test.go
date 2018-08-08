package selfupdate

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"testing"
)

var testHash = sha256.New()

func TestUpdaterFetchMustReturnNonNilReaderCloser(t *testing.T) {
	mr := &mockRequester{}
	mr.handleRequest(
		func(url string) (io.ReadCloser, error) {
			return nil, nil
		})
	updater := createUpdater(mr)
	err := updater.BackgroundRun()
	if err != nil {
		equals(t, "Fetch was expected to return non-nil ReadCloser", err.Error())
	} else {
		t.Log("Expected an error")
		t.Fail()
	}

}

func TestUpdaterWithEmptyPaloadNoErrorNoUpdate(t *testing.T) {
	mr := &mockRequester{}
	mr.handleRequest(
		func(url string) (io.ReadCloser, error) {
			equals(t, "http://updates.yourdomain.com/myapp/darwin-amd64.json", url)
			return newTestReaderCloser("{}"), nil
		})
	updater := createUpdater(mr)

	err := updater.BackgroundRun()
	if err != nil {
		t.Errorf("Error occured: %#v", err)
	}

}

func createUpdater(mr *mockRequester) *Updater {
	return &Updater{
		CurrentVersion: "1.2",
		ApiURL:         "http://updates.yourdomain.com/",
		BinURL:         "http://updates.yourdownmain.com/",
		DiffURL:        "http://updates.yourdomain.com/",
		Dir:            "update/",
		CmdName:        "myapp", // app name
		Requester:      mr,
	}
}

func equals(t *testing.T, expected, actual interface{}) {
	if expected != actual {
		t.Log(fmt.Sprintf("Expected: %#v %#v\n", expected, actual))
		t.Fail()
	}
}

type testReadCloser struct {
	buffer *bytes.Buffer
}

func newTestReaderCloser(payload string) io.ReadCloser {
	return &testReadCloser{buffer: bytes.NewBufferString(payload)}
}

func (trc *testReadCloser) Read(p []byte) (n int, err error) {
	return trc.buffer.Read(p)
}

func (trc *testReadCloser) Close() error {
	return nil
}
