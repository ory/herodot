// Copyright © 2026 Ory Corp
// SPDX-License-Identifier: Apache-2.0

package herodot_test

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ory/herodot/v2"
)

var (
	flowExpired      = herodot.Define("kratos.ory.com", "FLOW_EXPIRED", herodot.CodeExpired, "The self-service flow has expired.")
	identityNotFound = herodot.Define("kratos.ory.com", "IDENTITY_NOT_FOUND", herodot.CodeNotFound, "The identity could not be found.")
)

func TestDefine(t *testing.T) {
	assert.Equal(t, "kratos.ory.com", flowExpired.Domain())
	assert.Equal(t, "FLOW_EXPIRED", flowExpired.Reason())
	assert.Equal(t, herodot.CodeExpired, flowExpired.Code())
	assert.Equal(t, "The self-service flow has expired.", flowExpired.Message())

	assert.Equal(t, flowExpired, herodot.Define("kratos.ory.com", "FLOW_EXPIRED", herodot.CodeExpired, "The self-service flow has expired."))
	assert.NotEqual(t, flowExpired, identityNotFound)

	for _, tc := range []struct {
		domain, reason string
		code           herodot.Code
	}{
		{"kratos", "FLOW_EXPIRED", herodot.CodeExpired},
		{"Kratos.ory.com", "FLOW_EXPIRED", herodot.CodeExpired},
		{"kratos.ory.com", "flow_expired", herodot.CodeExpired},
		{"kratos.ory.com", "FLOW__EXPIRED", herodot.CodeExpired},
		{"kratos.ory.com", "_FLOW", herodot.CodeExpired},
		{"kratos.ory.com", "1FLOW", herodot.CodeExpired},
		{"kratos.ory.com", strings.Repeat("A", 64), herodot.CodeExpired},
		{"kratos.ory.com", "FLOW_EXPIRED", 0},
		{"kratos.ory.com", "FLOW_EXPIRED", 99},
	} {
		t.Run(fmt.Sprintf("%s/%s/%d", tc.domain, tc.reason, tc.code), func(t *testing.T) {
			assert.Panics(t, func() { herodot.Define(tc.domain, tc.reason, tc.code, "message") })
		})
	}
}

func TestInternal(t *testing.T) {
	assert.Equal(t, "ory.com", herodot.Internal.Domain())
	assert.Equal(t, "INTERNAL", herodot.Internal.Reason())
	assert.Equal(t, herodot.CodeInternal, herodot.Internal.Code())
}

func TestNew(t *testing.T) {
	cause := errors.New("sql: no rows")
	e := identityNotFound.New(
		herodot.Cause(cause),
		herodot.Metadata("identityId", "1234"),
		herodot.Violation("id", "must exist"),
	)

	assert.Equal(t, identityNotFound, e.Definition())
	assert.Equal(t, "The identity could not be found.", e.Error())
	assert.Equal(t, map[string]string{"identityId": "1234"}, e.Metadata())
	assert.Equal(t, []herodot.FieldViolation{{Field: "id", Description: "must exist"}}, e.Violations())
	assert.Same(t, cause, e.Unwrap())
	assert.ErrorIs(t, e, cause)

	t.Run("case=copies are immutable", func(t *testing.T) {
		e.Metadata()["identityId"] = "changed"
		e.Violations()[0].Field = "changed"
		assert.Equal(t, "1234", e.Metadata()["identityId"])
		assert.Equal(t, "id", e.Violations()[0].Field)
	})

	t.Run("case=no details", func(t *testing.T) {
		e := flowExpired.New()
		assert.Nil(t, e.Metadata())
		assert.Nil(t, e.Violations())
		assert.Nil(t, e.Unwrap())
	})
}

func TestZeroValues(t *testing.T) {
	var e herodot.Error
	assert.Equal(t, herodot.Internal, e.Definition())
	assert.Equal(t, herodot.Internal.Message(), e.Error())
	assert.Nil(t, e.Unwrap())

	var d herodot.Definition
	assert.Equal(t, herodot.Internal.Reason(), d.Reason())
	assert.Equal(t, herodot.Internal, d.New().Definition())
}

func TestNewDefinition(t *testing.T) {
	d, err := herodot.NewDefinition("kratos.ory.com", "FLOW_EXPIRED", herodot.CodeExpired, "The self-service flow has expired.")
	require.NoError(t, err)
	assert.Equal(t, flowExpired, d)

	_, err = herodot.NewDefinition("kratos", "FLOW_EXPIRED", herodot.CodeExpired, "m")
	assert.ErrorContains(t, err, `invalid domain "kratos"`)
	_, err = herodot.NewDefinition("kratos.ory.com", "flow_expired", herodot.CodeExpired, "m")
	assert.ErrorContains(t, err, `invalid reason "flow_expired"`)
	_, err = herodot.NewDefinition("kratos.ory.com", "FLOW_EXPIRED", herodot.Code(42), "m")
	assert.ErrorContains(t, err, "invalid code 42")
}

