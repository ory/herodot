package herodot

import (
	"github.com/pkg/errors"
)

type stackTracer interface {
	StackTrace() errors.StackTrace
}
