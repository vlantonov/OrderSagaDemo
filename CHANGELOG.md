# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.1] - 2026-09-06

### Fixed

- **CI `lint` job (golangci-lint) now passes on `main`.**
  - `goimports` (`-local github.com/vladiant/ordersagademo`) / `gofmt` violations corrected in `internal/inventory/store/store.go`, `internal/messaging/envelope.go`, `internal/order/saga/orchestrator.go`, and `internal/inventory/grpc/server.go` (import grouping, struct-field alignment, and a trailing-comment reformat). No behavioural change.
  - **gosec G115 (integer overflow `int → int32`)** in the saga reserve step (`buildReserveRequest`): order-item quantities are now bounds-checked to `[0, math.MaxInt32]` before the `int32` conversion. An out-of-range quantity now returns an error and drives saga compensation (payment already taken), consistent with the existing reservation-failure path. The `//nolint:gosec` originally retained on the guarded conversion was **removed** after the golangci-lint v2 migration — v2's newer gosec recognises the preceding range guard and no longer raises G115 (verified: an unguarded `int32(x)` is still flagged, the guarded conversion is not); the runtime bounds check remains the real fix.

### Changed

- **Go toolchain bumped 1.22 → 1.25** (`go.mod` `go 1.25`; built/verified with `go1.25.13`) as the root-cause fix for the expanded `govulncheck` findings (ADR-003; retires the D-3 toolchain deferral and supersedes ADR-002). Coordinated dependency upgrades: `google.golang.org/grpc v1.65.0 → v1.82.1`, the OpenTelemetry family `v1.29.0 → v1.44.0` (core/sdk/metric/trace/exporters, the paired `v0.20.0` log SDK/exporter, and `contrib` `otelgrpc v0.54.0 → v0.69.0`), and `golang.org/x/net v0.28.0 → v0.58.0` (transitively, ≥ the v0.36.0 fix). No application-code API changes were required — the codebase already used the current `grpc.NewClient` and `otelgrpc.New{Client,Server}Handler` idioms. Updated `actions/setup-go` `go-version` to `1.25.13` across all CI jobs and the three service Dockerfiles' builder image `golang:1.22-alpine → golang:1.25-alpine`. The `govulncheck` install runs `@latest` under the Go 1.25 toolchain (superseding ADR-002's temporary Go-1.22-era pin).
- **OpenTelemetry advanced `v1.43.0 → v1.44.0`** (ADR-003 amendment) to clear the final called vulnerability **GO-2026-5158** (baggage-header length cap in `go.opentelemetry.io/otel`, reached via `messaging.ExtractTrace`). Coordinated set: stable modules `→ v1.44.0`, the log modules (`log`, `sdk/log`, `otlploggrpc`) `→ v0.20.0`, and `contrib` `otelgrpc → v0.69.0`. `govulncheck ./...` now reports **zero called vulnerabilities**. No application-code changes required.
- **golangci-lint migrated v1.60.x → v2.13.2** (ADR-001, executed early — v1.x cannot run under Go 1.25). `.golangci.yml` rewritten to the **v2 schema** (`version: "2"`, `linters.default: none`, same six linters `errcheck`, `govet`, `staticcheck`, `revive`, `gosec` plus `goimports` moved to the new `formatters` section with `local-prefixes` preserved). The CI `lint` job bumps `golangci/golangci-lint-action@v6 → v9` with `version: v2.13.2`; the Node-24-native action **clears the lingering "Node 20 is being deprecated" warning** on the `lint` job. `golangci-lint run ./...` passes clean on the v2 toolchain under Go 1.25.
- **CI runner action versions bumped** — `actions/checkout@v4 → v5` and `actions/setup-go@v5 → v6` across `.github/workflows/ci.yml` and `release.yml` (both Node-24-native), clearing the Node-20 deprecation warning on those steps.
- **CI `vuln` job made blocking** — with `govulncheck ./...` now reporting zero called vulnerabilities, `continue-on-error: true` was dropped from the `vuln` job in `.github/workflows/ci.yml`.

### Security

- **D-3 accepted-risk findings cleared (ADR-003, amended).** The `govulncheck ./...` scan dropped from **35 called vulnerabilities** (2 modules + Go stdlib) to **zero**: the Go 1.25.13 toolchain clears every stdlib advisory (GO-2026-6091/6090/6089/5972/5856/5039/5037, GO-2025-3503), the dependency upgrades resolve the third-party findings **GO-2026-6061** (grpc) and **GO-2026-5426** (otel/sdk), and the OTel `v1.44.0` bump clears the last advisory **GO-2026-5158** (`go.opentelemetry.io/otel`, baggage-header length cap). With the scan clean, the CI `vuln` job is now blocking (`continue-on-error` dropped). The previously accepted D-3 risk (grpc/otel/stdlib advisories deferred behind the Go 1.22 pin) is therefore **retired** — no called vulnerabilities remain.

## [0.2.0] - 2026-09-06

### Added

#### Application Services

