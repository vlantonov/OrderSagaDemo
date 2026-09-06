# Project Status — OrderSagaDemo

**Version:** 0.5.0  
**Date:** 2026-09-06  
**Current stage:** Documentation — the Go 1.22 → 1.25 toolchain upgrade (ADR-003) shipped: coordinated `grpc` / OTel / `golang.org/x/net` dependency bumps, golangci-lint v1.60.3 → v2.13.2 migration (`golangci-lint-action@v9`), and `govulncheck ./...` down to **0 called vulnerabilities** (CI `vuln` gate now blocking). QA verdict **PASS-with-notes**; **D-3 retired**, ADR-001 **Done**, ADR-002 **Superseded**, ADR-003 **Done (amended)**

---

## SDLC Stage Completion

| Stage | Status | Artefact |
|-------|--------|---------|
| Requirements | ✅ Complete | `docs/requirements/SRS.md` v0.1.0 |
| Design | ✅ Complete | `docs/design/architecture.md` v0.1.0, `docs/design/project-layout.md` v0.1.0, `docs/tech-stack.md` v0.5.0 (ADR-001 Done, ADR-002 Superseded, ADR-003 Done/amended in §12) |
| Implementation | ✅ Complete | Go 1.25 monorepo — three services, shared telemetry/messaging, gRPC stubs, Dockerfiles, Compose, Helm, Grafana dashboards; deps `grpc v1.82.1` / OTel `v1.44.0` family / `x/net v0.58.0`; golangci-lint `v2.13.2` (`golangci-lint-action@v9`) |
| QA | ✅ **PASS (with notes)** — gates green under Go 1.25 (0 called vulns, `vuln` gate blocking; lint green on v2.13.2); one tracked test gap (see below) | Unit tests race-detector clean; golangci-lint v2.13.2 PASS; buf lint PASS; `govulncheck ./...` **0 called vulnerabilities** (CI `vuln` job blocking) |
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
2. **Stack decision — RESOLVED (ADR-001 Done, 2026-09-06):** clearing the Node-20 warning on the `lint` job required golangci-lint `v1.60.x → v2` plus a `.golangci.yml` schema migration and `golangci-lint-action@v9`. Executed in the Go 1.25 upgrade pass below (forced early — golangci-lint v1.x cannot run under Go 1.25).

---

## Maintenance Pass — CI Vuln-Install Fix (2026-09-06)

A `semver(patch)` maintenance fix landed on `main` (commit `866f49e`) restoring a working CI `vuln` job. `VERSION` remains `0.2.0`; the `v0.2.1` bump + tag + push is still deferred to a separate release step. See the `[Unreleased]` **Fixed** entry in `CHANGELOG.md` and `docs/tech-stack.md` §12 (**ADR-002**) for full detail.

- **govulncheck install failure resolved — the `vuln` job installs cleanly under Go 1.22.**
  - The step used `go install golang.org/x/vuln/cmd/govulncheck@latest`, which resolved to `x/vuln v1.7.0`; that release raised its module `go` directive to `1.25.0`, so `go install` aborted with `requires go >= 1.25.0` under the pinned Go 1.22 toolchain (`GOTOOLCHAIN=local`).
  - **Fix (ADR-002, `docs/tech-stack.md` §12):** pin govulncheck to **`@v1.1.4`** — verified by both Maintenance and QA as the highest `x/vuln` release whose module `go` directive is `≤ 1.22` (`v1.2.0`–`v1.7.0` all declare `go 1.25.0`). Applied in `make vuln` (`Makefile`), the CI workflow (`.github/workflows/ci.yml`), and `docs/ci-cd/pipeline.md`. The vuln step stays non-blocking (`continue-on-error: true`).
  - **Go stays pinned at 1.22** — the D-3 toolchain-upgrade deferral is honoured; the fix pins the *tool*, not the *language*.

**Status:** resolved — no new open follow-up. QA verdict **PASS**; all regression gates green (`go build`, `go vet`, `go test -race`, `golangci-lint run`).

---

## Architecture Decision — Go 1.25 Toolchain Bump (2026-09-06)

A later CI `govulncheck ./...` run materially changed the accepted-risk picture: it reported **35 called vulnerabilities from 2 modules + the Go standard library**, up from the three D-3 items. Most are stdlib advisories fixed only in Go 1.25.x. This is the **third consecutive Go-1.22-rooted CI issue** in the session (after the lint fix and the ADR-002 govulncheck pin).

