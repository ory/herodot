// Copyright © 2026 Ory Corp
// SPDX-License-Identifier: Apache-2.0

// Command protoc-gen-herodot generates herodot definitions, documentation and
// artifacts for layers that aren't written in Go from error catalogs.
//
// A catalog is an enum annotated with (ory.herodot.v1.catalog). Its values are
// reasons annotated with (ory.herodot.v1.value).reason. See
// ory/herodot/v1/herodot.proto.
//
// The "lang" parameter selects the outputs and may be repeated:
//
//   - go (default): one <file>.herodot.go per proto file, next to the
//     protoc-gen-go output of the file's go_package, with one
//     herodot.Definition per reason.
//   - json: <domain>.errors.json, the full catalog of each domain.
//   - ts: <domain>.errors.ts, reasons and definitions for TypeScript.
//   - md: <domain>.md, the documentation of each domain.
//   - openapi: <domain>.openapi.json, the schema of ErrorInfo.reason.
//   - codetable: the code table of package github.com/ory/herodot/v2, from
//     ory/herodot/v1/herodot.proto. Only herodot itself uses it.
//
// Example buf.gen.yaml:
//
//	plugins:
//	  - local: ["go", "run", "github.com/ory/herodot/v2/cmd/protoc-gen-herodot"]
//	    out: .
//	    opt: [paths=source_relative]
//	  - local: ["go", "run", "github.com/ory/herodot/v2/cmd/protoc-gen-herodot"]
//	    out: gen
//	    opt: [lang=json, lang=ts, lang=md, lang=openapi]
package main

import (
	"flag"
	"fmt"
	"os"
	"slices"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/pluginpb"
)

var languages = []string{"go", "json", "ts", "md", "openapi", "codetable"}

func main() {
	if len(os.Args) > 1 {
		fmt.Fprintln(os.Stderr, "protoc-gen-herodot is a protoc plugin; run it through buf or protoc.")
		os.Exit(1)
	}

	var flags flag.FlagSet
	var langs []string
	flags.Func("lang", "output to generate, one of go, json, ts, md, openapi, codetable", func(v string) error {
		if !slices.Contains(languages, v) {
			return fmt.Errorf("unknown lang %q, want one of %v", v, languages)
		}
		langs = append(langs, v)
		return nil
	})

	protogen.Options{ParamFunc: flags.Set}.Run(func(gen *protogen.Plugin) error {
		gen.SupportedFeatures = uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL)
		if len(langs) == 0 {
			langs = []string{"go"}
		}
		return generate(gen, langs)
	})
}

func generate(gen *protogen.Plugin, langs []string) error {
	codes, err := loadCodeTable(gen)
	if err != nil {
		return err
	}
	cat, err := loadCatalogs(gen, codes)
	if err != nil {
		return err
	}

	for _, lang := range langs {
		switch lang {
		case "codetable":
			f, ok := gen.FilesByPath[codeProtoPath]
			if !ok || !f.Generate {
				return fmt.Errorf("lang=codetable requires generating %s", codeProtoPath)
			}
			generateCodeTable(gen, f, codes)
		case "go":
			if err := cat.validateGoNames(); err != nil {
				return err
			}
			for _, fc := range cat.files {
				generateGo(gen, fc)
			}
		case "json":
			for _, d := range cat.domains {
				if err := generateJSON(gen, d); err != nil {
					return err
				}
			}
		case "ts":
			for _, d := range cat.domains {
				generateTS(gen, d)
			}
		case "md":
			for _, d := range cat.domains {
				generateMarkdown(gen, d)
			}
		case "openapi":
			for _, d := range cat.domains {
				if err := generateOpenAPI(gen, d); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
