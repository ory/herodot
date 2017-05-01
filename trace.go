package herodot

import (
	"fmt"
	"reflect"

	"github.com/pkg/errors"
)

type stackTracer interface {
	StackTrace() errors.StackTrace
}

// getErrorTrace is a helper that returns an error's trace
func getErrorTrace(err error) string {
	if e, ok := err.(stackTracer); ok {
		return fmt.Sprintf("Stack trace: %+v", e.StackTrace())
	}

	return fmt.Sprintf("Stack trace could not be recovered from error type %s", reflect.TypeOf(err))
}
