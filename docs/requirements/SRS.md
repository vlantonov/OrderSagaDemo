# Software Requirements Specification — OrderSagaDemo

**Version:** 0.1.0  
**Date:** 2026-09-06  
**Status:** Draft — awaiting System Architect review  
**Audience:** System Architect, Developer, QA Engineer (SDLC pipeline)

---

## 1. Overview & Goals

### 1.1 Purpose

OrderSagaDemo is a portfolio project that demonstrates distributed-transaction coordination using the **Saga pattern** across multiple independent services. It is not a production system; its purpose is to be demoable, readable, and instructive to a technical reviewer.

### 1.2 What This Project Demonstrates

| Skill / Concept | Evidence Produced |
|---|---|
| Saga pattern with compensating transactions | Working happy-path and forced-failure runs |
| gRPC synchronous inter-service calls | `.proto` contract + generated stubs in use |
| Kafka event-driven choreography/orchestration | Producer/consumer code; visible event flow |
| Observability (metrics, logs, traces) | OTel-instrumented services; Grafana dashboards |
| Containerisation & Kubernetes deployment | Multi-stage Dockerfiles; Compose; Helm chart |
| SLO design & alerting | Dashboard + alert rules visible in Grafana |

### 1.3 Intended Audience

Primary: portfolio reviewers, hiring engineers, and technical interviewers evaluating distributed-systems and cloud-native skills.  
Secondary: the SDLC agent pipeline (System Architect → Developer → QA → Release).

---

## 2. Actors & Scope

### 2.1 Services (Actors)

| Service | Responsibility |
|---|---|
| **Order Service** | Entry point; accepts an order creation request; initiates the saga; emits `OrderCreated` event |
| **Payment Service** | Consumes `OrderCreated`; attempts charge; emits `PaymentProcessed` or `PaymentFailed` |
| **Inventory Service** | Exposes a gRPC endpoint for synchronous reservation; also participates in the event flow for release/rollback |

### 2.2 Saga Coordination

The project must implement a saga that spans all three services. Whether this is **choreography** (each service reacts to events), **orchestration** (a dedicated saga orchestrator drives the steps), or a hybrid is an **open question** for the System Architect (see Section 7).

### 2.3 Explicitly Out of Scope

- Real payment processing or financial compliance.
- User authentication, authorisation, or API gateway.
- Persistent production databases (any durable store used is for demo purposes only).
- Multi-tenancy or multi-region deployment.
- UI / frontend of any kind.
- Performance benchmarking or load testing.
- CI/CD pipeline automation (GitHub Actions workflows, etc.) — deferred to a later iteration.
- Service mesh (Istio, Linkerd) — deferred.

---

## 3. Functional Requirements

### Order Creation Flow

**FR-1** — The Order Service shall accept a request to create an order containing at minimum: a unique order identifier, a list of one or more items (each with item ID and quantity), and a payment amount.

**FR-2** — Upon receiving a valid order creation request, the Order Service shall persist the order in a `PENDING` state and publish an `OrderCreated` event to the Kafka event backbone before returning a response to the caller.

**FR-3** — The Order Service shall expose the order creation endpoint such that a caller can trigger it without any pre-existing runtime state (i.e., the endpoint must be exercisable via a single CLI command, script, or curl/grpcurl invocation in the demo).

### gRPC Synchronous Call — Inventory Reservation

**FR-4** — The Inventory Service shall expose at least one synchronous gRPC endpoint, `ReserveInventory`, defined in a `.proto` contract file checked into the repository. The contract shall specify the request message (minimum fields: order ID, list of item IDs with quantities) and the response message (minimum fields: success flag, reservation ID or error reason).

**FR-5** — The `ReserveInventory` RPC shall check whether sufficient stock exists for all requested items and, if so, atomically mark those units as reserved and return success; otherwise it shall return a structured failure response (not an unhandled error).

**FR-6** — The `.proto` file(s) shall be the single source of truth for the gRPC contract; no service shall hardcode field layouts that duplicate or diverge from the proto definition.

