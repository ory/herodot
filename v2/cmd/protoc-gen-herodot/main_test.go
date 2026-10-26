// Copyright © 2026 Ory Corp
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/bufbuild/protocompile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/rpc/code"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"

	"github.com/ory/herodot/v2/internal/herodotpb"
)

var update = flag.Bool("update", false, "update golden files")

func run(t *testing.T, param string, generateCodeProto bool, files ...*descriptorpb.FileDescriptorProto) (map[string]string, error) {
	t.Helper()
	req := &pluginpb.CodeGeneratorRequest{Parameter: new(param)}
	for _, fd := range []*descriptorpb.FileDescriptorProto{
		protodesc.ToFileDescriptorProto(descriptorpb.File_google_protobuf_descriptor_proto),
		protodesc.ToFileDescriptorProto(code.File_google_rpc_code_proto),
		protodesc.ToFileDescriptorProto(herodotpb.File_ory_herodot_v1_herodot_proto),
	} {
		req.ProtoFile = append(req.ProtoFile, fd)
	}
	if generateCodeProto {
		req.FileToGenerate = append(req.FileToGenerate, codeProtoPath)
	}
	for _, f := range files {
		req.ProtoFile = append(req.ProtoFile, f)
		req.FileToGenerate = append(req.FileToGenerate, f.GetName())
	}

	var langs []string
	var fs flag.FlagSet
	fs.Func("lang", "", func(v string) error { langs = append(langs, v); return nil })
	gen, err := protogen.Options{ParamFunc: fs.Set}.New(req)
	require.NoError(t, err)
	if len(langs) == 0 {
		langs = []string{"go"}
	}
	if err := generate(gen, langs); err != nil {
		return nil, err
	}
	resp := gen.Response()
	require.Empty(t, resp.GetError())

	out := map[string]string{}
	for _, f := range resp.File {
		out[f.GetName()] = f.GetContent()
	}
	return out, nil
}

// compile compiles protos, including their source info, against the real
// herodot.proto. Sources are looked up in sources first, then in testdata.
func compile(t *testing.T, sources map[string]string, paths ...string) []*descriptorpb.FileDescriptorProto {
	t.Helper()
	c := protocompile.Compiler{
		Resolver: protocompile.WithStandardImports(protocompile.CompositeResolver{
			&protocompile.SourceResolver{Accessor: protocompile.SourceAccessorFromMap(sources)},
			&protocompile.SourceResolver{ImportPaths: []string{"testdata", "../../proto"}},
			protocompile.ResolverFunc(func(path string) (protocompile.SearchResult, error) {
				fd, err := protoregistry.GlobalFiles.FindFileByPath(path)
				return protocompile.SearchResult{Desc: fd}, err
			}),
		}),
		SourceInfoMode: protocompile.SourceInfoStandard,
	}
	compiled, err := c.Compile(t.Context(), paths...)
	require.NoError(t, err)

	var files []*descriptorpb.FileDescriptorProto
	for _, f := range compiled {
		// Round trip through the wire format, as protoc does, so that the
		// options hold the generated extension types.
		b, err := proto.Marshal(protodesc.ToFileDescriptorProto(f))
		require.NoError(t, err)
		fd := &descriptorpb.FileDescriptorProto{}
		require.NoError(t, proto.Unmarshal(b, fd))
		files = append(files, fd)
	}
	return files
}

func kratosCatalogs(t *testing.T) []*descriptorpb.FileDescriptorProto {
	return compile(t, nil, "ory/kratos/errors/v1/flow.proto", "ory/kratos/errors/v1/identity.proto")
}

// TestGolden generates into testdata like buf with "out: ." and
// paths=source_relative: Go code lands next to its proto.
func TestGolden(t *testing.T) {
	out, err := run(t, "paths=source_relative,lang=go,lang=json,lang=ts,lang=md,lang=openapi", false, kratosCatalogs(t)...)
	require.NoError(t, err)

	expected := []string{
		"ory/kratos/errors/v1/flow.herodot.go",
		"ory/kratos/errors/v1/identity.herodot.go",
		"kratos.ory.com.errors.json",
		"kratos.ory.com.errors.ts",
		"kratos.ory.com.md",
		"kratos.ory.com.openapi.json",
	}
	var names []string
	for name := range out {
		names = append(names, name)
	}
	assert.ElementsMatch(t, expected, names)

	for name, content := range out {
		golden := filepath.Join("testdata", name)
		if *update {
			require.NoError(t, os.WriteFile(golden, []byte(content), 0o644))
			continue
		}
		b, err := os.ReadFile(golden)
		require.NoError(t, err, "run go test -update to create the golden files")
		assert.Equal(t, string(b), content, name)
	}
}

