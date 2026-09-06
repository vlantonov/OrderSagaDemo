# Tech Stack — OrderSagaDemo

**Version:** 0.3.0  
**Date:** 2026-09-06  
**Owner:** System Architect  

---

## 1. Primary Language & Runtime

| Item | Choice | Rationale |
|---|---|---|
| Language | **Go 1.22** | First-class gRPC + Kafka + OTel SDK support; produces small static binaries ideal for distroless images; straightforward goroutine concurrency for saga handlers; fast builds; wide portfolio-reviewer familiarity. (Pinned at 1.22 — the installed runtime is Go 1.22.2; `go.mod` declares `go 1.22`.) |
| Module path | `github.com/vladiant/ordersagademo` | Matches the GitHub namespace; single Go module at repo root. |
| Module layout | Single `go.mod` at repo root | All three services share generated proto code in `internal/gen/`; reduces toolchain ceremony for a demo. |

---

## 2. Build System & Dependency Management

| Item | Choice | Version / Notes |
|---|---|---|
| Build orchestration | **GNU Make** | Single `Makefile` at repo root with targets: `build`, `test`, `lint`, `proto`, `docker-build`, `compose-up`, `kind-deploy`. |
| Go dependency manager | **Go modules** (`go mod`) | Built into Go 1.22; `go.sum` committed. |
| Proto toolchain | **buf** | `v1.35.x`; handles `buf lint`, `buf generate`, and breaking-change detection. |
| Proto plugins (buf-managed) | `protoc-gen-go` v1.34, `protoc-gen-go-grpc` v1.4 | Declared in `buf.gen.yaml`; no system-level `protoc` required. |
| Linter | **golangci-lint** | **Active pin: `v1.60.x`** (config schema v1) via `golangci-lint-action@v6`; config in `.golangci.yml` at repo root. Enabled linters: `errcheck`, `govet`, `staticcheck`, `goimports`, `revive`, `gosec`. Migration to **golangci-lint v2** (+ `golangci-lint-action@v7/v8`) is **approved-but-scheduled** — see §12 ADR-001. |
| Security scan | **govulncheck** | **Pinned: `golang.org/x/vuln/cmd/govulncheck@v1.1.4`** — the last `x/vuln` release whose `go.mod` requires ≤ Go 1.22 — via `go install golang.org/x/vuln/cmd/govulncheck@v1.1.4`; run in `make vuln`. **Do NOT use `@latest`:** `x/vuln@v1.7.0` requires Go ≥ 1.25 and fails to install under the Go 1.22 toolchain (`GOTOOLCHAIN=local`). Pin verified/confirmed via the procedure in §12 ADR-002. |
| Formatter | `gofmt` / `goimports` | Enforced by golangci-lint; no separate step needed. |

---

## 3. gRPC

| Item | Choice | Version |
|---|---|---|
| gRPC library | `google.golang.org/grpc` | `v1.65.0` |
| Protobuf runtime | `google.golang.org/protobuf` | `v1.34.x` |
| OTel gRPC interceptors | `go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc` | `v0.54.0` |

The only `.proto` file is `proto/inventory/v1/inventory.proto`; generated Go stubs land in `internal/gen/inventory/v1/`.

---

## 4. Kafka Client

| Item | Choice | Version | Notes |
|---|---|---|---|
| Kafka client library | `github.com/segmentio/kafka-go` | `v0.4.x` | Pure Go, no CGo, no librdkafka; simplifies multi-stage Docker builds on Alpine/distroless. |
| Delivery semantics | At-least-once | — | Per assumption A-5 in SRS. |
| Serialisation | **JSON** | — | `encoding/json` from stdlib; no Schema Registry container needed. Event schemas are documented Go structs. |
| Consumer groups | Configurable via env var | — | `KAFKA_CONSUMER_GROUP` per service; never hardcoded (FR-9). |

---

## 5. OpenTelemetry SDK

All three signals (metrics, logs, traces) exported via OTLP/gRPC to the shared OTel Collector.