- **Order Service** — HTTP `POST /orders` entry point; `GET /healthz` liveness probe; in-memory order store; saga state machine (`PENDING → AWAITING_PAYMENT → RESERVING → COMPLETED` / `COMPENSATING → COMPENSATED`).
- **Payment Service** — Kafka worker consuming `OrderCreated` and `CompensatePayment`; publishing `PaymentProcessed`, `PaymentFailed`, `PaymentRefunded`; in-memory ledger with event-level idempotency.
- **Inventory Service** — gRPC server (`ReserveInventory`, `ReleaseInventory`); forced-failure injection via `INVENTORY_FAIL_ITEM_PREFIX` env var; publishes `InventoryReserved` / `InventoryReleased` audit events to Kafka.

#### Saga & Messaging

- Saga **orchestration** model: Order Service owns the state machine and drives every step and compensation centrally (no separate orchestrator binary).
- Seven Kafka topics following the `saga.<domain>.<event>` naming convention:
  - `saga.orders.created`, `saga.payments.processed`, `saga.payments.failed`
  - `saga.payments.compensate`, `saga.payments.refunded`
  - `saga.inventory.reserved`, `saga.inventory.released`
- Topics auto-created on first produce (`KAFKA_AUTO_CREATE_TOPICS_ENABLE=true`, KRaft, no Zookeeper).
- JSON event serialisation with a shared envelope (event type, UUID idempotency key, RFC 3339 timestamp, W3C `traceparent`).
- W3C TraceContext propagation across Kafka message headers and gRPC interceptors — single end-to-end `trace_id` per saga run.

#### Observability

- OpenTelemetry SDK (traces, metrics, structured logs) on all three services; OTLP/gRPC export to a shared OTel Collector.
- OTel Collector fans out: metrics → Prometheus (scrape `:8889`), traces → Tempo (OTLP/gRPC), logs → Loki (HTTP push). No remote-write path.
- Grafana dashboards provisioned from repo: **Saga Overview** and **SLO dashboard**.
- Prometheus alerting rules (`observability/prometheus/rules/saga-alerts.yaml`).
- Trace-to-log correlation enabled in Grafana (Tempo → Loki by `trace_id`).
- Saga metrics: `saga_orders_total`, `saga_step_duration_seconds`, `saga_compensation_total`.

#### Infrastructure & Packaging

- Multi-stage Dockerfiles per service — builder: `golang:1.22-alpine`; runtime: `gcr.io/distroless/static-debian12`.
- Docker Compose full-stack file (`deploy/docker-compose/docker-compose.yml`) — Kafka KRaft, OTel Collector, Prometheus, Loki, Tempo, Grafana, three services; health-checks on all containers.
- Plain Kubernetes manifests (`deploy/kubernetes/`) — Namespace, Deployment, Service, ConfigMap per service.
- Helm chart `ordersagademo` (`deploy/helm/ordersagademo/`) — targets Kubernetes ≥ 1.28; configurable image tags, replica counts, Kafka address.

#### Tooling & CI/CD

- `proto/inventory/v1/inventory.proto` — gRPC contract; buf-managed code generation to `internal/gen/`.
- `Makefile` — targets: `build`, `test`, `vet`, `lint`, `ci`, `vuln`, `generate`, `docker-build`, `helm-lint`, `helm-template`, `up`, `down`.
- GitHub Actions **CI workflow** (`.github/workflows/ci.yml`): build · vet · race-test · golangci-lint v1.60.x · govulncheck (non-blocking) · buf lint; triggers on push/PR to `main`.
- GitHub Actions **release workflow** (`.github/workflows/release.yml`): Helm lint & template, then build and push three service images to GHCR (`ghcr.io/vladiant/ordersagademo-{order,payment,inventory}`) on `v*.*.*` tags.
- Unit tests for saga orchestrator, HTTP handler, payment ledger, OTel propagation helpers — race-detector clean.
- Demo scripts: `scripts/create-order.sh` (happy path) and `scripts/create-order-fail.sh` (compensation path).

#### Documentation

- `README.md` — project overview, architecture diagram, quick start, observability walkthrough, Kafka topic table, sequence diagrams, Make target reference, project structure, CI/CD summary, known limitations, SDLC stage notes.
- `CHANGELOG.md` (this file).
- `docs/ci-cd/pipeline.md` — detailed CI and release pipeline walkthrough.
- `docs/status.md` — project status and SDLC stage record.
- `docs/requirements/SRS.md` — functional and non-functional requirements, acceptance criteria.
- `docs/design/architecture.md` — component diagram, saga state machine, Kafka schemas, gRPC contract.
- `docs/design/project-layout.md` — authoritative directory tree.
- `docs/tech-stack.md` — language, framework, and image version pinning.

[Unreleased]: https://github.com/vladiant/ordersagademo/compare/v0.2.1...HEAD
[0.2.1]: https://github.com/vladiant/ordersagademo/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/vladiant/ordersagademo/releases/tag/v0.2.0