func TestIdentity(t *testing.T) {
	e := flowExpired.New(herodot.Metadata("flowId", "1"))
	wrapped := fmt.Errorf("wrapped: %w", e)

	assert.True(t, flowExpired.Matches(wrapped))
	assert.False(t, identityNotFound.Matches(wrapped))
	assert.False(t, flowExpired.Matches(nil))
	assert.False(t, flowExpired.Matches(errors.New("other")))

	assert.ErrorIs(t, wrapped, flowExpired.New(), "message and details don't affect identity")
	assert.NotErrorIs(t, wrapped, identityNotFound.New())

	// Another version of the catalog with a different code and message.
	reworded := herodot.Define("kratos.ory.com", "FLOW_EXPIRED", herodot.CodeGone, "The flow is gone.")
	assert.True(t, reworded.SameIdentity(flowExpired))
	assert.True(t, reworded.Matches(wrapped))
	assert.ErrorIs(t, wrapped, reworded.New())
	otherDomain := herodot.Define("hydra.ory.com", "FLOW_EXPIRED", herodot.CodeExpired, "The self-service flow has expired.")
	assert.False(t, otherDomain.SameIdentity(flowExpired))
	assert.False(t, otherDomain.Matches(wrapped))

	assert.True(t, herodot.Definition{}.SameIdentity(herodot.Internal), "the zero Definition is Internal")
	assert.True(t, herodot.Internal.Matches(herodot.Error{}))

	var target herodot.Error
	require.ErrorAs(t, wrapped, &target)
	assert.Equal(t, e, target)
	assert.Equal(t, flowExpired, target.Definition())

	t.Run("case=matches any Error in the tree", func(t *testing.T) {
		inner := identityNotFound.New()
		outer := flowExpired.New(herodot.Cause(inner))
		assert.True(t, identityNotFound.Matches(outer))
		assert.True(t, flowExpired.Matches(outer))
		joined := errors.Join(errors.New("x"), inner)
		assert.True(t, identityNotFound.Matches(joined))
	})
}

func TestFormat(t *testing.T) {
	e := flowExpired.New(
		herodot.Metadata("b", "2"), herodot.Metadata("a", "1"),
		herodot.Violation("f", "bad"),
		herodot.Cause(errors.New("root")),
	)
	assert.Equal(t, "The self-service flow has expired.", fmt.Sprintf("%s", e))
	assert.Equal(t, "The self-service flow has expired.", fmt.Sprintf("%v", e))
	assert.Equal(t, `"The self-service flow has expired."`, fmt.Sprintf("%q", e))
	assert.Equal(t, `kratos.ory.com/FLOW_EXPIRED (EXPIRED): The self-service flow has expired.
metadata a=1
metadata b=2
violation f: bad
cause: root`, fmt.Sprintf("%+v", e))
}

type classifier struct{ e herodot.Error }

func (c classifier) Error() string                       { return "classifier" }
func (c classifier) HerodotError() (herodot.Error, bool) { return c.e, c.e != herodot.Error{} }

type wrappingClassifier struct {
	classifier
	err error
}

func (w wrappingClassifier) Unwrap() error { return w.err }