| Package | Version |
|---|---|
| `go.opentelemetry.io/otel` | `v1.29.0` |
| `go.opentelemetry.io/otel/sdk` | `v1.29.0` |
| `go.opentelemetry.io/otel/sdk/log` | `v0.5.0` |
| `go.opentelemetry.io/otel/sdk/metric` | `v1.29.0` |
| `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc` | `v1.29.0` |
| `go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc` | `v1.29.0` |
| `go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc` | `v0.5.0` |
| `go.opentelemetry.io/contrib/propagators/b3` | `v1.29.0` (optional) |
| W3C TraceContext propagator | included in `go.opentelemetry.io/otel` | Used for gRPC and Kafka header injection |

---

## 6. Observability Stack — Image Tags

All images are pinned to a specific tag. Pull happens at `docker compose build` / `docker compose up --pull always`; no internet access required at container start-up (NFR-12).

| Component | Image | Tag |
|---|---|---|
| **OTel Collector** | `otel/opentelemetry-collector-contrib` | `0.104.0` |
| **Prometheus** | `prom/prometheus` | `v2.53.1` |
| **Loki** | `grafana/loki` | `3.1.1` |
| **Tempo** | `grafana/tempo` | `2.5.0` |
| **Grafana** | `grafana/grafana` | `11.1.4` |
| **Kafka** | `confluentinc/cp-kafka` | `7.7.0` (KRaft mode, no Zookeeper) |

---

## 7. Container Base Images

| Stage | Image | Notes |
|---|---|---|
| Build | `golang:1.22-alpine` | Go compiler + Alpine tools |
| Runtime | `gcr.io/distroless/static-debian12` | No shell, no package manager; ~2 MB base; statically linked Go binary drops straight in |

---

## 8. Local Orchestration

| Item | Choice | Version |
|---|---|---|
| Docker Compose | Docker Compose v2 (plugin) | `>=2.27` (ships with Docker Desktop 4.30 / Docker Engine 26) |
| Compose file version | `3.9` syntax | Compatible with Compose v2 |

---

## 9. Kubernetes & Helm

| Item | Choice | Version / Notes |
|---|---|---|
| Local cluster | **kind** | `v0.24.0` |
| Kubernetes API target | `1.30` | `apiVersion: apps/v1` (Deployments), `v1` (Services, ConfigMaps, Secrets) |
| Helm | `v3.16.x` | CLI only; chart committed to `deploy/helm/ordersagademo/` |
| Helm chart distribution | **Repo-only** | Chart lives in the repo; no external chart registry or OCI push required (OQ-10). |
| `kubeVersion` constraint in `Chart.yaml` | `>=1.28.0` | Allows kind 0.22+ and minikube users on recent clusters |

---

## 10. Test Frameworks & Scope

**Scope for this iteration (OQ-9): Unit tests + Integration tests against the running Compose stack. No contract tests (buf lint covers the proto contract).**

| Layer | Tool | Notes |
|---|---|---|
| Unit | Go built-in `testing` | One `_test.go` per internal package; no external runtime required. |
| Assertions | `github.com/stretchr/testify` `v1.9.x` | `require` and `assert` packages; avoids verbose `if` checks. |
| Integration | `github.com/testcontainers/testcontainers-go` `v0.31.x` | Spins up Kafka + the three service containers; exercises happy path and compensation path via HTTP + Kafka consumer assertions. |
| Make target | `make test-unit` / `make test-integration` | Integration tests guarded by `//go:build integration` tag; not run by default. |

---

## 11. Required Make Targets (QA Contract)

The QA Engineer must be able to run all of the following without any setup beyond `go`, `docker`, `make`, `buf`, and `golangci-lint` being on `PATH`:

| Target | Command | What it does |
|---|---|---|
| `proto` | `make proto` | `buf generate`; regenerates `internal/gen/` |
| `build` | `make build` | `go build ./cmd/...` for all three services |
| `test-unit` | `make test-unit` | `go test ./...` (unit only) |
| `lint` | `make lint` | `golangci-lint run ./...` |
| `vuln` | `make vuln` | `govulncheck ./...` |
| `docker-build` | `make docker-build` | Builds all three service images |
| `compose-up` | `make compose-up` | `docker compose -f deploy/docker-compose/docker-compose.yml up -d` |
| `compose-down` | `make compose-down` | `docker compose … down -v` |
| `test-integration` | `make test-integration` | Runs integration test suite (requires Docker) |
| `kind-deploy` | `make kind-deploy` | Creates kind cluster, loads images, applies manifests |