### Kafka Event Backbone

**FR-7** — The system shall use Kafka as the message broker for all asynchronous inter-service events. The following event types shall be defined with documented schemas (field names and types, independent of serialisation format):

| Event | Producer | Consumers |
|---|---|---|
| `OrderCreated` | Order Service | Payment Service (and optionally saga orchestrator) |
| `PaymentProcessed` | Payment Service | Inventory Service (and/or saga orchestrator) |
| `PaymentFailed` | Payment Service | Order Service (and/or saga orchestrator) |
| `InventoryReserved` | Inventory Service | Order Service (and/or saga orchestrator) |
| `InventoryReleased` | Inventory Service | Order Service (and/or saga orchestrator) |

**FR-8** — Each service that produces events shall use an idiomatic Kafka producer with at-least-once delivery semantics.

**FR-9** — Each service that consumes events shall use a Kafka consumer group; consumer group IDs shall be configurable, not hardcoded.

**FR-10** — The happy-path event sequence shall be demonstrable end-to-end: `OrderCreated` → `PaymentProcessed` → `InventoryReserved` → order marked `COMPLETED`.

### Compensating Transactions / Rollback

**FR-11** — The system shall implement compensating transactions for at least the following failure scenario: payment succeeds but inventory reservation subsequently fails (or is forced to fail).

**FR-12** — When a compensation is triggered, the system shall:
  - (a) publish a compensation event (e.g., `PaymentRefunded` or equivalent) causing any charged payment to be reversed, and  
  - (b) mark the order `FAILED` or `COMPENSATED`.

**FR-13** — Compensating actions shall be idempotent: re-delivering a compensation event shall not result in a double-refund or double-release.

### Forced-Failure Scenario

**FR-14** — The demo shall include a documented, reproducible mechanism to inject a failure that triggers the compensation path (e.g., a configuration flag, a special item ID, or an injected fault in the Inventory Service) without requiring source-code modification at demo time.

**FR-15** — During a forced-failure run, structured log output and/or trace data shall clearly indicate: which step failed, which compensating steps were executed, and the final order state.

---

## 4. Non-Functional Requirements

### Observability — OpenTelemetry Instrumentation

**NFR-1** — Every service shall be instrumented with an OpenTelemetry (OTel) SDK, emitting all three observability signals: **metrics**, **structured logs**, and **distributed traces**.

**NFR-2** — All three signals shall share a common `trace_id` / `span_id` correlation so that a single failing request can be navigated from a Grafana metric alert → trace → correlated log lines without leaving the Grafana UI.

**NFR-3** — Distributed trace context shall be propagated across both the gRPC synchronous call (FR-4 / FR-5) and across Kafka message headers, so that a single end-to-end saga run appears as one connected trace.

### Metrics & Alerting

**NFR-4** — Prometheus shall scrape metrics from all services. At minimum the following metric categories shall be present:
  - Request/event processing rate and error rate per service.
  - Saga step duration (latency histogram).
  - Compensation event counter (to make rollbacks visible in dashboards).

**NFR-5** — At least one Prometheus alerting rule shall be defined and committed to the repository, covering a meaningful failure condition (e.g., compensation rate > 0 for a sustained period, or payment failure rate above a threshold).

**NFR-6** — At least one SLO dashboard shall be provided in Grafana, demonstrating an error-budget or success-rate view for the order creation flow.

### Logs

**NFR-7** — Loki shall be the log aggregation backend. All services shall ship structured logs (key-value or JSON format) to Loki.

**NFR-8** — Log entries for saga state transitions (order created, payment processed, inventory reserved, compensation triggered) shall include the `trace_id` as a structured field.

### Traces

**NFR-9** — Tempo shall be the distributed tracing backend. Each service-to-service interaction (gRPC call, Kafka produce/consume) shall produce a child span under the root saga trace.

**NFR-10** — The Grafana instance shall be configured to enable trace-to-log correlation: clicking a span in Tempo shall navigate to the correlated Loki log lines.

