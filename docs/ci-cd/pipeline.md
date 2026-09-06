# CI / CD Pipeline — OrderSagaDemo

**Version:** 0.1.0  
**Date:** 2026-09-06  
**Source files:** `.github/workflows/ci.yml`, `.github/workflows/release.yml`, `Makefile`

---

## Overview

Two GitHub Actions workflows handle quality control and image publishing:

| Workflow | File | Trigger |
|----------|------|---------|
| CI | `.github/workflows/ci.yml` | Push or PR to `main` |
| Release | `.github/workflows/release.yml` | Push of a `v*.*.*` version tag |

Both workflows use Go 1.22 (`actions/setup-go@v5`, `go-version: "1.22"`).

---

## CI Workflow (`ci.yml`)

The CI workflow runs four independent jobs in parallel on `ubuntu-latest`.

### Job 1 — Build · Vet · Test (`build-test`)

| Step | Command |
|------|---------|
| Checkout | `actions/checkout@v4` |
| Setup Go 1.22 | `actions/setup-go@v5` with `cache: true` |
| Build | `go build ./...` |
| Vet | `go vet ./...` |
| Race-test | `go test -race -count=1 ./...` |

**Local equivalent:** `make ci`

The `make ci` target runs all four steps in sequence and is the recommended command before opening a pull request:

```bash
make ci
```

### Job 2 — golangci-lint (`lint`)

Uses `golangci/golangci-lint-action@v6` pinned to `v1.60.3` (matching `docs/tech-stack.md`). Configuration is in `.golangci.yml` at the repo root. Enabled linters include `errcheck`, `govet`, `staticcheck`, `goimports`, `revive`, and `gosec`.

**Local equivalent:**

```bash
make lint
```

### Job 3 — govulncheck (`vuln`) — Non-blocking

Runs `govulncheck ./...` to scan for known CVEs in the dependency graph. The job has `continue-on-error: true`, which means it **never blocks merges**. The scan runs so that findings remain visible in the GitHub Checks UI and CI logs.

**Accepted-risk policy (D-3):** The demo has transitive CVEs in third-party dependencies (see [Known Limitations](../../README.md#known-limitations--accepted-risks) in the README). These are accepted out-of-scope for this portfolio iteration. Remove `continue-on-error: true` once upstream packages ship fixes or dependencies are upgraded.

**Local equivalent:**

```bash
make vuln
# govulncheck is installed via: go install golang.org/x/vuln/cmd/govulncheck@v1.1.4
# Pinned to v1.1.4 (ADR-002): last x/vuln release requiring <= Go 1.22.
```

### Job 4 — buf lint (`proto-lint`)

Uses `bufbuild/buf-setup-action@v1` pinned to `v1.35.0`. Runs `buf lint` in the `proto/` directory, enforcing the proto style rules defined in `proto/buf.yaml`.

**Local equivalent:**

```bash
# Requires buf CLI (see Prerequisites in README)
buf lint   # run from the proto/ directory
# or via make:
make generate   # runs buf generate; buf lint is run by the CI job separately
```

---

## Release Workflow (`release.yml`)

Triggered by pushing a tag matching `v*.*.*` (e.g., `v0.1.0`). Runs two jobs sequentially.

### Job 1 — Helm lint & template (`helm-validate`)

Validates the Helm chart before any image work. Fails the release if the chart is malformed.

| Step | Command |
|------|---------|
| `helm lint` | `helm lint deploy/helm/ordersagademo` |
| `helm template` | `helm template ordersagademo deploy/helm/ordersagademo` |

**Local equivalents:**

```bash
make helm-lint
make helm-template
```

### Job 2 — Build & push images (`docker-build-push`)

Runs as a matrix over `[order, payment, inventory]` after `helm-validate` passes. Each matrix leg:

1. Checks out the repo (the **repo root is the Docker build context** — required because `go.mod` and `internal/gen/` are shared across services).
2. Logs in to GHCR via `docker/login-action@v3` — **only when `github.repository == 'vladiant/ordersagademo'`** (fork builds skip the login and do a local build instead).
3. Builds and pushes with `docker/build-push-action@v6`:
   - `context: .` (repo root)
   - `file: services/<service>/Dockerfile`
   - Push guarded by the same canonical-repo condition.

#### Image naming / tagging

Each image is pushed with two tags:

```
ghcr.io/vladiant/ordersagademo-<service>:<git-tag>   # e.g. v0.1.0
ghcr.io/vladiant/ordersagademo-<service>:latest
```

Where `<service>` is one of `order`, `payment`, or `inventory`.

**Local equivalent** (builds but does not push):

```bash
# Build with default dev tag
make docker-build

# Build with a specific tag
make docker-build IMAGE_TAG=v0.1.0
```

---

## Running the Full Stack Locally

```bash
# Start all services + Kafka + observability stack
make up

# Tail logs (optional)
docker compose -f deploy/docker-compose/docker-compose.yml logs -f

# Run happy-path demo
bash scripts/create-order.sh

# Run compensation demo
bash scripts/create-order-fail.sh

# Stop and clean up (removes volumes)
make down
```

The compose file builds service images from the repo root before starting. No separate `make docker-build` step is needed for local development.

---

## Reproducibility Checklist

| Goal | Command |
|------|---------|
| Reproduce the full CI gate locally | `make ci` |
| Check for CVEs | `make vuln` |
| Validate Helm chart | `make helm-lint && make helm-template` |
| Build service images locally | `make docker-build` |
| Re-generate gRPC stubs | `make generate` |
| Start the demo stack | `make up` |
| Tear down | `make down` |

All `make` targets are defined in the `Makefile` at the repo root. No target invokes internet-dependent tooling at runtime; `make vuln` installs `govulncheck` via `go install` on first run.

---

## govulncheck Accepted-Risk Policy

`govulncheck` findings are **not release-blocking** for this portfolio demo. The CI job is marked `continue-on-error: true` and the `make vuln` target is documented as non-blocking. Findings are recorded in [CHANGELOG.md](../../CHANGELOG.md) under "Security — Accepted Risk (D-3)" and in the README Known Limitations section.

To re-evaluate the risk, run `make vuln` and review the output against the upstream package changelogs. Remove `continue-on-error: true` from `ci.yml` once the dependencies ship fixes.
