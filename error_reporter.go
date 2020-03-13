package herodot

import (
	"fmt"
	"net/http"
	"reflect"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type stackTracer interface {
	StackTrace() errors.StackTrace
}

type logLeveler interface {
	GetLevel() logrus.Level
}

func DefaultErrorReporter(logger logrus.FieldLogger, args ...interface{}) func(w http.ResponseWriter, r *http.Request, code int, err error) {
	return func(w http.ResponseWriter, r *http.Request, code int, err error) {
		if logger == nil {
			logger = logrus.StandardLogger()
			logger.Warning("No logger was set in json, defaulting to standard logger.")
		}

		DefaultErrorLogger(logger, ToDefaultError(err, r.Header.Get("X-Request-ID"))).
			WithField("writer", "JSON").
			WithField("status", code).
			Error(args...)
	}
}

func DefaultErrorLogger(logger logrus.FieldLogger, err error) logrus.FieldLogger {
	richError := ToDefaultError(err, "")
	richLogger := logger.
		WithField("request-id", richError.RequestID()).
		WithField("code", richError.StatusCode()).
		WithField("reason", richError.Reason()).
		WithField("debug", richError.Debug()).
		WithField("details", richError.Details()).
		WithField("status", richError.Status()).
		WithError(err)

	if leveler, ok := logger.(logLeveler); ok && leveler.GetLevel() >= logrus.DebugLevel {
		if e, ok := err.(stackTracer); ok {
			richLogger.WithField("trace", fmt.Sprintf("%+v", e.StackTrace()))
		} else {
			richLogger.WithField("trace", fmt.Sprintf("Stack trace could not be recovered from error type %s", reflect.TypeOf(err)))
		}
	}

	return richLogger
}
