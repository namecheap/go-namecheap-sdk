#!/usr/bin/env bash
# Decides whether release-please should run for the current HEAD.
#
# release-please proposes a patch release for ANY conventional commit, so a
# lone docs:/chore:/ci:/test:/refactor: commit on master would open a release
# PR that ships no change to consumers. This script looks at the commits since
# the last release (the version in .release-please-manifest.json) and answers
# "true" only when at least one of them is releasing:
#   - type feat, fix, perf or revert (any scope), so fix(docs): counts;
#   - any type marked breaking with "!" or a "BREAKING CHANGE:" footer;
#   - a "Release-As:" footer;
#   - the merge commit of a release PR itself, because that run creates the tag.
#
# Usage: has-releasing-commits.sh [manifest-path]   (prints true|false)
set -euo pipefail

manifest="${1:-.release-please-manifest.json}"
last="v$(jq -r '."."' "$manifest")"

if ! git rev-parse -q --verify "refs/tags/$last^{commit}" >/dev/null; then
  echo "last release tag $last not found; letting release-please decide" >&2
  echo true
  exit 0
fi

if git log -1 --format=%s HEAD | grep -Eq '^chore\(master\): release '; then
  echo "HEAD is a release PR merge; release-please must cut the tag" >&2
  echo true
  exit 0
fi

range="$last..HEAD"
subjects="$(git log --format=%s "$range")"
bodies="$(git log --format=%B "$range")"

if grep -Eq '^(feat|fix|perf|revert)(\([^)]*\))?!?:' <<<"$subjects" \
  || grep -Eq '^[a-z]+(\([^)]*\))?!:' <<<"$subjects" \
  || grep -Eq '^(BREAKING CHANGE|Release-As):' <<<"$bodies"; then
  echo "releasing commits found since $last" >&2
  echo true
else
  echo "no releasing commits since $last; skipping release-please" >&2
  echo false
fi
