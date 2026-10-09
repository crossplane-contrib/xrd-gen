# Contributing to xrd-gen

## Getting Started

**Prerequisites:**
- Go 1.27+
- Docker
- [Earthly](https://earthly.dev/get-earthly)

**Development Setup:**
1. Fork and clone the repo
2. Build: `earthly +build`, which writes `bin/xrd-gen` and `_output/bin/<os>_<arch>/xrd-gen`
3. Run xrd-gen locally: `./bin/xrd-gen go:xrd --input-path=./path/to/go/types --output-path=./path/to/output`

Earthly runs every step in a container, so the Go, linter and Crossplane versions
match what CI uses. They are declared as `ARG --global` values at the top of the
`Earthfile`.

## Testing

Unit tests live alongside the implementations. The integration tests under
`integrationtest/` generate code for a dummy Go project and check the result,
and some of them install Crossplane into a throwaway k3d cluster.

```shell
# Tidy, lint, unit and integration tests, and the dependency licence check
earthly +reviewable

# Unit tests plus the integration tests that only need the binary
earthly +test

# The integration tests that stand up a k3d cluster and install Crossplane.
# -P (--allow-privileged) is required for Docker-in-Docker
earthly -P +e2e

# Against a specific Crossplane version
earthly -P +e2e --CROSSPLANE_VERSION=2.3.5
```

Use the testify library for tests, specifically the `require` package, and
`assert.AnError` for any mock or stub errors.

Every dependency linked into the released binary has to carry a licence on the
allow-list in the `+go-licenses-check` target. `earthly +licenses` checks this,
and CI fails if a new dependency falls outside it.

## PR Process

1. Rebase on `upstream/main`
2. Run `earthly +reviewable` and `earthly -P +e2e`
3. Open a PR with a clear description, issue references, and testing notes
4. Address review feedback promptly

**Commit format:**
```
<type>: <short description>

<longer description if needed>

Fixes #issue-number
```
Types: `feat`, `fix`, `docs`, `test`, `refactor`, `chore`

New source files need the Apache-2.0 header. The `goheader` linter checks this
as part of `earthly +lint`; copy the block from any existing file.

## Getting Help

Reach out to a maintainer listed in [OWNERS.md](OWNERS.md).
