# Project Status — OrderSagaDemo

**Version:** 0.1.0  
**Date:** 2026-09-06  
**Current stage:** Released candidate / Documentation complete

---

## SDLC Stage Completion

| Stage | Status | Artefact |
|-------|--------|---------|
| Requirements | ✅ Complete | `docs/requirements/SRS.md` v0.1.0 |
| Design | ✅ Complete | `docs/design/architecture.md` v0.1.0, `docs/design/project-layout.md` v0.1.0, `docs/tech-stack.md` v0.1.0 |
| Implementation | ✅ Complete | Go 1.22 monorepo — three services, shared telemetry/messaging, gRPC stubs, Dockerfiles, Compose, Helm, Grafana dashboards |
| QA | ✅ **PASS** (with D-3 accepted risk — see below) | Unit tests race-detector clean; golangci-lint PASS; buf lint PASS; govulncheck non-blocking |
| Release / CI-CD | ✅ Complete | `.github/workflows/ci.yml`, `.github/workflows/release.yml`; images pushed to GHCR on `v*.*.*` tags |
| Documentation | ✅ Complete | `README.md`, `CHANGELOG.md`, `docs/ci-cd/pipeline.md`, `docs/status.md` (this file) |

---

## QA Verdict

**PASS** — All blocking quality gates pass:

- `go build ./...` — clean
- `go vet ./...` — clean
- `go test -race -count=1 ./...` — clean (race detector enabled)
- `golangci-lint run` — PASS
- `buf lint` — PASS

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

1. **Project Manager** — bump `VERSION` to `0.1.0`, push `v0.1.0` git tag to trigger the release workflow and publish GHCR images.
2. **Optional** — upgrade `grpc` and `otel/sdk` once upstream patches are available; re-enable blocking `govulncheck` in CI.