---

## 12. Deferred / Pending Decisions (ADRs)

### ADR-001 — golangci-lint v1 → v2 migration (Node-20 deprecation on the `lint` job)

**Date:** 2026-09-06 **Owner:** System Architect **Status:** ✅ Approved — **Scheduled** (deferred)

**Context.** CI emits `"Node 20 is being deprecated. This workflow is running with Node 24 by default."` from the `golangci/golangci-lint-action@v6` step. The 2026-09-06 maintenance pass moved `actions/checkout@v4→v5` and `actions/setup-go@v5→v6` (both Node-24-native) but intentionally left the linter action at `v6`, because clearing the warning on the `lint` job requires `golangci-lint-action@v7/v8`, and those majors **require golangci-lint v2**. golangci-lint v2 uses a **different `.golangci.yml` schema** and reorganises some linter settings, so this is a toolchain + config-migration change, not a drop-in bump — an architecture-owned decision.

**Decision.** **golangci-lint v2** (with `golangci-lint-action@v7` or `v8`, whichever is current at implementation time) is the **approved target** stack for linting. Adoption is **deferred**: the project stays on **golangci-lint v1.60.x + `golangci-lint-action@v6`** until the Node-20 removal is announced or imminent (i.e., while the runner default remains Node 24 and the message is a non-blocking deprecation notice).

**Rationale / trade-off.**
- The warning is a **deprecation notice, not a failure** — the `lint` job is green and already runs on Node 24. No current functional impact.
- The v1→v2 migration carries **real churn and risk** — a config schema rewrite plus re-verification that the six enabled linters behave equivalently, with the possibility of new/changed findings across the codebase — for **no reviewer-visible benefit today**.
- Node-20 removal is nonetheless **certain**, so pinning v2 as the authoritative target avoids re-litigating the decision and keeps the doc internally consistent. Alternative (A) *migrate now* was rejected as premature churn; alternative (C) *decline* was rejected because it loses the reasoning and leaves the pin non-authoritative.

**Trigger to execute.** Any of: (a) GitHub announces a removal date for Node-20 action support; (b) the runner default flips such that the deprecation becomes an error; (c) an unrelated need to adopt a v2-only linter feature.

