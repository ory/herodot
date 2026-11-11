// Copyright © 2026 Ory Corp
// SPDX-License-Identifier: Apache-2.0

package herodot

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/rpc/code"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/ory/herodot/v2/internal/herodotpb"
)

func TestCodeTable(t *testing.T) {
	for c, info := range codeTable {
		t.Run(info.name, func(t *testing.T) {
			canonical := info.canonical
			assert.Equal(t, canonical, codeTable[canonical].canonical, "canonical code must map to itself")
			assert.True(t, canonical >= 1 && canonical <= 16, "canonical code out of range")
			assert.Equal(t, code.Code(canonical).String(), canonical.String(), "canonical code must equal google.rpc.Code")
			if c < 100 {
				assert.Equal(t, c, canonical)
			}

			assert.Equal(t, codeTable[canonical].httpStatus/100, info.httpStatus/100,
				"refinement must keep the HTTP status class of its canonical code")
			assert.Contains(t, []int{4, 5}, info.httpStatus/100)

			// The generated table must match herodot.proto. If this fails, run
			// buf generate.
			assert.Equal(t, herodotpb.Code(c).String(), c.String())
			vo, _ := proto.GetExtension(herodotpb.Code(c).Descriptor().Values().ByNumber(protoreflect.EnumNumber(c)).Options(), herodotpb.E_Value).(*herodotpb.ValueOptions)
			m := vo.GetCode()
			require.NotNil(t, m)
			assert.EqualValues(t, m.GetCanonical(), canonical)
			assert.EqualValues(t, m.GetHttpStatus(), info.httpStatus)

			// Decoders recover the code from these two.
			found, ok := LookupCode(canonical, info.httpStatus)
			assert.True(t, ok)
			assert.Equal(t, c, found)
			assert.Equal(t, c < 100, c.IsCanonical())
		})
	}
	assert.Len(t, codeTable, len(herodotpb.Code_name)-1)
}

func TestCodeMethods(t *testing.T) {
	assert.Equal(t, CodeFailedPrecondition, CodeExpired.Canonical())
	assert.Equal(t, 410, CodeExpired.HTTPStatus())
	assert.Equal(t, "EXPIRED", CodeExpired.String())

	assert.Equal(t, CodeNotFound, CodeNotFound.Canonical())
	assert.Equal(t, 404, CodeNotFound.HTTPStatus())

	invalid := Code(0)
	assert.Equal(t, CodeUnknown, invalid.Canonical())
	assert.Equal(t, 500, invalid.HTTPStatus())
	assert.Equal(t, "CODE(0)", invalid.String())
}

func TestCodeLookups(t *testing.T) {
	c, ok := LookupCode(CodeFailedPrecondition, 410)
	assert.True(t, ok)
	assert.Equal(t, CodeExpired, c)
	_, ok = LookupCode(CodeFailedPrecondition, 418)
	assert.False(t, ok)
	_, ok = LookupCode(CodeExpired, 410)
	assert.False(t, ok, "not a canonical code")

	c, ok = CanonicalCode("FAILED_PRECONDITION")
	assert.True(t, ok)
	assert.Equal(t, CodeFailedPrecondition, c)
	_, ok = CanonicalCode("EXPIRED")
	assert.False(t, ok, "refinements aren't canonical")
	_, ok = CanonicalCode("Not Found")
	assert.False(t, ok)

	assert.False(t, Code(0).IsCanonical())
}
