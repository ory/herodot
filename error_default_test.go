package herodot

import (
	"net/http"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestToDefaultError(t *testing.T) {
	t.Run("case=stack", func(t *testing.T) {
		e := errors.New("hi")
		assert.EqualValues(t, e.(stackTracer).StackTrace(), ToDefaultError(e, "").StackTrace())
	})

	t.Run("case=wrap", func(t *testing.T) {
		orig := errors.New("hi")
		wrap := new(DefaultError)
		wrap.Wrap(orig)

		assert.EqualValues(t, orig.(stackTracer).StackTrace(), wrap.StackTrace())
	})

	t.Run("case=wrap_self", func(t *testing.T) {
		wrap := new(DefaultError)
		wrap.Wrap(wrap)

		assert.Empty(t, wrap.StackTrace())
	})

	t.Run("case=status", func(t *testing.T) {
		e := &DefaultError{
			StatusField: "foo-status",
		}
		assert.EqualValues(t, "foo-status", ToDefaultError(e, "").Status())
	})

	t.Run("case=reason", func(t *testing.T) {
		e := &DefaultError{
			ReasonField: "foo-reason",
		}
		assert.EqualValues(t, "foo-reason", ToDefaultError(e, "").Reason())
	})

	t.Run("case=debug", func(t *testing.T) {
		e := &DefaultError{
			DebugField: "foo-debug",
		}
		assert.EqualValues(t, "foo-debug", ToDefaultError(e, "").Debug())
	})

	t.Run("case=details", func(t *testing.T) {
		e := &DefaultError{
			DetailsField: map[string]interface{}{"foo-debug": "bar"},
		}
		assert.EqualValues(t, map[string]interface{}{"foo-debug": "bar"}, ToDefaultError(e, "").Details())
	})

	t.Run("case=rid", func(t *testing.T) {
		e := &DefaultError{
			RIDField: "foo-rid",
		}
		assert.EqualValues(t, "foo-rid", ToDefaultError(e, "").RequestID())
		assert.EqualValues(t, "fallback-rid", ToDefaultError(new(DefaultError), "fallback-rid").RequestID())
	})

	t.Run("case=id", func(t *testing.T) {
		e := &DefaultError{
			IDField: "foo-rid",
		}
		assert.EqualValues(t, "foo-rid", ToDefaultError(e, "").ID())
	})

	t.Run("case=code", func(t *testing.T) {
		e := &DefaultError{CodeField: 501}
		assert.EqualValues(t, 501, ToDefaultError(e, "").StatusCode())
		assert.EqualValues(t, http.StatusText(501), ToDefaultError(e, "").Status())

		e = &DefaultError{CodeField: 501, StatusField: "foobar"}
		assert.EqualValues(t, 501, ToDefaultError(e, "").StatusCode())
		assert.EqualValues(t, "foobar", ToDefaultError(e, "").Status())

		assert.EqualValues(t, 500, ToDefaultError(errors.New(""), "").StatusCode())
	})
}
