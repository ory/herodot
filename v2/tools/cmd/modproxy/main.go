// Copyright © 2026 Ory Corp
// SPDX-License-Identifier: Apache-2.0

// Command modproxy writes a module version, built from a git revision, into a
// directory that the go command can use as a module proxy (GOPROXY=file://...).
//
// The zip is built the way the go command builds it from a version control
// tag, so the go.sum lines it produces equal those that sum.golang.org records
// once the tag is pushed. This lets a release commit require a version before
// it is tagged.
//
// Usage:
//
//	modproxy -module github.com/ory/herodot/v2 -version v2.1.0 \
//		-repo . -rev HEAD -subdir v2 -out /tmp/proxy
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/mod/module"
	modzip "golang.org/x/mod/zip"
)

func main() {
	var (
		modPath = flag.String("module", "", "module path")
		version = flag.String("version", "", "version to publish, e.g. v2.1.0")
		repo    = flag.String("repo", ".", "git repository root")
		rev     = flag.String("rev", "HEAD", "git revision to build the module from")
		subdir  = flag.String("subdir", "", "module directory relative to the repository root")
		out     = flag.String("out", "", "proxy directory to write to")
	)
	flag.Parse()
	if err := run(*modPath, *version, *repo, *rev, *subdir, *out); err != nil {
		fmt.Fprintln(os.Stderr, "modproxy:", err)
		os.Exit(1)
	}
}

func run(modPath, version, repo, rev, subdir, out string) error {
	if modPath == "" || version == "" || out == "" {
		return fmt.Errorf("-module, -version and -out are required")
	}
	m := module.Version{Path: modPath, Version: version}
	if err := module.Check(m.Path, m.Version); err != nil {
		return err
	}
	repo, err := filepath.Abs(repo)
	if err != nil {
		return err
	}

	escPath, err := module.EscapePath(m.Path)
	if err != nil {
		return err
	}
	escVersion, err := module.EscapeVersion(m.Version)
	if err != nil {
		return err
	}
	dir := filepath.Join(out, filepath.FromSlash(escPath), "@v")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	gomod, err := git(repo, "show", rev+":"+path.Join(subdir, "go.mod"))
	if err != nil {
		return err
	}
	if !strings.Contains(string(gomod), "module "+m.Path) {
		return fmt.Errorf("%s/go.mod at %s doesn't declare module %s", subdir, rev, m.Path)
	}
	if err := os.WriteFile(filepath.Join(dir, escVersion+".mod"), gomod, 0o644); err != nil {
		return err
	}

	zf, err := os.Create(filepath.Join(dir, escVersion+".zip"))
	if err != nil {
		return err
	}
	if err := modzip.CreateFromVCS(zf, m, repo, rev, subdir); err != nil {
		_ = zf.Close()
		return err
	}
	if err := zf.Close(); err != nil {
		return err
	}

	info, err := json.Marshal(struct {
		Version string
		Time    time.Time
	}{m.Version, time.Now().UTC()})
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, escVersion+".info"), info, 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "list"), []byte(m.Version+"\n"), 0o644)
}

func git(repo string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return out, nil
}