func TestCodeTableOutput(t *testing.T) {
	flowFile := kratosCatalogs(t)[0]
	out, err := run(t, "lang=codetable,module=github.com/ory/herodot/v2", true)
	require.NoError(t, err)
	assert.Contains(t, out, "code.herodot.go")

	out, err = run(t, "paths=source_relative", true, flowFile)
	require.NoError(t, err)
	assert.NotContains(t, out, "github.com/ory/herodot/v2/code.herodot.go", "only lang=codetable emits the code table")

	_, err = run(t, "lang=codetable", false, flowFile)
	assert.ErrorContains(t, err, "lang=codetable requires generating ory/herodot/v1/herodot.proto")
}

// catalog returns the source of a catalog enum in package pkg.
func catalog(pkg, domain, values string) string {
	return `syntax = "proto3";
package ` + pkg + `;
import "ory/herodot/v1/herodot.proto";
option go_package = "example.com/` + pkg + `";
enum Reason {
  option (ory.herodot.v1.catalog).domain = "` + domain + `";
  REASON_UNSPECIFIED = 0;
  ` + values + `
}`
}

func TestValidation(t *testing.T) {
	const valid = `X = 1 [(ory.herodot.v1.value).reason = {code: NOT_FOUND, message: "Message."}];`
	for _, tc := range []struct {
		name    string
		sources map[string]string
		err     string
	}{
		{"invalid domain", map[string]string{"a.proto": catalog("a", "Kratos", valid)}, `invalid domain "Kratos"`},
		{"invalid reason", map[string]string{"a.proto": catalog("a", "a.com",
			`Bad_Name = 1 [(ory.herodot.v1.value).reason = {code: NOT_FOUND, message: "Message."}];`)}, `invalid reason "Bad_Name"`},
		{"long reason", map[string]string{"a.proto": catalog("a", "a.com",
			strings.Repeat("A", 64)+` = 1 [(ory.herodot.v1.value).reason = {code: NOT_FOUND, message: "Message."}];`)}, "invalid reason"},
		{"missing spec", map[string]string{"a.proto": catalog("a", "a.com", `X = 1;`)}, "missing (ory.herodot.v1.value).reason"},
		{"code mapping on a reason", map[string]string{"a.proto": catalog("a", "a.com",
			`X = 1 [(ory.herodot.v1.value) = {reason: {code: NOT_FOUND, message: "Message."}, code: {http_status: 400}}];`)}, "(ory.herodot.v1.value).code is only for"},
		{"missing code", map[string]string{"a.proto": catalog("a", "a.com",
			`X = 1 [(ory.herodot.v1.value).reason = {message: "Message."}];`)}, "missing code"},
		{"unknown code", map[string]string{"a.proto": catalog("a", "a.com",
			`X = 1 [(ory.herodot.v1.value).reason = {code: 105, message: "Message."}];`)}, "unknown code 105"},
		{"missing message", map[string]string{"a.proto": catalog("a", "a.com",
			`X = 1 [(ory.herodot.v1.value).reason = {code: NOT_FOUND, message: " "}];`)}, "missing message"},
		{"lowercase message", map[string]string{"a.proto": catalog("a", "a.com",
			`X = 1 [(ory.herodot.v1.value).reason = {code: NOT_FOUND, message: "message."}];`)}, "starting with a capital letter"},
		{"message without period", map[string]string{"a.proto": catalog("a", "a.com",
			`X = 1 [(ory.herodot.v1.value).reason = {code: NOT_FOUND, message: "Not found"}];`)}, "ending with a period"},
		{"multi-line message", map[string]string{"a.proto": catalog("a", "a.com",
			`X = 1 [(ory.herodot.v1.value).reason = {code: NOT_FOUND, message: "Not found.\nTry again."}];`)}, "single line"},
		{"message with surrounding whitespace", map[string]string{"a.proto": catalog("a", "a.com",
			`X = 1 [(ory.herodot.v1.value).reason = {code: NOT_FOUND, message: "Not found. "}];`)}, "single line"},
		{"metadata key", map[string]string{"a.proto": catalog("a", "a.com",
			`X = 1 [(ory.herodot.v1.value).reason = {code: NOT_FOUND, message: "Message.", metadata_keys: ["flow_id"]}];`)}, `metadata key "flow_id" is not lowerCamelCase`},
		{"duplicate metadata key", map[string]string{"a.proto": catalog("a", "a.com",
			`X = 1 [(ory.herodot.v1.value).reason = {code: NOT_FOUND, message: "Message.", metadata_keys: ["a", "a"]}];`)}, `duplicate metadata key "a"`},
		{"duplicate reason across files", map[string]string{
			"a.proto": catalog("a", "a.com", valid),
			"b.proto": catalog("b", "a.com", valid),
		}, "duplicate reason a.com/X, already declared by a.X"},
	} {
		t.Run("case="+tc.name, func(t *testing.T) {
			_, err := run(t, "", false, compile(t, tc.sources, slices.Sorted(maps.Keys(tc.sources))...)...)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.err)
		})
	}

	t.Run("case=same reason in different domains", func(t *testing.T) {
		sources := map[string]string{
			"a.proto": catalog("a", "a.com", valid),
			"b.proto": catalog("b", "b.com", valid),
		}
		_, err := run(t, "", false, compile(t, sources, "a.proto", "b.proto")...)
		require.NoError(t, err)
	})
}