**Ruling (System Architect):** **Option (B) — bump the Go toolchain to the latest Go 1.25.x patch (≥ go1.25.13)** as the root-cause fix, paired with dependency bumps (`grpc → v1.82.1`, OTel family `→ v1.43.0`, `golang.org/x/net → v0.36.0`). Recorded as **ADR-003** in `docs/tech-stack.md` §12. This **retires D-3**, **supersedes ADR-002** (govulncheck reverts to `@latest`), and **unblocks ADR-001** (linter v2 loses its Go-version obstacle). Options (A) hold-the-line/expand-D-3 and (C) partial dep-only bump were rejected — see the ADR-003 trade-off section.

**This pass was decision + docs only** — the coupled source / `go.mod` / workflow changes were routed to a separate follow-up pass (ADR-003 scope). **That pass has since shipped — see the Maintenance Pass below.**

---

## Maintenance Pass — Go 1.25 Toolchain + Dependency & Linter Upgrade (2026-09-06)

The ADR-003 ruling was executed as a single coordinated upgrade pass. It passed QA (**PASS-with-notes**) and is documented here in the AS-BUILT state. `VERSION` remains `0.2.0`; the version bump + tag + push is deferred to a separate release step. See the `[Unreleased]` **Fixed** / **Changed** / **Security** entries in `CHANGELOG.md` and `docs/tech-stack.md` §12 (**ADR-003**, amended) for full detail.

- **Go toolchain bumped 1.22 → 1.25** — `go.mod` declares `go 1.25`; built/verified with `go1.25.13`. `actions/setup-go` `go-version` set to `1.25.13` across all CI jobs (`ci.yml`, `release.yml`); the three service Dockerfiles' builder image moved `golang:1.22-alpine → golang:1.25-alpine`. Root-cause fix for the expanded `govulncheck` findings (ADR-003).
- **Coordinated dependency upgrades** — `google.golang.org/grpc v1.65.0 → v1.82.1`; the OpenTelemetry family `v1.29.0 → v1.44.0` (core/sdk/metric/trace/exporters, the paired `v0.20.0` log modules, and `contrib` `otelgrpc v0.54.0 → v0.69.0`); `golang.org/x/net v0.28.0 → v0.58.0`; `testify → v1.11.1` (transitive). No application-code API changes were required — the codebase already used the current `grpc.NewClient` and `otelgrpc.New{Client,Server}Handler` idioms.
- **golangci-lint migrated v1.60.3 → v2.13.2 (ADR-001, executed early)** — golangci-lint v1.x cannot run under Go 1.25, so the migration was forced by the toolchain bump. `.golangci.yml` rewritten to the **v2 schema** (`version: "2"`, `linters.default: none`, same six linters `errcheck`/`govet`/`staticcheck`/`revive`/`gosec` plus `goimports` moved to the new `formatters` section with `local-prefixes` preserved). CI `golangci/golangci-lint-action@v6 → v9` (`v9.3.0`) — Node-24-native, which **clears the lingering Node-20 deprecation warning** on the `lint` job.
- **gosec `//nolint:gosec` (G115) removed** — the newer gosec bundled in golangci-lint v2 recognises the preceding `[0, math.MaxInt32]` bounds check in `buildReserveRequest`, so the suppression is no longer needed; the runtime bounds check itself is retained.
- **Security gate now blocking** — `govulncheck ./...` reports **0 called vulnerabilities** (down from 35); `continue-on-error: true` was dropped from the CI `vuln` job. The govulncheck install reverted to `@latest` (ADR-002 superseded — the `@v1.1.4` pin was a Go-1.22-era stopgap).

**ADR status after this pass:** ADR-001 **Done**, ADR-002 **Superseded by ADR-003**, ADR-003 **Done (amended)**. **D-3 retired** (0 called vulns).

**Tracked follow-up (not lost):** ~~the out-of-range-quantity compensation branch in `buildReserveRequest` (error → compensation path) is still not unit-tested.~~ **RESOLVED (2026-09-06)** — now covered by `TestCompensationOnInvalidQuantity` (commit `e9ede1f`); see the Maintenance Pass below. QA still notes several packages (`telemetry`, the inventory gRPC server, the Kafka consumers, `cmd/*`) lack direct unit coverage — a known coverage gap, non-blocking for this pass.

---

## Maintenance Pass — Compensation-Branch Test Added (2026-09-06)

A test-only change landed on `main` (commit `e9ede1f`): `TestCompensationOnInvalidQuantity` was added to `internal/order/saga/orchestrator_test.go`, covering the out-of-range-quantity → compensation branch in `buildReserveRequest` (error → compensation path). This closes the last tracked coverage-gap follow-up from the CI lint-regression fix. No source or behavioural change; `VERSION` unchanged. The broader multi-package coverage gap (`telemetry`, inventory gRPC server, Kafka consumers, `cmd/*`) remains open — see Next Steps.

