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
- release-please opens or updates a release PR only when at least one commit
  since the last tag falls into a visible changelog section
  (`changelog-sections` in `.release-please-config.json`: `feat`, `fix`,
  `perf`, `revert`). Otherwise it logs "No user facing commits found" and
  does nothing, which is what makes `docs:`, `chore:`, `ci:`, `test:` and
  `refactor:` non-releasing. Do not add `docs` (or any other non-releasing
  type) to that list: while it was listed, the docs-only #207 opened a
  release PR for a module with no code change (#208).
- release-please walks commits since the last tag, classifies them, and:
  - computes the next SemVer bump (`fix:` → patch, `feat:` → minor,
    `feat!:`/`BREAKING CHANGE:` → major),
  - updates `CHANGELOG.md` with a generated entry grouped by type,
  - updates `.release-please-manifest.json` with the new version,
  - opens (or updates) a PR titled `chore(master): release X.Y.Z`. The
    `otelnamecheap` module gets its own release PR; see
    [The `otelnamecheap` module](#the-otelnamecheap-module).
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

## Releasing and non-releasing commit types

Only `fix:`, `feat:`, `perf:`, `revert:` and breaking changes open or update
the release PR (see step 2 above). `docs:`, `chore:`, `ci:`, `test:`,
`refactor:` and `build:` never do and never appear in the changelog. Pick the
type by what reaches consumers: Go doc comments on exported symbols are read
on pkg.go.dev only at a new tag, so a change to them is `fix(docs):`;
repo-internal markdown (`README.md`, `CONTRIBUTING.md`, `RELEASE.md`,
`CLAUDE.md`, `docs/namecheap-api-v2.md`) is `docs:`.

## The `otelnamecheap` module

The nested module `github.com/namecheap/go-namecheap-sdk/otelnamecheap` is a
second release-please package with its own version line. It goes through the
same flow as the SDK, separately:

- **Its own release PR**, titled `chore(master): release otelnamecheap X.Y.Z`
  (`separate-pull-requests` keeps it apart from the SDK's release PR, so either
  can be merged without the other).
- **Its own changelog**, `otelnamecheap/CHANGELOG.md`.
- **Its own tags**, in the directory-prefixed form Go requires for a nested
  module: `otelnamecheap/vX.Y.Z` (`component` plus `tag-separator: "/"`).
  Install with `go get github.com/namecheap/go-namecheap-sdk/otelnamecheap@vX.Y.Z`.

Which package a commit releases depends on the files it touches. The SDK
package excludes `otelnamecheap/`, so a releasing commit that touches only
that directory releases only otelnamecheap. A commit that touches both modules,
such as a toolchain or CVE bump, releases both, and its PR title goes into both
changelogs. Keep such bumps in their own PR: squashed into a core change, the
core change's title becomes the otelnamecheap changelog entry (as #203 and #215
did). To correct an entry already merged, put the right message between
`BEGIN_COMMIT_OVERRIDE` and `END_COMMIT_OVERRIDE` in the merged PR's body;
release-please reads it in place of the commit message.

While the module is at `0.x`, `bump-minor-pre-major` turns a breaking change
into a minor bump. Moving to `v1.0.0` is a deliberate API-stability promise,
made by setting the version (`Release-As: 1.0.0` in a commit body), never by
accident.

The module's SDK requirement is not bumped by Dependabot (see
`.github/dependabot.yml`): it is a minimum, not a pin, so a stale value holds
no consumer back. Raise it by hand when the module needs a newer SDK API. CI
still tests the module against the SDK on every commit (`Test against the SDK
on this commit` in `ci.yml`), so a core change that breaks it fails before the
SDK is released.

release-please cannot set GitHub's `make_latest`, so a new otelnamecheap
release would take the repository's "Latest" label from the SDK.
`versioning.yml` hands it back to the SDK release after every otelnamecheap
release.

> Note: the historical tags `votelnamecheap/v0.1.1` and `votelnamecheap/v0.1.2`
> are malformed (extra leading `v`) and invisible to the Go module proxy. The
> versions before automation were tagged by hand as `otelnamecheap/v0.1.0` and
> `otelnamecheap/v0.1.3`; release-please starts from the `0.1.3` tag.

## Required configuration

| Name | Kind | Used by | Purpose |
|---|---|---|---|
| `APP_CLIENT_ID` | variable | `versioning.yml` | GitHub App client ID for release-please |
| `APP_PRIVATE_KEY` | secret | `versioning.yml` | GitHub App private key |

The org's release GitHub App must be installed on this repository (with
`contents: write` and `pull-requests: write`) for the above to work.

The flow also assumes these repository settings:

- **Squash merging only**, with the squash commit title taken from the **PR
  title** (`squash_merge_commit_title: PR_TITLE`) — otherwise a single-commit
  PR lands its commit subject, bypassing the title check. Rebase merging must
  stay disabled for the same reason.
- Squash commit **message** set to `PR_BODY` or blank — with
  `COMMIT_MESSAGES`, a stray `BREAKING CHANGE:` footer in an intermediate
  commit would trigger an unintended major bump.
- The **Conventional PR Title** check listed as a required status check, so a
  non-conforming title actually blocks merge.

## Manual / emergency release

If `master` is red for reasons unrelated to the code (for example a newly
disclosed CVE failing the Trivy scan), `versioning.yml` never auto-runs and
releases quietly stop. It can be started by hand via **workflow_dispatch**
(Actions → Versioning → Run workflow) — note this deliberately bypasses the
green-CI gate.

If release-please itself is unavailable, a release can be cut fully by hand:

1. Open a PR that bumps the version in `.release-please-manifest.json` and adds
   the corresponding entry to `CHANGELOG.md` (direct pushes to `master` are
   blocked by branch protection), and merge it.
2. Tag the merge commit:
   `git tag -a vX.Y.Z -m "Release vX.Y.Z" && git push origin vX.Y.Z`.
3. The next release-please run reconciles its state with the updated manifest.
