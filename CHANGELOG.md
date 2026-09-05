# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

> This section will become **[0.1.0]** when the first version tag (`v0.1.0`) is pushed.

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
- GitHub Actions **CI workflow** (`.github/workflows/ci.yml`): build · vet · race-test · golangci-lint v1.60.3 · govulncheck (non-blocking, `continue-on-error: true`) · buf lint; triggers on push/PR to `main`.
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

### Security — Accepted Risk (D-3)

The following vulnerabilities are flagged by `govulncheck` and accepted as out-of-scope for this portfolio demo iteration. `govulncheck` runs in CI as a non-blocking job so findings remain visible.

- **GO-2026-6061** — `google.golang.org/grpc v1.65.0`; awaiting upstream patch release.
- **GO-2026-5426** — `go.opentelemetry.io/otel/sdk v1.29.0`; awaiting upstream patch release.
- **GO-2026-\*** — Go 1.22.2 stdlib CVEs; toolchain upgrade deferred to a future iteration.

[Unreleased]: https://github.com/vladiant/ordersagademo/compare/HEAD...HEAD
