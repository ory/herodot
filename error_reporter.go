package herodot

import (
	"net/http"

	"github.com/pkg/errors"

	"github.com/ory/x/logrusx"
)

type stackTracer interface {
	StackTrace() errors.StackTrace
}

func DefaultErrorReporter(logger *logrusx.Logger, args ...interface{}) func(w http.ResponseWriter, r *http.Request, code int, err error) {
	return func(w http.ResponseWriter, r *http.Request, code int, err error) {
		if logger == nil {
			logger = logrusx.New("", "")
			logger.Warn("No logger was set in json, defaulting to standard logger.")
		}

		logger.WithError(err).WithRequest(r).WithField("http_response", map[string]interface{}{
			"status_code": code,
		}).Error(args...)
	}
}
