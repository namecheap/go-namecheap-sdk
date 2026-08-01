# Release Process

This document describes how changes merged to `master` become published
versions of the SDK, installable via
`go get github.com/namecheap/go-namecheap-sdk/v2@vX.Y.Z`.

This is a Go library: a release is a SemVer git tag plus a GitHub release and a
changelog entry. There are no binaries to build, sign, or upload — the Go
module proxy serves the tag directly.

## Overview

The project uses a **semi-automated, maintainer-gated** release flow. Changes
merged to `master` do **not** ship immediately. They accumulate in a long-lived
"release PR" maintained by
[release-please](https://github.com/googleapis/release-please), and a version
is tagged only when a maintainer merges that PR.

Two workflows participate:

| Workflow | Trigger | Role |
|---|---|---|
| [`ci.yml`](.github/workflows/ci.yml) | push, PRs | Build, vet, lint, unit tests, race detector |
| [`versioning.yml`](.github/workflows/versioning.yml) | `CI` success on `master`, manual dispatch | Runs release-please to open/update the release PR and cut the tag |

Supporting configuration:

- [`pr-title.yml`](.github/workflows/pr-title.yml) — enforces
  [Conventional Commits](https://www.conventionalcommits.org/) on PR titles;
  release-please consumes these to compute version bumps.
- [`.release-please-config.json`](.release-please-config.json) /
  [`.release-please-manifest.json`](.release-please-manifest.json) —
  release-please configuration and state. The manifest is the source of truth
  for the current version.

## Step-by-step flow

### 1. PR is opened and merged

- PR title must follow Conventional Commits; `pr-title.yml` fails the PR
  otherwise.
- PRs are squash-merged, so the PR title becomes the commit subject that
  release-please classifies.
- **The PR *body* becomes the commit body, and release-please parses that too.**
  A body line that begins with an identifier immediately followed by `(` — a Go
  call at the start of a line in a fenced example, say — is read as a
  `type(scope)` header, and a nested parenthesis makes the whole commit
  unparseable:

  ```
  commit could not be parsed: 8f94f7a feat(dns): add WithEmailType ... (#161)
  error message: Error: unexpected token '(' at 27:53, valid tokens [)]
  Considering: 0 commits — No commits for path: ., skipping
  ```

  release-please then opens **no release PR at all** and the change silently does
  not ship. It happened to #161. Either indent such lines inside the fence so
  they do not start at column 0, or pass an explicit body when merging:

  ```shell
  gh pr merge <n> --squash --body "Short prose summary, no code."
  ```

- `ci.yml` runs on the PR and again on the resulting merge commit.

At this point nothing is released — the commit simply sits on `master`.

### 2. release-please opens or updates the release PR

- `versioning.yml` triggers on a successful `CI` run against `master`
  (`workflow_run` with `conclusion == 'success'`).
- release-please walks commits since the last tag, classifies them, and:
  - computes the next SemVer bump (`fix:` → patch, `feat:` → minor,
    `feat!:`/`BREAKING CHANGE:` → major),
  - updates `CHANGELOG.md` with a generated entry grouped by type,
  - updates `.release-please-manifest.json` with the new version,
  - opens (or updates) a PR titled `chore(master): release X.Y.Z`.
- Authentication uses a dedicated GitHub App (`APP_CLIENT_ID`,
  `APP_PRIVATE_KEY`), **not** the default `GITHUB_TOKEN`: events
  authored by `GITHUB_TOKEN` do not re-trigger workflows, so the release PR
  would never get CI runs.

The release PR is long-lived — as more PRs merge to `master`, release-please
keeps updating the same PR.

### 3. Maintainer cuts the release

A release happens only when a maintainer reviews the computed version bump and
changelog in the release PR and merges it. Merging the release PR:

- commits the version bump and regenerated `CHANGELOG.md` to `master`,
- creates the `vX.Y.Z` git tag and the GitHub release.

There is no fixed cadence — merge when there is enough change to justify a new
version.

### 4. Availability

Once the tag is pushed, the version is immediately installable:

```
go get github.com/namecheap/go-namecheap-sdk/v2@vX.Y.Z
```

The Go module proxy and pkg.go.dev pick up the tag automatically (the first
request for a new version warms the proxy cache).

## Versioning

- Current version lives in
  [`.release-please-manifest.json`](.release-please-manifest.json).
- [Semantic Versioning](https://semver.org/), derived automatically from
  Conventional Commit types since the previous tag.
- The module is at major version 2 (`/v2` import path suffix). A `feat!:` /
  `BREAKING CHANGE:` commit would bump to 3.0.0, which per Go's module rules
  also requires changing the module path to `/v3` — treat major bumps as a
  deliberate, coordinated change, never an accidental side effect of a commit
  message.

## Dependency bumps

Dependabot opens weekly PRs (see
[`.github/dependabot.yml`](.github/dependabot.yml)):

- **gomod** bumps use `fix(deps):` — a releasing type, because module bumps
  change what consumers `go get`. Merging one makes the release PR propose a
  patch release. Dependencies are vendored, so bump PRs also update `vendor/`.
- **github-actions** bumps use `ci(deps):` — non-releasing, CI-only.

## The `otelnamecheap` module

The nested module `github.com/namecheap/go-namecheap-sdk/otelnamecheap` is
**not** managed by release-please. It is released manually by tagging
`otelnamecheap/vX.Y.Z` (the directory-prefixed form Go requires for nested
modules):

```
git tag -a otelnamecheap/vX.Y.Z -m "otelnamecheap vX.Y.Z" && git push origin otelnamecheap/vX.Y.Z
```

> Note: the historical tag `votelnamecheap/v0.1.1` is malformed (extra leading
> `v`) and invisible to the Go module proxy — otelnamecheap v0.1.1 is
> effectively unreleased. The latest usable tag is `otelnamecheap/v0.1.0`.

## Required configuration

| Name | Kind | Used by | Purpose |
|---|---|---|---|
| `APP_CLIENT_ID` | variable | `versioning.yml` | GitHub App client ID for release-please |
| `APP_PRIVATE_KEY` | secret | `versioning.yml` | GitHub App private key |

The org's release GitHub App must be installed on this repository (with
`contents: write` and `pull-requests: write`) for the above to work.

## Manual / emergency release

Prefer the normal flow. In rare cases (release-please unavailable, out-of-band
hotfix), a release can be cut by hand:

1. Bump the version in `.release-please-manifest.json` and add the
   corresponding entry to `CHANGELOG.md`. Commit to `master`.
2. Tag the commit:
   `git tag -a vX.Y.Z -m "Release vX.Y.Z" && git push origin vX.Y.Z`.
3. The next release-please run reconciles its state with the updated manifest.
