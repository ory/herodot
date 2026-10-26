#!/usr/bin/env bash
# Checks that the root module is ready for a release: it requires the latest
# herodot/v2 release, and its go.mod and go.sum are complete and build without
# the workspace, as they do for consumers.
#
# Usage: scripts/check-herodot-v2.sh [version]
#
# Without a version, the latest v2.x.y tag is expected, which needs the
# repository's tags (actions/checkout with fetch-depth: 0). scripts/release.sh
# passes the version it is about to tag.
set -euo pipefail

module=github.com/ory/herodot/v2

cd "$(dirname "$0")/.."
export GOWORK=off

expected="${1:-}"
if [[ -z "$expected" ]]; then
  expected="$(git tag --list 'v2.*' | grep -Ev -- '-' | sort -V | tail -n 1 || true)"
  [[ -n "$expected" ]] || { echo "error: no herodot/v2 release is tagged yet; release v2 first" >&2; exit 1; }
fi

required="$(go mod edit -json | jq -r --arg m "$module" '.Require[]? | select(.Path == $m) | .Version')"
if [[ "$required" != "$expected" ]]; then
  echo "error: the root module requires $module $required, but the latest release is $expected" >&2
  exit 1
fi

go mod tidy -diff
go build ./...
echo "The root module requires $module $expected and builds without the workspace."