func TestCodeValidation(t *testing.T) {
	entry := func(name string, number int32, status uint32, canonical *codeEntry) *codeEntry {
		e := &codeEntry{name: name, number: number, httpStatus: status, canonical: canonical}
		if canonical == nil {
			e.canonical = e
		}
		return e
	}
	unimplemented := entry("UNIMPLEMENTED", 12, 501, nil)
	notFound := entry("NOT_FOUND", 5, 404, nil)
	gone := entry("GONE", 101, 410, notFound)

	assert.NoError(t, notFound.validate())
	assert.NoError(t, gone.validate())
	assert.ErrorContains(t, entry("METHOD_NOT_ALLOWED", 105, 405, unimplemented).validate(), "same class")
	assert.ErrorContains(t, entry("X", 50, 404, notFound).validate(), "numbered from 100")
	assert.ErrorContains(t, entry("X", 102, 410, gone).validate(), "must map to a canonical code")
	assert.ErrorContains(t, entry("X", 5, 404, gone).validate(), "must map to itself")
	assert.ErrorContains(t, entry("X", 102, 302, notFound).validate(), "want 4xx or 5xx")

	table := &codeTable{entries: []*codeEntry{notFound, gone}}
	assert.NoError(t, table.validateUnique())
	table.entries = append(table.entries, entry("REMOVED", 102, 410, notFound))
	assert.ErrorContains(t, table.validateUnique(), "codes GONE and REMOVED both map to NOT_FOUND and HTTP status 410")
}

func TestGoNameCollisions(t *testing.T) {
	for _, tc := range []struct{ name, first, second, domain string }{
		{"normalized names", "FOO_1", "FOO1", "a.com"},
		{"different domains", "FOO", "FOO", "b.com"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			value := func(name string) string {
				return name + ` = 1 [(ory.herodot.v1.value).reason = {code: NOT_FOUND, message: "Message."}];`
			}
			a := catalog("a", "a.com", value(tc.first))
			b := catalog("b", tc.domain, value(tc.second))
			sources := map[string]string{
				"a.proto": a,
				"b.proto": strings.Replace(b, "example.com/b", "example.com/a", 1),
			}
			files := compile(t, sources, "a.proto", "b.proto")
			_, err := run(t, "", false, files...)
			require.Error(t, err)
			assert.ErrorContains(t, err, "Go identifier")
			assert.ErrorContains(t, err, "a."+tc.first)
			assert.ErrorContains(t, err, "b."+tc.second)

			_, err = run(t, "lang=json", false, files...)
			require.NoError(t, err, "Go names must not constrain other outputs")

			sources["b.proto"] = b
			_, err = run(t, "", false, compile(t, sources, "a.proto", "b.proto")...)
			require.NoError(t, err, "separate Go packages may reuse identifiers")
		})
	}
}

func TestReasonRequiresCatalogDomain(t *testing.T) {
	const value = `X = 1 [(ory.herodot.v1.value).reason = {code: NOT_FOUND, message: "Message."}];`
	empty := catalog("a", "", value)
	missing := strings.Replace(empty, `option (ory.herodot.v1.catalog).domain = "";`, "", 1)
	for _, source := range []string{empty, missing} {
		_, err := run(t, "", false, compile(t, map[string]string{"a.proto": source}, "a.proto")...)
		require.Error(t, err)
		assert.ErrorContains(t, err, "a.proto: a.X")
		assert.ErrorContains(t, err, "requires a catalog domain on enum a.Reason")
	}

	ordinary := strings.Replace(missing, value, "X = 1;", 1)
	out, err := run(t, "", false, compile(t, map[string]string{"a.proto": ordinary}, "a.proto")...)
	require.NoError(t, err)
	assert.Empty(t, out, "ordinary enums without annotations are ignored")
}