### Containerisation

**NFR-11** — Each service shall have a multi-stage Dockerfile that produces a minimal runtime image (build artefacts separated from the final image layer).

**NFR-12** — All images shall be buildable on a developer workstation with a single documented command and shall not require internet access at container start-up time (dependencies baked into the image).

### Local Orchestration

**NFR-13** — A Docker Compose file shall bring up the entire system (all three services + Kafka + observability stack: Prometheus, Loki, Tempo, Grafana) with a single `docker compose up` command.

**NFR-14** — The Docker Compose configuration shall include health-check definitions for Kafka and all application services, so that dependent services wait for their dependencies to be ready.

### Kubernetes Deployment

**NFR-15** — Plain Kubernetes YAML manifests shall be provided for each service (Deployment, Service, ConfigMap at minimum).

**NFR-16** — A Helm chart shall be provided that wraps the Kubernetes manifests, with values for at minimum: image tags, replica counts, and Kafka bootstrap server address.

**NFR-17** — The Helm chart and plain manifests shall target a standard Kubernetes API version compatible with a recent stable release (exact version is an open question — see Section 7).

### Documentation

**NFR-18** — A sequence diagram depicting the **happy path** (order created → payment processed → inventory reserved → order completed) shall be committed to the repository as a text-based diagram (e.g., Mermaid or PlantUML) and rendered in the README.

**NFR-19** — A sequence diagram depicting the **compensation path** (forced failure + rollback steps) shall be committed alongside the happy-path diagram and rendered in the README.

**NFR-20** — A `README.md` shall provide: project overview, prerequisites, one-command local run instructions (Docker Compose), instructions to trigger the forced-failure scenario, and a pointer to the Grafana dashboard(s).

---

## 5. Acceptance Criteria

The following checklist constitutes the definition of "done" for the first releasable iteration. Each item must be independently verifiable by a reviewer with access to the repository and a Docker-capable workstation.

### Happy-Path Demo

- [ ] **AC-1** (→ FR-1, FR-2, FR-3): A reviewer can send a single order-creation request; the Order Service responds with an order ID and the order reaches `COMPLETED` state without manual intervention.
- [ ] **AC-2** (→ FR-7, FR-10): Running `docker compose up` and then the order-creation command produces observable Kafka events in the sequence `OrderCreated → PaymentProcessed → InventoryReserved`.
- [ ] **AC-3** (→ FR-4, FR-5, FR-6): A `.proto` file exists in the repository; the gRPC `ReserveInventory` call is made during the happy-path run and is visible as a span in Tempo.
- [ ] **AC-4** (→ NFR-3): A single Tempo trace for the happy-path run spans all three services and includes spans for both the gRPC call and the Kafka events.
- [ ] **AC-5** (→ NFR-2, NFR-10): Clicking the root span in Tempo navigates to correlated Loki log lines containing the same `trace_id`.
- [ ] **AC-6** (→ NFR-6): The Grafana SLO dashboard shows a 100 % success rate after the happy-path run.

### Compensation / Rollback Demo

- [ ] **AC-7** (→ FR-14): The README documents exactly how to trigger the forced-failure scenario (a single command, flag, or configuration change).
- [ ] **AC-8** (→ FR-11, FR-12): After triggering the failure scenario, the order reaches `FAILED` or `COMPENSATED` state; a `PaymentRefunded` (or equivalent) event is observable in Kafka.
- [ ] **AC-9** (→ FR-13): Triggering the forced failure twice for the same order ID does not result in a double-refund or double-release (idempotency check).
- [ ] **AC-10** (→ FR-15): Structured logs during the compensation run contain the sequence of saga state transitions including the compensating steps, all tagged with the same `trace_id`.
- [ ] **AC-11** (→ NFR-5): At least one Prometheus alert fires (or its condition is met) during the compensation run; the alert is visible in Grafana.

### Infrastructure & Packaging

