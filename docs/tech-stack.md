# Tech Stack — OrderSagaDemo

**Version:** 0.5.0  
**Date:** 2026-09-06  
**Owner:** System Architect  

---

## 1. Primary Language & Runtime

| Item | Choice | Rationale |
|---|---|---|
| Language | **Go 1.25** | First-class gRPC + Kafka + OTel SDK support; produces small static binaries ideal for distroless images; straightforward goroutine concurrency for saga handlers; fast builds; wide portfolio-reviewer familiarity. **Bumped Go 1.22 → 1.25 (ADR-003)** as the root-cause fix for the stdlib CVEs surfaced by `govulncheck`; also retires the D-3 toolchain deferral and removes the Go-version obstacle to ADR-001 (linter v2) and ADR-002 (`govulncheck@latest`). Target: the latest **Go 1.25.x** patch — must be **≥ go1.25.13** to include the html/template, crypto/tls, net/http and encoding/asn1 fixes; `go.mod` declares `go 1.25`. Implementer confirms the exact patch at build time. |
| Module path | `github.com/vladiant/ordersagademo` | Matches the GitHub namespace; single Go module at repo root. |
| Module layout | Single `go.mod` at repo root | All three services share generated proto code in `internal/gen/`; reduces toolchain ceremony for a demo. |

---

## 2. Build System & Dependency Management

| Item | Choice | Version / Notes |
|---|---|---|
| Build orchestration | **GNU Make** | Single `Makefile` at repo root with targets: `build`, `test`, `lint`, `proto`, `docker-build`, `compose-up`, `kind-deploy`. |
| Go dependency manager | **Go modules** (`go mod`) | Built into Go 1.25; `go.sum` committed. |
| Proto toolchain | **buf** | `v1.35.x`; handles `buf lint`, `buf generate`, and breaking-change detection. |
| Proto plugins (buf-managed) | `protoc-gen-go` v1.34, `protoc-gen-go-grpc` v1.4 | Declared in `buf.gen.yaml`; no system-level `protoc` required. |
| Linter | **golangci-lint** | **Active pin: `v2.13.2`** (config schema **v2**) via **`golangci-lint-action@v9`** (target `v9.3.0`); config in `.golangci.yml` at repo root. Enabled linters (unchanged set of six): `errcheck`, `govet`, `staticcheck`, `goimports`, `revive`, `gosec`. In the v2 schema `goimports` moves to the `formatters` section and keeps `local-prefixes: github.com/vladiant/ordersagademo`. **Migrated v1.60.x → v2 (ADR-001, executed early — see §12): golangci-lint v1.x is built with an older Go and cannot run under Go 1.25, so the ADR-003 Go bump forced the migration.** `golangci-lint-action@v9` is Node-24-native, which also clears the lingering Node-20 deprecation warning (the ADR-001 co-benefit — note `v7`/`v8` require golangci-lint v2 but are still Node-20, so **v9** is required to clear the warning). |
| Security scan | **govulncheck** | **`golang.org/x/vuln/cmd/govulncheck@latest`** via `go install …@latest`; run in `make vuln`. **Restored to `@latest` under the Go 1.25 toolchain — ADR-003 supersedes ADR-002:** with Go ≥ 1.25 the current `x/vuln` line installs cleanly, so the temporary `v1.1.4` pin (a Go-1.22-era stopgap) is retired. Expectation after the ADR-003 implementation pass lands: `govulncheck ./...` reports **no called vulnerabilities**. |
| Formatter | `gofmt` / `goimports` | Enforced by golangci-lint; no separate step needed. |

---

## 3. gRPC

| Item | Choice | Version |
|---|---|---|
| gRPC library | `google.golang.org/grpc` | `v1.82.1` (bumped from `v1.65.0` — clears GO-2026-6061; ADR-003) |
| Protobuf runtime | `google.golang.org/protobuf` | `v1.34.x` |
| OTel gRPC interceptors | `go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc` | contrib release paired with OTel `v1.44.0` — **`v0.69.0`** (from the coordinated contrib release `v1.44.0/v2.5.1/v0.69.0/…`; implementer confirms via `go get` + `go mod tidy`) |

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

**Bumped OTel `v1.29.0 → v1.44.0` (ADR-003, amended)** — originally `v1.43.0` to clear GO-2026-5426; **advanced one minor to `v1.44.0`** to also clear the last called vulnerability **GO-2026-5158** (baggage-header length cap in `go.opentelemetry.io/otel`, reached via `messaging.ExtractTrace` → `propagation.Baggage.Extract`; fixed in otel `v1.44.0`). OTel ships its `v1.x` (stable) and `v0.x` (log/experimental) modules as a **coordinated batch** — this bump uses the release group `v1.44.0/v0.20.0` (stable/log) with contrib `v0.69.0`; still supports Go 1.25 (`v1.46.0` is the last release to do so), so it does **not** cascade into a further toolchain change. The implementer confirms via `go get` + `go mod tidy`.

