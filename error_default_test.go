package herodot

import (
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestToDefaultError(t *testing.T) {
	t.Run("case=stack", func(t *testing.T) {
		e := errors.New("hi")
		assert.EqualValues(t, e.(stackTracer).StackTrace(), toDefaultError(e, "").StackTrace())
	})

	t.Run("case=status", func(t *testing.T) {
		e := &DefaultError{
			StatusField: "foo-status",
		}
		assert.EqualValues(t, "foo-status", toDefaultError(e, "").Status())
	})

	t.Run("case=reason", func(t *testing.T) {
		e := &DefaultError{
			ReasonField: "foo-reason",
		}
		assert.EqualValues(t, "foo-reason", toDefaultError(e, "").Reason())
	})

	t.Run("case=debug", func(t *testing.T) {
		e := &DefaultError{
			DebugField: "foo-debug",
		}
		assert.EqualValues(t, "foo-debug", toDefaultError(e, "").Debug())
	})

	t.Run("case=debug", func(t *testing.T) {
		e := &DefaultError{
			DetailsField: map[string][]interface{}{"foo-debug": {"bar"}},
		}
		assert.EqualValues(t, map[string][]interface{}{"foo-debug": {"bar"}}, toDefaultError(e, "").Details())
	})

	t.Run("case=debug", func(t *testing.T) {
		e := &DefaultError{
			RIDField: "foo-rid",
		}
		assert.EqualValues(t, "foo-rid", toDefaultError(e, "").RequestID())
		assert.EqualValues(t, "fallback-rid", toDefaultError(new(DefaultError), "fallback-rid").RequestID())
	})

	t.Run("case=status-code", func(t *testing.T) {
		e := &DefaultError{
			CodeField: 501,
		}
		assert.EqualValues(t, 501, toDefaultError(e, "").StatusCode())
		assert.EqualValues(t, 500, toDefaultError(errors.New(""), "").StatusCode())
	})
}
