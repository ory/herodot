// Copyright © 2026 Ory Corp
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"

	"github.com/ory/herodot/v2/internal/herodotpb"
)

const codeProtoPath = "ory/herodot/v1/herodot.proto"

// codeEntry is one value of the ory.herodot.v1.Code enum.
type codeEntry struct {
	name       string
	number     int32
	canonical  *codeEntry
	httpStatus uint32
	comment    protogen.Comments
}

// goName returns the name of the Go constant, e.g. CodeExpired.
func (c *codeEntry) goName() string { return "Code" + camelCase(c.name) }

type codeTable struct {
	entries  []*codeEntry
	byNumber map[int32]*codeEntry
}

// loadCodeTable reads the code table from the herodot.proto in the request,
// which every catalog imports, and validates it.
func loadCodeTable(gen *protogen.Plugin) (*codeTable, error) {
	f, ok := gen.FilesByPath[codeProtoPath]
	if !ok {
		// No catalog in the request.
		return &codeTable{byNumber: map[int32]*codeEntry{}}, nil
	}

	var enum *protogen.Enum
	for _, e := range f.Enums {
		if e.Desc.Name() == "Code" {
			enum = e
		}
	}
	if enum == nil {
		return nil, fmt.Errorf("%s: enum Code not found", codeProtoPath)
	}

	t := &codeTable{byNumber: map[int32]*codeEntry{}}
	canonical := map[int32]int32{}
	for _, v := range enum.Values {
		n := int32(v.Desc.Number())
		if n == 0 {
			continue
		}
		vo, _ := proto.GetExtension(v.Desc.Options(), herodotpb.E_Value).(*herodotpb.ValueOptions)
		m := vo.GetCode()
		if m == nil {
			return nil, fmt.Errorf("%s: code %s has no mapping", codeProtoPath, v.Desc.Name())
		}
		e := &codeEntry{name: string(v.Desc.Name()), number: n, httpStatus: m.GetHttpStatus(), comment: v.Comments.Leading}
		t.entries = append(t.entries, e)
		t.byNumber[n] = e
		canonical[n] = int32(m.GetCanonical())
	}

	for _, e := range t.entries {
		c, ok := t.byNumber[canonical[e.number]]
		if !ok {
			return nil, fmt.Errorf("%s: code %s maps to unknown canonical code %d", codeProtoPath, e.name, canonical[e.number])
		}
		e.canonical = c
	}
	for _, e := range t.entries {
		if err := e.validate(); err != nil {
			return nil, fmt.Errorf("%s: %w", codeProtoPath, err)
		}
	}
	if err := t.validateUnique(); err != nil {
		return nil, fmt.Errorf("%s: %w", codeProtoPath, err)
	}
	return t, nil
}

// validateUnique checks that no two codes share a canonical code and an HTTP
// status. Decoders rely on it to recover a refined code from an HTTP response,
// which carries only these two.
func (t *codeTable) validateUnique() error {
	type key struct {
		canonical  int32
		httpStatus uint32
	}
	seen := map[key]*codeEntry{}
	for _, e := range t.entries {
		k := key{e.canonical.number, e.httpStatus}
		if other, ok := seen[k]; ok {
			return fmt.Errorf("codes %s and %s both map to %s and HTTP status %d", other.name, e.name, e.canonical.name, e.httpStatus)
		}
		seen[k] = e
	}
	return nil
}

func (c *codeEntry) validate() error {
	isCanonical := c.number >= 1 && c.number <= 16
	switch {
	case isCanonical && c.canonical != c:
		return fmt.Errorf("canonical code %s must map to itself, not %s", c.name, c.canonical.name)
	case !isCanonical && c.number < 100:
		return fmt.Errorf("refinement %s must be numbered from 100", c.name)
	case !isCanonical && c.canonical.canonical != c.canonical:
		return fmt.Errorf("refinement %s must map to a canonical code, not %s", c.name, c.canonical.name)
	case c.httpStatus < 400 || c.httpStatus > 599:
		return fmt.Errorf("code %s maps to HTTP status %d, want 4xx or 5xx", c.name, c.httpStatus)
	case c.httpStatus/100 != c.canonical.httpStatus/100:
		// Client-side canonical codes map to 4xx and server-side codes to
		// 5xx, so that retry and alerting logic agrees across transports.
		return fmt.Errorf("code %s maps to HTTP status %d, but its canonical code %s maps to %d; both must be in the same class",
			c.name, c.httpStatus, c.canonical.name, c.canonical.httpStatus)
	}
	return nil
}

func generateCodeTable(gen *protogen.Plugin, f *protogen.File, t *codeTable) {
	g := gen.NewGeneratedFile(herodotImportPath+"/code.herodot.go", herodotImportPath)
	fileHeader(g, f)
	g.P("// ", generatedHeader)
	g.P("// source: ", f.Desc.Path())
	g.P()
	g.P("package herodot")
	g.P()
	g.P("const (")
	for i, e := range t.entries {
		if i > 0 && e.number >= 100 && t.entries[i-1].number < 100 {
			g.P()
			g.P("// Refinements.")
		}
		if e.comment != "" {
			g.P(trimComment(e.comment.String()))
		}
		g.P(e.goName(), " Code = ", e.number)
	}
	g.P(")")
	g.P()
	g.P("var codeTable = map[Code]codeInfo{")
	for _, e := range t.entries {
		g.P(e.goName(), `: {name: "`, e.name, `", canonical: `, e.canonical.goName(), ", httpStatus: ", e.httpStatus, "},")
	}
	g.P("}")
}