- [ ] **AC-12** (→ NFR-11, NFR-12): All service images build successfully with `docker build` on a clean workstation.
- [ ] **AC-13** (→ NFR-13, NFR-14): `docker compose up` brings all services and observability components to a healthy state; services that depend on Kafka do not start processing until Kafka is ready.
- [ ] **AC-14** (→ NFR-15, NFR-16): `kubectl apply -f manifests/` or `helm install` deploys the application to a local Kubernetes cluster (e.g., kind or minikube) without error.
- [ ] **AC-15** (→ NFR-18, NFR-19): Both sequence diagrams (happy path and compensation path) are rendered in the README or linked documentation.

---

## 6. Assumptions & Constraints

| # | Statement |
|---|---|
| A-1 | The demo will be run on a developer workstation capable of running Docker and (optionally) a local Kubernetes cluster; exact hardware specs are not constrained. |
| A-2 | The repository is a single-repo (monorepo) containing all services, protos, infrastructure configs, and docs. (Subject to System Architect confirmation — see OQ-5.) |
| A-3 | The Kafka cluster is provided via Docker Compose for local development; no cloud-managed Kafka is required. |
| A-4 | The observability stack (Prometheus, Loki, Tempo, Grafana) is provided via Docker Compose and/or Kubernetes manifests included in this repository. |
| A-5 | "At-least-once" Kafka delivery is acceptable for this demo; exactly-once semantics are out of scope. |
| A-6 | Service-to-service communication uses gRPC for synchronous calls and Kafka for asynchronous events; no REST/HTTP inter-service calls are required. |
| A-7 | The project is licensed under the existing LICENSE file in the repository root. |

---

## 7. Open Questions for the System Architect

These items must be decided before implementation begins. They are listed in rough priority order.

| # | Question | Impact |
|---|---|---|
| **OQ-1** | **Primary implementation language(s)?** (e.g., Go, Java, Python, Rust, Node.js, or a mix) | Affects build toolchain, OTel SDK choice, gRPC code-gen tooling, Dockerfile base images, and test framework. Must be recorded in `docs/tech-stack.md`. |
| **OQ-2** | **Saga coordination model: choreography, orchestration, or hybrid?** Choreography means each service reacts to events with no central coordinator; orchestration means a dedicated saga orchestrator (or the Order Service acting as one) drives each step explicitly. | Affects service count, event schema design, failure detection logic, and sequence diagrams. |
| **OQ-3** | **Kafka event schema / serialisation format?** (e.g., Avro with Schema Registry, Protobuf, JSON with a documented schema, MessagePack) | Affects schema governance, producer/consumer boilerplate, and whether a Schema Registry container is needed in Compose. |
| **OQ-4** | **OTel Collector topology?** (sidecar per service, single shared collector in Compose, agent-mode) | Affects Compose and Kubernetes manifest complexity. |
| **OQ-5** | **Repository layout: monorepo with per-service subdirectories, or polyrepo?** | Affects how protos are shared, how Helm chart references images, and CI build scoping. |
| **OQ-6** | **Target Kubernetes API version / cluster flavour for local testing?** (e.g., kind 0.x, minikube, k3d) | Needed to pin `apiVersion` fields in manifests and the Helm chart's `kubeVersion` constraint. |
| **OQ-7** | **Grafana dashboard provisioning mechanism?** (dashboards as code committed as JSON/JSONNET/Grafonnet, or provisioned via Grafana's sidecar/ConfigMap mechanism in Kubernetes) | Affects how NFR-6 and NFR-10 are implemented and tested. |
| **OQ-8** | **Build system / dependency management toolchain?** | Required to complete `docs/tech-stack.md` and to define `make`/`task`/`just` targets. |
| **OQ-9** | **Test scope for this iteration?** (unit tests per service, integration tests against the running Compose stack, contract tests for the proto, or all three) | Affects QA Engineer's plan and CI design. |
| **OQ-10** | **Helm chart distribution?** (committed to repo only, published to a chart repository, or packaged as an OCI artefact) | Affects Release Engineer's work. |

---

*Requirements are ready for the System Architect agent to design against.*
