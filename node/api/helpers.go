package api

import (
	"io"
	"net/http"

	"github.com/virtual-kubelet/virtual-kubelet/errdefs"
	"github.com/virtual-kubelet/virtual-kubelet/log"
)

type handlerFunc func(http.ResponseWriter, *http.Request) error

func handleError(f handlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		err := f(w, req)
		if err == nil {
			return
		}

		code := httpStatusCode(err)
		w.WriteHeader(code)
		io.WriteString(w, err.Error())
		logger := log.G(req.Context()).WithError(err).WithField("httpStatusCode", code)

		if code >= 500 {
			logger.Error("Internal server error on request")
		} else {
			logger.Debug("Error on request")
		}
	}
}

func flushOnWrite(w io.Writer) io.Writer {
	if fw, ok := w.(writeFlusher); ok {
		return &flushWriter{fw}
	}
	return w
}

type flushWriter struct {
	w writeFlusher
}

type writeFlusher interface {
	Flush() error
	Write([]byte) (int, error)
}

func (fw *flushWriter) Write(p []byte) (int, error) {
	n, err := fw.w.Write(p)
	if n > 0 {
		if err := fw.w.Flush(); err != nil {
			return n, err
		}
	}
	return n, err
}

func httpStatusCode(err error) int {
	switch {
	case err == nil:
		return http.StatusOK
	case errdefs.IsNotFound(err):
		return http.StatusNotFound
	case errdefs.IsInvalidInput(err):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}