---

## QA Verdict

**PASS (with notes)** — All blocking quality gates pass under Go 1.25:

- `go build ./...` — clean
- `go vet ./...` — clean
- `go test -race -count=1 ./...` — clean (race detector enabled)
- `golangci-lint run ./...` — PASS on golangci-lint **v2.13.2** (v2 schema; `golangci-lint-action@v9`)
- `buf lint` — PASS
- `govulncheck ./...` — **0 called vulnerabilities**; the CI `vuln` job is now **blocking** (`continue-on-error` removed)

*Note:* the out-of-range-quantity compensation branch in `buildReserveRequest` is now unit-tested (`TestCompensationOnInvalidQuantity`, commit `e9ede1f`, 2026-09-06). One coverage gap remains (non-blocking): several packages — `telemetry`, the inventory gRPC server, the Kafka consumers (`internal/order/kafka`, `internal/payment/kafka`), and `cmd/*/main.go` — remain without direct unit coverage (see Next Steps).

**D-3 — RESOLVED (ADR-003 implemented, 2026-09-06).**

The Go 1.22 → 1.25 toolchain bump plus the coordinated dependency upgrades (`grpc v1.65.0 → v1.82.1`, the OTel family `v1.29.0 → v1.44.0`, `golang.org/x/net v0.28.0 → v0.58.0`) cleared **all 35 previously-called vulnerabilities**; `govulncheck ./...` now reports **0 called vulnerabilities**. The CI `vuln` job is therefore **blocking** (`continue-on-error: true` removed) and acts as a real quality gate.

*Historical note:* D-3 was previously an **accepted risk** — the grpc/otel/stdlib advisories were knowingly deferred behind the Go 1.22 pin while `govulncheck` ran non-blocking. That risk is now **retired**: it no longer applies (0 called vulns). The final called advisory, GO-2026-5158 (`go.opentelemetry.io/otel` baggage-header length cap), was cleared by advancing OTel one minor to `v1.44.0` (ADR-003 amendment).

| Advisory (abridged) | Source | Fixed in | Cleared by |
|---|---|---|---|
| GO-2026-6091 / 6090 / 6089 / 5972 | stdlib (html/template, crypto/tls, net/http, encoding/asn1) | go1.25.13 | Go 1.25 bump |
| GO-2026-5856 | stdlib crypto/tls | go1.25.12 | Go 1.25 bump |
| GO-2026-5039 | stdlib net/textproto | go1.25.11 | Go 1.25 bump |
| GO-2026-5037 | stdlib crypto/x509 | newer Go | Go 1.25 bump |
| GO-2025-3503 | stdlib net/http + `golang.org/x/net v0.28.0` | go1.23.7 / x/net v0.36.0 | Go bump + `x/net v0.58.0` |
| GO-2026-6061 | `google.golang.org/grpc v1.65.0` | v1.82.1 | grpc bump |
| GO-2026-5426 | `go.opentelemetry.io/otel/sdk v1.29.0` | v1.43.0 | OTel bump |
| GO-2026-5158 | `go.opentelemetry.io/otel v1.43.0` | v1.44.0 | OTel v1.44.0 (ADR-003 amendment) |

All advisories above are now cleared; `govulncheck ./...` is clean and the CI gate is blocking.

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

1. **Release step** — bump `VERSION`, tag, and push to trigger the release workflow (deferred; `VERSION` currently stays `0.2.0`). The version bump for the Go 1.25 + dependency + linter upgrade is a separate release decision.
2. **Coverage gap (test-only, non-blocking)** — add direct unit coverage for `telemetry`, the inventory gRPC server, the Kafka consumers (`internal/order/kafka`, `internal/payment/kafka`), and `cmd/*/main.go` (compilation-only today), which QA flagged as untested.

**Resolved (no longer open):**

- **QA follow-up (test-only) — out-of-range-quantity compensation branch** — RESOLVED (2026-09-06). The error → compensation path in `buildReserveRequest` is now covered by `TestCompensationOnInvalidQuantity` in `internal/order/saga/orchestrator_test.go` (commit `e9ede1f`). This closes the specific coverage-gap follow-up tracked since the CI lint-regression fix; the broader multi-package coverage gap above remains open and distinct.
- **Go 1.25 bump (ADR-003)** — shipped; see the Maintenance Pass above. `govulncheck ./...` clean, all gates green, D-3 retired, ADR-002 superseded.
- **golangci-lint v1 → v2 (ADR-001)** — shipped (`v2.13.2` + `golangci-lint-action@v9`); the Node-20 deprecation warning on the `lint` job is cleared.
