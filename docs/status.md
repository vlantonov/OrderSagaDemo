# Project Status — OrderSagaDemo

**Version:** 0.2.0  
**Date:** 2026-09-06  
**Current stage:** Maintenance — CI lint-regression fix landed on `main`; docs updated (release of `v0.2.1` deferred)

---

## SDLC Stage Completion

| Stage | Status | Artefact |
|-------|--------|---------|
| Requirements | ✅ Complete | `docs/requirements/SRS.md` v0.1.0 |
| Design | ✅ Complete | `docs/design/architecture.md` v0.1.0, `docs/design/project-layout.md` v0.1.0, `docs/tech-stack.md` v0.1.0 |
| Implementation | ✅ Complete | Go 1.22 monorepo — three services, shared telemetry/messaging, gRPC stubs, Dockerfiles, Compose, Helm, Grafana dashboards |
| QA | ✅ **PASS (with notes)** — D-3 accepted risk + one tracked test gap (see below) | Unit tests race-detector clean; golangci-lint PASS (re-fixed 2026-09-06); buf lint PASS; govulncheck non-blocking |
| Release / CI-CD | ✅ Complete | `.github/workflows/ci.yml`, `.github/workflows/release.yml`; images pushed to GHCR on `v*.*.*` tags |
| Documentation | ✅ Complete | `README.md`, `CHANGELOG.md`, `docs/ci-cd/pipeline.md`, `docs/status.md` (this file) |

---

## Maintenance Pass — CI Lint-Regression Fix (2026-09-06)

A `semver(patch)` maintenance fix landed on `main` (commit `e5106c9`) restoring a green CI `lint` job. `VERSION` remains `0.2.0`; the `v0.2.1` bump + tag + push is deferred to a separate release step. See the `[Unreleased]` **Fixed** / **Changed** entries in `CHANGELOG.md` for full detail.

- **golangci-lint regression resolved — lint is green again.**
  - `goimports` (`-local github.com/vladiant/ordersagademo`) / `gofmt` violations corrected in `internal/inventory/store/store.go`, `internal/messaging/envelope.go`, `internal/order/saga/orchestrator.go`, `internal/inventory/grpc/server.go` (formatting only, no behavioural change).
  - **gosec G115 (integer overflow `int → int32`)** in `buildReserveRequest` (saga reserve step): order-item quantities are now bounds-checked to `[0, math.MaxInt32]` before the `int32` conversion. An out-of-range quantity returns an error and drives saga compensation (already-taken payment is released), consistent with the existing reservation-failure path. A justified `//nolint:gosec` is retained on the guarded conversion because gosec v2.20 (pinned via golangci-lint v1.60.x) performs no flow analysis and cannot see the preceding guard.
- **CI action version bumps** — `actions/checkout@v4 → v5` and `actions/setup-go@v5 → v6` in `.github/workflows/ci.yml` and `.github/workflows/release.yml`, clearing the "Node 20 is being deprecated" warning on those steps.

**Tracked follow-ups (not lost):**

1. **Non-blocking QA follow-up (test-only):** add a unit test covering the out-of-range-quantity compensation branch in `buildReserveRequest` (error → compensation path). QA verdict is **PASS-with-notes** on account of this gap.
2. **Deferred stack decision — for System Architect:** fully clearing the Node-20 deprecation warning on the `lint` job requires bumping `golangci/golangci-lint-action` to v7/v8, which in turn requires golangci-lint `v1.60.x → v2` plus a `.golangci.yml` schema migration — a tech-stack change. Currently deferred; the action is left at `v6`.

---

## QA Verdict

**PASS (with notes)** — All blocking quality gates pass:

- `go build ./...` — clean
- `go vet ./...` — clean
- `go test -race -count=1 ./...` — clean (race detector enabled)
- `golangci-lint run` — PASS (regressed and re-fixed in the 2026-09-06 maintenance pass above; now green)
- `buf lint` — PASS

*Note:* one non-blocking test gap is tracked — the out-of-range-quantity compensation branch in `buildReserveRequest` is not yet unit-tested (see Maintenance Pass follow-up 1).

**D-3 accepted risk** — `govulncheck` reports CVEs in transitive dependencies:

| Advisory | Dependency | Decision |
|---------|-----------|---------|
| GO-2026-6061 | `google.golang.org/grpc v1.65.0` | Accepted — upstream fix pending; out of scope for this iteration |
| GO-2026-5426 | `go.opentelemetry.io/otel/sdk v1.29.0` | Accepted — upstream fix pending; out of scope for this iteration |
| GO-2026-\* (various) | Go 1.22.2 stdlib | Accepted — toolchain upgrade deferred |

`govulncheck` runs in CI with `continue-on-error: true` so findings remain visible without blocking merges.

---

## How to Verify the Demo

Prerequisites: Docker + Docker Compose 24+.

```bash
# 1. Start the full stack
make up

# 2. Happy path (order reaches COMPLETED, ReserveInventory succeeds)
bash scripts/create-order.sh

# 3. Compensation path (inventory forced to fail, payment refunded, order COMPENSATED)
bash scripts/create-order-fail.sh

# 4. Open Grafana — http://localhost:3000
#    - Saga Overview dashboard: order states and compensation counter
#    - SLO dashboard: success rate
#    - Explore → Tempo: find the trace for the compensation run; click a span to jump to Loki logs

# 5. Tear down
make down
```

Full observability walkthrough is in the [README](../README.md#forced-failure--compensation-demo).

---

## Design-Doc Drift — Resolved (2026-09-06)

All previously flagged stale references have been corrected by the Technical Writer (PM-authorized accuracy fix):

| File | Location | Was | Now |
|------|---------|-----|-----|
| `docs/design/architecture.md` | OQ-1 | `Go 1.23` | `Go 1.22` |
| `docs/design/architecture.md` | Dockerfile stage comment | `golang:1.23-alpine` | `golang:1.22-alpine` |
| `docs/design/project-layout.md` | `go.mod` snippet | `go 1.23` | `go 1.22` |
| `docs/design/architecture.md` | Component diagram edge | `"remote_write / scrape :8889"` | `"scrape :8889"` |
| `docs/tech-stack.md` | gRPC library version | `v1.64.x` | `v1.65.0` |

---

## Next Steps

1. **Release step** — bump `VERSION` to `0.2.1`, tag `v0.2.1`, and push to trigger the release workflow (deferred from the 2026-09-06 maintenance pass; `VERSION` currently stays `0.2.0`).
2. **QA follow-up (test-only)** — add the missing unit test for the out-of-range-quantity compensation branch in `buildReserveRequest` (Maintenance Pass follow-up 1).
3. **System Architect decision** — evaluate the golangci-lint `v1.60.x → v2` + `.golangci.yml` migration needed to bump `golangci/golangci-lint-action` to v7/v8 and clear the remaining Node-20 warning on the `lint` job (Maintenance Pass follow-up 2).
4. **Optional** — upgrade `grpc` and `otel/sdk` once upstream patches are available; re-enable blocking `govulncheck` in CI.
