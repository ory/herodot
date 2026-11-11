#!/usr/bin/env bash
# Releases herodot/v2, herodot v1, or both, from the current commit.
#
# For a herodot/v2 release, the root module is bumped to require it in the
# same commit that gets tagged: the go.sum lines are computed from the commit's
# v2 directory before the tag exists, exactly as sum.golang.org will record
# them. Every herodot/v2 release bumps the root module, whether or not it needs
# anything from that release.
#
# Without --push, this creates the commit and tags locally and prints what it
# would push. With --push, it pushes master and the tags atomically: either all
# of them move, or none do.
#
# Usage: scripts/release.sh [--v2 v2.x.y] [--v1 v0.x.y] [--push]
set -euo pipefail

module=github.com/ory/herodot/v2
v2="" v1="" push=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --v2) v2="${2:?--v2 needs a version}"; shift 2 ;;
    --v1) v1="${2:?--v1 needs a version}"; shift 2 ;;
    --push) push=true; shift ;;
    *) echo "usage: $0 [--v2 v2.x.y] [--v1 v0.x.y] [--push]" >&2; exit 2 ;;
  esac
done

fail() { echo "error: $*" >&2; exit 1; }

[[ -n "$v2$v1" ]] || fail "nothing to release; pass --v2, --v1 or both"
[[ -z "$v2" || "$v2" =~ ^v2\.[0-9]+\.[0-9]+$ ]] || fail "$v2 is not a herodot/v2 release version (v2.x.y)"
[[ -z "$v1" || "$v1" =~ ^v0\.[0-9]+\.[0-9]+$ ]] || fail "$v1 is not a herodot v1 release version (v0.x.y)"

cd "$(dirname "$0")/.."

git diff --quiet && git diff --cached --quiet || fail "the working tree has uncommitted changes"

# A new version must not exist and must be newer than every release of its
# major version.
check_new() {
  local version="$1" series="$2" latest
  git rev-parse -q --verify "refs/tags/$version" >/dev/null && fail "tag $version already exists"
  latest="$(git tag --list "$series" | grep -Ev -- '-' | sort -V | tail -n 1 || true)"
  if [[ -n "$latest" && "$(printf '%s\n%s\n' "$latest" "$version" | sort -V | tail -n 1)" != "$version" ]]; then
    fail "$version is not newer than the latest release $latest"
  fi
}
[[ -z "$v2" ]] || check_new "$v2" 'v2.*'
[[ -z "$v1" ]] || check_new "$v1" 'v0.*'

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

expected=""
if [[ -n "$v2" ]]; then
  echo "Computing $module $v2 from $(git rev-parse --short HEAD)."
  (cd v2/tools && GOWORK=off go run ./cmd/modproxy \
    -module "$module" -version "$v2" -repo ../.. -rev HEAD -subdir v2 -out "$tmp/proxy")

  # Serve the unreleased version from the local proxy. It isn't in the checksum
  # database yet, so skip the lookup for this module only. A fresh module cache
  # makes sure no stale copy of the version is used.
  GOPROXY="file://$tmp/proxy,$(go env GOPROXY)"
  GONOSUMDB="$module${GONOSUMDB:+,$GONOSUMDB}"
  GOMODCACHE="$tmp/modcache"
  GOFLAGS="-modcacherw${GOFLAGS:+ $GOFLAGS}"
  export GOPROXY GONOSUMDB GOMODCACHE GOFLAGS

  GOWORK=off go get "$module@$v2"
  GOWORK=off go mod tidy

  # The workspace only replaces v2 while the required version isn't tagged.
  go work edit -json | jq -r --arg m "$module" \
    '.Replace[]? | select(.Old.Path == $m) | "-dropreplace=\(.Old.Path)\(if .Old.Version then "@" + .Old.Version else "" end)"' |
    xargs -r go work edit

  expected="$v2"
fi

scripts/check-herodot-v2.sh $expected

if ! git diff --quiet; then
  git add go.mod go.sum go.work
  git commit -q -m "chore: release $(printf '%s' "$v2 $v1" | xargs | sed 's/ / and /')"
  echo "Committed $(git log -1 --format='%h %s')."
fi

tags=()
for version in $v2 $v1; do
  git tag "$version"
  tags+=("$version")
done

if [[ "$push" == true ]]; then
  git push --atomic origin HEAD:master "${tags[@]}"
  echo "Released ${tags[*]}."
else
  echo "Dry run. To publish, run: git push --atomic origin HEAD:master ${tags[*]}"
fi