| Package | Version |
|---|---|
| `go.opentelemetry.io/otel` | `v1.44.0` |
| `go.opentelemetry.io/otel/sdk` | `v1.44.0` |
| `go.opentelemetry.io/otel/log` | `v0.20.0` |
| `go.opentelemetry.io/otel/sdk/log` | `v0.20.0` |
| `go.opentelemetry.io/otel/metric` | `v1.44.0` |
| `go.opentelemetry.io/otel/trace` | `v1.44.0` |
| `go.opentelemetry.io/otel/sdk/metric` | `v1.44.0` |
| `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc` | `v1.44.0` |
| `go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc` | `v1.44.0` |
| `go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc` | `v0.20.0` |
| `go.opentelemetry.io/contrib/propagators/b3` | `v1.44.0` (optional) |
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
| Build | `golang:1.25-alpine` | Go compiler + Alpine tools (bumped from `golang:1.22-alpine`; ADR-003) |
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

**Date:** 2026-09-06 **Owner:** System Architect **Status:** ✅ **Done** (2026-09-06 — executed early, triggered by the ADR-003 Go 1.25 bump)

> **Outcome (as-shipped target).** Migrated to **golangci-lint `v2.13.2`** (config schema v2) with **`golangci-lint-action@v9`** (target `v9.3.0`). **Trigger fired early:** golangci-lint v1.x is built with an older Go (v1.60.x reports “built with go1.23”) and **cannot run under Go 1.25** — the ADR-003 toolchain bump made the `lint` CI job hard-fail on the v1 pin, so option 1b (“stay on a v1 patch built with Go ≥ 1.25”) is **not viable**: the v1 line is frozen (~`v1.64.x`) and no v1.x is built with Go ≥ 1.25 (verify with `golangci-lint version --debug` / the binary's `built with go…` line, or the v1.60.x error `the Go language version (go1.23) used to build golangci-lint is lower than the targeted Go version (1.25.0)`). **Action-version correction:** `golangci-lint-action@v7`/`@v8` require golangci-lint v2 but remain **Node-20**; the Node-24 runtime (which clears the lingering Node-20 deprecation warning) first ships in **`@v9`** — so v9 is required to realise the ADR-001 co-benefit. **gosec re-check:** the newer gosec bundled in golangci-lint v2.13.2 may change G115 behaviour; the implementer MUST re-run `golangci-lint run ./...` and check whether the justified `//nolint:gosec` on the bounds-checked `int → int32` in `buildReserveRequest` is still required before removing it — do not assume.

**Context.** CI emits `"Node 20 is being deprecated. This workflow is running with Node 24 by default."` from the `golangci/golangci-lint-action@v6` step. The 2026-09-06 maintenance pass moved `actions/checkout@v4→v5` and `actions/setup-go@v5→v6` (both Node-24-native) but intentionally left the linter action at `v6`, because clearing the warning on the `lint` job requires `golangci-lint-action@v7/v8`, and those majors **require golangci-lint v2**. golangci-lint v2 uses a **different `.golangci.yml` schema** and reorganises some linter settings, so this is a toolchain + config-migration change, not a drop-in bump — an architecture-owned decision.

**Decision.** **golangci-lint v2** (with `golangci-lint-action@v7` or `v8`, whichever is current at implementation time) is the **approved target** stack for linting. Adoption is **deferred**: the project stays on **golangci-lint v1.60.x + `golangci-lint-action@v6`** until the Node-20 removal is announced or imminent (i.e., while the runner default remains Node 24 and the message is a non-blocking deprecation notice).

**Rationale / trade-off.**
- The warning is a **deprecation notice, not a failure** — the `lint` job is green and already runs on Node 24. No current functional impact.
- The v1→v2 migration carries **real churn and risk** — a config schema rewrite plus re-verification that the six enabled linters behave equivalently, with the possibility of new/changed findings across the codebase — for **no reviewer-visible benefit today**.
- Node-20 removal is nonetheless **certain**, so pinning v2 as the authoritative target avoids re-litigating the decision and keeps the doc internally consistent. Alternative (A) *migrate now* was rejected as premature churn; alternative (C) *decline* was rejected because it loses the reasoning and leaves the pin non-authoritative.

**Trigger to execute.** Any of: (a) GitHub announces a removal date for Node-20 action support; (b) the runner default flips such that the deprecation becomes an error; (c) an unrelated need to adopt a v2-only linter feature.

**Migration scope (for the implementation pass applying this ruling):**
- **Config:** rewrite `.golangci.yml` to the **golangci-lint v2 schema** (v2 reorganises the top-level structure and moves several `linters-settings`; preserve `disable-all` + the same six enabled linters `errcheck`, `govet`, `staticcheck`, `goimports`, `revive`, `gosec`, and the `goimports.local-prefixes: github.com/vladiant/ordersagademo` setting under its v2 equivalent). Consider `golangci-lint migrate` if available to bootstrap the conversion, then hand-verify.
- **Toolchain pin:** update §2 of this doc to the concrete golangci-lint `v2.x` version and the chosen `golangci-lint-action@v7/v8`.
- **CI:** bump `golangci/golangci-lint-action@v6 → v7/v8` in `.github/workflows/ci.yml` (and `release.yml` if it runs lint); confirm the Node-20 warning is gone.
- **gosec re-check (verify, don't assume):** v2 bundles a newer gosec that *may* change G115 behaviour. Re-evaluate whether the justified `//nolint:gosec` on the bounds-checked `int → int32` conversion in `buildReserveRequest` (saga reserve step) can be removed, and re-triage any newly surfaced findings before enabling.
- **Acceptance criteria:** `make lint` (`golangci-lint run ./...`) passes clean on the v2 toolchain; no new suppressed findings introduced without written justification; the `lint` CI job runs without the Node-20 deprecation warning; §2 + this ADR updated to reflect the shipped versions and `Status: Done`.

### ADR-002 — Pin govulncheck to a Go-1.22-compatible release (CI `security`/vuln step failure)

**Date:** 2026-09-06 **Owner:** System Architect **Status:** ⚠️ **Superseded by ADR-003 (2026-09-06)** — the `v1.1.4` pin was a Go-1.22-era stopgap; under the Go 1.25 bump, govulncheck returns to `@latest`. The context/decision below is retained for the record.

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

### ADR-003 — Bump the Go toolchain to 1.25.x (root-cause fix for the expanded govulncheck findings; retires D-3, supersedes ADR-002, unblocks ADR-001)

**Date:** 2026-09-06 **Owner:** System Architect **Status:** ✅ Approved — **Adopt now** (implementation in a separate follow-up pass)

**Context.** A CI `govulncheck ./...` run now reports **35 vulnerabilities that the code actually calls**, from 2 modules + the Go standard library — far beyond the original three-item D-3 accepted risk. The majority are **stdlib** advisories fixed only in newer Go toolchains: GO-2026-6091 (html/template), GO-2026-6090 (crypto/tls), GO-2026-6089 (net/http), GO-2026-5972 (encoding/asn1) — all **go1.25.13**; GO-2026-5856 (crypto/tls) — **go1.25.12**; GO-2026-5039 (net/textproto) — **go1.25.11**; GO-2026-5037 (crypto/x509) — newer Go; GO-2025-3503 (net/http) — go1.23.7. Three are **third-party**: GO-2026-6061 (`grpc v1.65.0 → v1.82.1`), GO-2026-5426 (`otel/sdk v1.29.0 → v1.43.0`), GO-2025-3503 (`golang.org/x/net v0.28.0 → v0.36.0`). This is the **third consecutive CI issue rooted in the Go 1.22 pin + stale deps** (after the goimports/gosec lint fix and ADR-002's govulncheck pin). All three standing deferrals — D-3 (toolchain upgrade), ADR-001 (linter v2), ADR-002 (govulncheck pin) — converge on one question: **is the Go 1.22 pin still the right call?** The vuln gate is intentionally non-blocking (`continue-on-error: true`), so the workflow run is technically green today; this is a **risk-posture + reviewer-optics** decision, not a hard CI block.

**Decision.** **Bump the Go toolchain to the latest Go 1.25.x patch (must be ≥ go1.25.13).** This is the root fix: it clears every stdlib advisory in one move, paired with dependency bumps for the three third-party findings (`grpc → v1.82.1`, the OTel family `→ v1.43.0`, `golang.org/x/net → v0.36.0`). Consequences: **D-3 is retired** (the toolchain deferral is executed, not re-scoped); **ADR-002 is superseded** — govulncheck returns to `@latest`; **ADR-001 remains Approved–Scheduled but loses its Go-version obstacle** and may execute on its own schedule.

**Rationale / trade-off.**
- **Root cause, not symptom.** Three straight CI issues trace to Go 1.22 + stale deps. Pinning tooling around the pin (ADR-002) and re-scoping accepted risk (option A) treat symptoms; the bump removes the shared cause.
- **Portfolio optics / honesty.** Letting the accepted-risk list grow from **3 → 35** items while a one-line `go` directive fixes most of them reads as risk-avoidance, not engineering judgment. A portfolio should demonstrate a clean, deliberate toolchain upgrade.
- **Consolidation.** One decision retires D-3, supersedes ADR-002, and unblocks ADR-001 — collapsing three open deferrals instead of maintaining them.
- **Rejected — (A) Hold the line / expand D-3.** Cheapest and technically green (the vuln job is `continue-on-error`), and defensible: the findings are DoS / parsing-hardening class (not RCE) and the demo services aren't internet-exposed. Rejected because the accepted-risk list becomes large and keeps growing, and the fix cost is low relative to the posture cost.
- **Rejected — (C) Partial (bump only the third-party deps).** Shrinks the list but leaves the stdlib majority outstanding (they need the Go bump anyway) — pays most of (B)'s blast radius without the payoff of a clean scan.

**Blast radius / coupled changes (follow-up implementation pass — do NOT implement here):**
- **`go.mod`** — `go 1.22` → `go 1.25` (optionally add `toolchain go1.25.13`); run `go mod tidy`.
- **`.github/workflows/ci.yml` + `release.yml`** — `actions/setup-go` `go-version: "1.22"` → the target 1.25.x patch, across all jobs (`build-test`, `lint`, `vuln`, and release build/lint).
- **`services/{order,payment,inventory}/Dockerfile`** — build stage `golang:1.22-alpine` → `golang:1.25-alpine`.
- **Dependency bumps** — `google.golang.org/grpc v1.65.0 → v1.82.1`; the OTel family `v1.29.0 → v1.43.0` (core / sdk / metric / trace / exporters) plus the coordinated `v0.x` log modules (`sdk/log`, `log`, `otlploggrpc`) and contrib (`otelgrpc`, `propagators/b3`) to their matching release — resolve via `go get` + `go mod tidy`; `golang.org/x/net v0.28.0 → v0.36.0`. Confirm grpc/otel still compile and don't cascade further.
- **`Makefile` + `.github/workflows/ci.yml` + `docs/ci-cd/pipeline.md`** — revert the govulncheck install from `@v1.1.4` to `@latest` (ADR-002 superseded). Optionally drop `continue-on-error` on the `vuln` job once the scan is clean.
- **Docs** — `docs/design/architecture.md` + `docs/design/project-layout.md` Go / Docker version references `1.22 → 1.25`.
- **Re-verify** — the six golangci-lint linters (`errcheck`, `govet`, `staticcheck`, `goimports`, `revive`, `gosec`) still pass; the justified `//nolint:gosec` in `buildReserveRequest` still applies; `go build`, `go vet`, `go test -race`, `buf lint` green; all three Docker images build.

**Acceptance criteria.** `govulncheck ./...` reports **no called vulnerabilities** (advisory scan clean); all CI gates green (`go build`, `go vet`, `go test -race`, `golangci-lint run`, `buf lint`); all three service images build; §1/§2/§3/§5/§7 of this doc and this ADR updated to the shipped versions with `Status: Done`.

**Follow-on to ADR-001 / ADR-002.** ADR-002 `Status` → **Superseded by ADR-003**. ADR-001 stays **Approved — Scheduled**; the Go-version obstacle is gone, so it may proceed on the next maintenance pass independently of this bump.

**Amendment (2026-09-06).**
- **OTel pin advanced `v1.43.0` → `v1.44.0`.** Implementation of the Go 1.25 bump left exactly **one** called vulnerability: **GO-2026-5158** in `go.opentelemetry.io/otel@v1.43.0` (baggage-header length cap, reached via `messaging.ExtractTrace` → `propagation.Baggage.Extract`), fixed in **otel `v1.44.0`**. Since this pass already performs a coordinated OTel bump, the family is advanced one minor to the release group **`v1.44.0`** (stable) / **`v0.20.0`** (`log`, `sdk/log`, `otlploggrpc`) / contrib **`v0.69.0`** (`otelgrpc`) / **`v1.44.0`** (`propagators/b3`). Verified clean: `v1.44.0` is a normal coordinated minor and **still supports Go 1.25** (`v1.46.0` is the last release to do so; `v1.47.0-rc` drops it), so it does not cascade into a further toolchain change. See §3/§5 for the exact set. **Result:** `govulncheck ./...` reaches **zero called vulnerabilities**, so the implementer may **drop `continue-on-error` on the `vuln` job**, making the security gate blocking.
- **ADR-001 executed early (linter).** golangci-lint v1.60.x cannot run under Go 1.25, which would turn this bump into a blocking `lint` failure. ADR-001 is therefore **executed now** (Status → Done): golangci-lint **`v2.13.2`** + **`golangci-lint-action@v9` (`v9.3.0`)**, `.golangci.yml` rewritten to the v2 schema (same six linters). See §2 and the ADR-001 outcome note above.
