// Copyright © 2026 Ory Corp
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/ory/herodot/v2/internal/herodotpb"
)

// These mirror the validation in package herodot, so that Define never panics
// on generated code.
var (
	domainPattern      = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+$`)
	reasonPattern      = regexp.MustCompile(`^[A-Z][A-Z0-9]*(_[A-Z0-9]+)*$`)
	metadataKeyPattern = regexp.MustCompile(`^[a-z][a-zA-Z0-9]*$`)
)

const maxReasonLength = 63

type reason struct {
	domain       string
	name         string
	code         *codeEntry
	message      string
	metadataKeys []string
	deprecated   bool
	comment      string
	value        *protogen.EnumValue
}

func (r *reason) goName() string { return camelCase(r.name) }

type catalogEnum struct {
	enum    *protogen.Enum
	domain  string
	reasons []*reason
}

type catalogFile struct {
	file  *protogen.File
	enums []*catalogEnum
}

type domain struct {
	name    string
	reasons []*reason
}

type catalogs struct {
	files   []*catalogFile
	domains []*domain
}

// loadCatalogs collects and validates the catalog enums of all files to
// generate. Duplicate reasons are detected across all of them.
func loadCatalogs(gen *protogen.Plugin, codes *codeTable) (*catalogs, error) {
	var (
		cat      = &catalogs{}
		byDomain = map[string]*domain{}
		seen     = map[string]string{} // domain/reason -> enum value full name
	)

	for _, f := range gen.Files {
		if !f.Generate {
			continue
		}
		cf := &catalogFile{file: f}
		for _, e := range allEnums(f) {
			co, _ := proto.GetExtension(e.Desc.Options(), herodotpb.E_Catalog).(*herodotpb.CatalogOptions)
			dn := co.GetDomain()
			if dn == "" {
				continue
			}
			if !domainPattern.MatchString(dn) || len(dn) > 253 {
				return nil, fmt.Errorf("%s: enum %s: invalid domain %q", f.Desc.Path(), e.Desc.FullName(), dn)
			}
			ce := &catalogEnum{enum: e, domain: dn}
			for _, v := range e.Values {
				r, err := loadReason(dn, v, codes)
				if err != nil {
					return nil, fmt.Errorf("%s: %s: %w", f.Desc.Path(), v.Desc.FullName(), err)
				}
				if r == nil {
					continue
				}
				key := dn + "/" + r.name
				if other, ok := seen[key]; ok {
					return nil, fmt.Errorf("%s: %s: duplicate reason %s, already declared by %s", f.Desc.Path(), v.Desc.FullName(), key, other)
				}
				seen[key] = string(v.Desc.FullName())
				ce.reasons = append(ce.reasons, r)

				d, ok := byDomain[dn]
				if !ok {
					d = &domain{name: dn}
					byDomain[dn] = d
					cat.domains = append(cat.domains, d)
				}
				d.reasons = append(d.reasons, r)
			}
			cf.enums = append(cf.enums, ce)
		}
		if len(cf.enums) > 0 {
			cat.files = append(cat.files, cf)
		}
	}
	return cat, nil
}

func loadReason(domain string, v *protogen.EnumValue, codes *codeTable) (*reason, error) {
	vo, _ := proto.GetExtension(v.Desc.Options(), herodotpb.E_Value).(*herodotpb.ValueOptions)
	if vo.GetCode() != nil {
		return nil, fmt.Errorf("(ory.herodot.v1.value).code is only for the ory.herodot.v1.Code enum")
	}
	spec := vo.GetReason()
	if v.Desc.Number() == 0 {
		if spec != nil {
			return nil, fmt.Errorf("the zero value can't be a reason")
		}
		return nil, nil
	}
	if spec == nil {
		return nil, fmt.Errorf("missing (ory.herodot.v1.value).reason")
	}

	name := string(v.Desc.Name())
	if len(name) > maxReasonLength || !reasonPattern.MatchString(name) {
		return nil, fmt.Errorf("invalid reason %q: want UPPER_SNAKE_CASE of at most %d characters", name, maxReasonLength)
	}
	if spec.GetCode() == herodotpb.Code_CODE_UNSPECIFIED {
		return nil, fmt.Errorf("missing code")
	}
	code, ok := codes.byNumber[int32(spec.GetCode())]
	if !ok {
		return nil, fmt.Errorf("unknown code %d", spec.GetCode())
	}
	if err := validateMessage(spec.GetMessage()); err != nil {
		return nil, err
	}
	keys := map[string]bool{}
	for _, k := range spec.GetMetadataKeys() {
		if !metadataKeyPattern.MatchString(k) {
			return nil, fmt.Errorf("metadata key %q is not lowerCamelCase", k)
		}
		if keys[k] {
			return nil, fmt.Errorf("duplicate metadata key %q", k)
		}
		keys[k] = true
	}

	opts, _ := v.Desc.Options().(*descriptorpb.EnumValueOptions)
	return &reason{
		domain:       domain,
		name:         name,
		code:         code,
		message:      spec.GetMessage(),
		metadataKeys: spec.GetMetadataKeys(),
		deprecated:   opts.GetDeprecated(),
		comment:      plainComment(v.Comments.Leading.String()),
		value:        v,
	}, nil
}

func allEnums(f *protogen.File) []*protogen.Enum {
	enums := append([]*protogen.Enum(nil), f.Enums...)
	var walk func([]*protogen.Message)
	walk = func(msgs []*protogen.Message) {
		for _, m := range msgs {
			enums = append(enums, m.Enums...)
			walk(m.Messages)
		}
	}
	walk(f.Messages)
	return enums
}

// camelCase turns UPPER_SNAKE_CASE into CamelCase.
func camelCase(s string) string {
	var b strings.Builder
	for _, part := range strings.Split(s, "_") {
		if part == "" {
			continue
		}
		b.WriteString(part[:1])
		b.WriteString(strings.ToLower(part[1:]))
	}
	return b.String()
}

// validateMessage checks that a message is one or more full English sentences
// on one line, as AIP-193 and AEP-193 use them.
func validateMessage(msg string) error {
	switch first, _ := utf8.DecodeRuneInString(msg); {
	case strings.TrimSpace(msg) == "":
		return fmt.Errorf("missing message")
	case msg != strings.TrimSpace(msg) || strings.ContainsAny(msg, "\r\n"):
		return fmt.Errorf("message %q: want a single line without surrounding whitespace", msg)
	case !unicode.IsUpper(first):
		return fmt.Errorf("message %q: want a full sentence starting with a capital letter", msg)
	case !strings.HasSuffix(msg, "."):
		return fmt.Errorf("message %q: want a full sentence ending with a period", msg)
	}
	return nil
}

// trimComment returns protogen's "// ..." comment lines without the trailing
// newline.
func trimComment(s string) string {
	return strings.TrimRight(s, "\n")
}

// plainComment strips the "//" markers from protogen comment lines.
func plainComment(s string) string {
	var lines []string
	for _, l := range strings.Split(trimComment(s), "\n") {
		l = strings.TrimPrefix(l, "//")
		lines = append(lines, strings.TrimPrefix(l, " "))
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// validateGoNames checks the package scope shared by all generated Go files.
func (cat *catalogs) validateGoNames() error {
	type key struct {
		pkg  protogen.GoImportPath
		name string
	}
	seen := map[key]*reason{}
	for _, f := range cat.files {
		for _, e := range f.enums {
			for _, r := range e.reasons {
				k := key{f.file.GoImportPath, r.goName()}
				if other, ok := seen[k]; ok {
					return fmt.Errorf("Go identifier %s in package %s collides between %s and %s",
						k.name, k.pkg, other.value.Desc.FullName(), r.value.Desc.FullName())
				}
				seen[k] = r
			}
		}
	}
	return nil
}