**Migration scope (for the future Maintenance/Developer pass — do NOT implement now):**
- **Config:** rewrite `.golangci.yml` to the **golangci-lint v2 schema** (v2 reorganises the top-level structure and moves several `linters-settings`; preserve `disable-all` + the same six enabled linters `errcheck`, `govet`, `staticcheck`, `goimports`, `revive`, `gosec`, and the `goimports.local-prefixes: github.com/vladiant/ordersagademo` setting under its v2 equivalent). Consider `golangci-lint migrate` if available to bootstrap the conversion, then hand-verify.
- **Toolchain pin:** update §2 of this doc to the concrete golangci-lint `v2.x` version and the chosen `golangci-lint-action@v7/v8`.
- **CI:** bump `golangci/golangci-lint-action@v6 → v7/v8` in `.github/workflows/ci.yml` (and `release.yml` if it runs lint); confirm the Node-20 warning is gone.
- **gosec re-check (verify, don't assume):** v2 bundles a newer gosec that *may* change G115 behaviour. Re-evaluate whether the justified `//nolint:gosec` on the bounds-checked `int → int32` conversion in `buildReserveRequest` (saga reserve step) can be removed, and re-triage any newly surfaced findings before enabling.
- **Acceptance criteria:** `make lint` (`golangci-lint run ./...`) passes clean on the v2 toolchain; no new suppressed findings introduced without written justification; the `lint` CI job runs without the Node-20 deprecation warning; §2 + this ADR updated to reflect the shipped versions and `Status: Done`.

### ADR-002 — Pin govulncheck to a Go-1.22-compatible release (CI `security`/vuln step failure)

**Date:** 2026-09-06 **Owner:** System Architect **Status:** ✅ Approved — **Adopt now**

**Context.** The CI `security`/vuln step (`go install golang.org/x/vuln/cmd/govulncheck@latest`) began failing:

```
golang.org/x/vuln/cmd/govulncheck@latest: golang.org/x/vuln@v1.7.0
requires go >= 1.25.0 (running go 1.22.12; GOTOOLCHAIN=local)
Error: Process completed with exit code 1
```

`x/vuln@v1.7.0` raised its own `go.mod` `go` directive to `1.25.0`. Because CI runs `GOTOOLCHAIN=local` (§1) it will **not** auto-download a newer toolchain, so the `go install` fails outright under Go 1.22. The floating `@latest` is the moving part that broke; §1 pins the runtime at **Go 1.22** and `docs/status.md` records a **D-3 accepted risk** deferring the Go 1.22.2 stdlib toolchain upgrade.

**Decision.** Replace the floating `@latest` with a **specific pinned version** of `golang.org/x/vuln/cmd/govulncheck` whose module `go.mod` requires **≤ Go 1.22**. The declared pin is **`v1.1.4`** (the last `v1.1.x` release, predating the `go ≥ 1.25` bump in the `v1.7.0` line). Go 1.22 (§1) is retained; the D-3 deferral is **left intact**.

**Rationale / trade-off.**
- **Root cause is the floating `@latest`, not Go 1.22.** Pinning CI tooling is standard practice; it makes the security gate reproducible and removes the moving target.
- **The vuln gate is advisory, not blocking** — it runs `continue-on-error: true` (see `docs/status.md`), so the acceptable cost of pinning (a periodically-stale scanner that must be manually rolled forward) does not gate merges or releases.
- **Honors D-3.** Option **(B) bump Go ≥ 1.25** was rejected: it would reverse an explicitly-deferred, accepted risk as a side effect of a tool-install error, force a full recompile/retest, require re-verifying the six linters and the justified gosec `//nolint` (ADR-001), and stack a second toolchain migration on top of the already-scheduled golangci-lint v1→v2 work — large blast radius for no reviewer-visible benefit today. A Go bump remains a valid *future* decision, but must be made deliberately (jointly retiring D-3), not here.
- Option **(C)** (make the step tolerant / pin *and* schedule a Go bump) was rejected as either redundant (the step is already `continue-on-error`) or as smuggling in the deferred Go decision.

**Verification the implementer MUST run before shipping the pin** (do not assume the version number):

```bash
go list -m -versions golang.org/x/vuln          # enumerate all published tags
# For each candidate, highest-first, inspect its go.mod 'go' directive:
go mod download golang.org/x/vuln@vX.Y.Z
awk '/^go /{print}' "$(go env GOMODCACHE)/cache/download/golang.org/x/vuln/@v/vX.Y.Z.mod"
# Pin the HIGHEST version whose 'go' directive is <= 1.22.
```

If `v1.1.4` is not the highest ≤ Go 1.22 release (or fails to install under Go 1.22), pin the verified highest compatible release instead and update the §2 Security-scan row to match.

**Trigger to revisit.** Any of: (a) the project performs the deferred Go toolchain bump (retiring D-3), at which point govulncheck may return to a current pinned release ≥ its new floor; (b) a critical advisory requires a scanner feature only present in a Go-1.25+ govulncheck; (c) the pinned release is retracted/yanked.

**Implementation scope (separate pass — do NOT implement here):**
- **`Makefile`** — `make vuln` target: change the govulncheck install from `@latest` to the pinned `@v1.1.4` (post-verification).
- **`.github/workflows/*.yml`** — the `go install golang.org/x/vuln/cmd/govulncheck@latest` line in the `security`/vuln step: change to the pinned version. Keep the step's `continue-on-error: true`.
- **`go.mod`** — **no change** (this ADR deliberately does *not* touch the Go version; that stays §1 / Go 1.22).
- **Acceptance criteria:** the CI `security`/vuln step **installs and runs cleanly under Go 1.22** (`GOTOOLCHAIN=local`); `make vuln` (`govulncheck ./...`) executes without the `requires go >= 1.25.0` install error; the D-3 accepted-risk findings continue to surface as advisory output; §2 + this ADR updated with the final verified version and `Status: Done`.