func TestFrom(t *testing.T) {
	t.Run("case=nil", func(t *testing.T) {
		assert.Equal(t, herodot.Error{}, herodot.From(nil))
	})

	t.Run("case=Error is returned as is", func(t *testing.T) {
		e := flowExpired.New()
		assert.Equal(t, e, herodot.From(e))
		assert.Equal(t, e, herodot.From(fmt.Errorf("a: %w", e)))
	})

	t.Run("case=unclassified errors are internal", func(t *testing.T) {
		cause := errors.New("boom")
		e := herodot.From(herodot.Annotate(cause, herodot.Metadata("dropped", "1")))
		assert.Equal(t, herodot.Internal, e.Definition())
		assert.Equal(t, herodot.Internal.Message(), e.Error(), "never the cause's text")
		assert.Nil(t, e.Metadata(), "annotations are dropped")
		assert.ErrorIs(t, e, cause)
	})

	t.Run("case=first Error in the tree", func(t *testing.T) {
		err := errors.Join(errors.New("x"), fmt.Errorf("y: %w", identityNotFound.New()), flowExpired.New())
		assert.Equal(t, identityNotFound, herodot.From(err).Definition())
	})

	t.Run("case=Error is preferred over an outer Classifier", func(t *testing.T) {
		err := wrappingClassifier{classifier: classifier{e: identityNotFound.New()}, err: flowExpired.New()}
		assert.Equal(t, flowExpired, herodot.From(err).Definition())
	})

	t.Run("case=first Classifier with a classification", func(t *testing.T) {
		err := fmt.Errorf("a: %w", errors.Join(
			classifier{},
			classifier{e: identityNotFound.New()},
			classifier{e: flowExpired.New()},
		))
		assert.Equal(t, identityNotFound, herodot.From(err).Definition())
	})

	t.Run("case=annotations", func(t *testing.T) {
		below := herodot.Annotate(errors.New("cause"), herodot.Metadata("below", "ignored"))
		e := flowExpired.New(herodot.Metadata("flowId", "1"), herodot.Metadata("keep", "inner"), herodot.Violation("a", "1"), herodot.Cause(below))

		err := herodot.Annotate(
			fmt.Errorf("ctx: %w", herodot.Annotate(e, herodot.Metadata("flowId", "inner"), herodot.Violation("b", "2"))),
			herodot.Metadata("flowId", "outer"), herodot.Metadata("returnTo", "/x"), herodot.Violation("c", "3"),
		)

		got := herodot.From(err)
		assert.Equal(t, flowExpired, got.Definition())
		assert.Equal(t, e.Error(), got.Error())
		assert.Equal(t, map[string]string{"flowId": "outer", "keep": "inner", "returnTo": "/x"}, got.Metadata())
		assert.Equal(t, []herodot.FieldViolation{{"a", "1"}, {"b", "2"}, {"c", "3"}}, got.Violations())
		assert.Same(t, below, got.Unwrap())

		assert.Equal(t, map[string]string{"flowId": "1", "keep": "inner"}, e.Metadata(), "the original is unchanged")

		var plain herodot.Error
		require.ErrorAs(t, err, &plain)
		assert.Equal(t, e, plain, "errors.As returns the error without the annotations")
		assert.True(t, flowExpired.Matches(err))
		assert.Equal(t, "ctx: "+e.Error(), err.Error())
	})

	t.Run("case=annotations apply to Classifier results", func(t *testing.T) {
		err := herodot.Annotate(classifier{e: identityNotFound.New()}, herodot.Metadata("k", "v"))
		got := herodot.From(err)
		assert.Equal(t, identityNotFound, got.Definition())
		assert.Equal(t, map[string]string{"k": "v"}, got.Metadata())
	})

	t.Run("case=annotations on other branches are ignored", func(t *testing.T) {
		err := errors.Join(herodot.Annotate(io.EOF, herodot.Metadata("other", "1")), herodot.Annotate(flowExpired.New(), herodot.Metadata("mine", "1")))
		assert.Equal(t, map[string]string{"mine": "1"}, herodot.From(err).Metadata())
	})
}

func TestAnnotate(t *testing.T) {
	assert.NoError(t, herodot.Annotate(nil, herodot.Metadata("a", "b")))
	assert.Same(t, io.EOF, herodot.Annotate(io.EOF))
	err := herodot.Annotate(io.EOF, herodot.Metadata("a", "b"))
	assert.ErrorIs(t, err, io.EOF)
	assert.Equal(t, io.EOF.Error(), err.Error())
}

type stackErr struct{ frames []string }

func (e stackErr) Error() string        { return "with stack" }
func (e stackErr) StackTrace() []string { return e.frames }

func TestDebugInfo(t *testing.T) {
	_, ok := flowExpired.New().DebugInfo()
	assert.False(t, ok, "no cause")

	info, ok := flowExpired.New(herodot.Cause(errors.New("root cause"))).DebugInfo()
	require.True(t, ok)
	assert.Equal(t, herodot.DebugInfo{Detail: "root cause"}, info)

	inner := stackErr{frames: []string{"inner.go:1", "inner.go:2"}}
	outer := wrapStack{stackErr{frames: []string{"outer.go:1"}}, inner}
	info, ok = flowExpired.New(herodot.Cause(fmt.Errorf("db: %w", outer))).DebugInfo()
	require.True(t, ok)
	assert.Equal(t, []string{"inner.go:1", "inner.go:2"}, info.StackEntries, "innermost stack trace wins")
	assert.Equal(t, "db: with stack", info.Detail)
}

// wrapStack is an error with a stack trace that wraps another one.
type wrapStack struct {
	stackErr
	inner error
}

func (w wrapStack) Unwrap() error { return w.inner }
