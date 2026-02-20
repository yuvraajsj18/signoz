# SigNoz Agent CLI Plan (`signozctl`)

## Why We Are Doing This
AI Agents can operate SigNoz via raw HTTP APIs today, but direct API usage has recurring problems:
- Inconsistent payloads across endpoints and versions (`v1`, `v2`, `v5`)
- Weak discoverability (agents must remember endpoint shapes)
- Poor error ergonomics (API errors are technically correct but not always actionable)
- Repeated auth/session handling boilerplate

The CLI becomes a stable, agent-friendly control surface over SigNoz APIs.

## End Goal
Provide a single command-line interface that allows any AI agent (or human operator) to safely and reliably perform SigNoz operational actions with:
- Built-in documentation (`--help`, examples, command docs)
- Strong guidance-first error handling
- Consistent output modes for automation (`json`) and humans (`table`/`text`)
- Authentication and workspace context management

In short: **agents should use `signozctl` instead of raw API calls for production-grade automation.**

---

## Current Implementation Status (As of now)
Implemented:
1. Working CLI module at `tools/signozctl` with Go + Cobra
2. Root wrapper at `cmd/signozctl` so repo-root invocation works:
- `go run ./cmd/signozctl ...`
3. Authentication and profile management:
- `auth login/status/logout/use/profiles`
4. Core action commands:
- `query` (metrics/logs/traces and additional service/error/dependency helpers)
- `dashboard` (list/create/update/delete)
- `alerts` (alerts, rules/channels/route-policies/downtime CRUD and test endpoints)
- `iam` (invite, roles, API keys, users)
- `system` (health/version/usage/disks/raw-export/ttl/apdex)
5. Repository examples and command docs:
- `tools/signozctl/README.md`
- `tools/signozctl/examples/*`
6. Unit test coverage for command contracts and behavior in `internal/commands`
7. Live integration tests added (opt-in) for local profile workflows:
- `auth status`
- `query traces` (v5 payload)
- `dashboard create/delete`
8. Baseline normalized API error model implemented:
- status/code -> error class mapping
- actionable hint text in returned errors
9. Query payload ergonomics added:
- `query template --signal <traces|logs|metrics>` for runnable payload generation
- `query schema --signal <...>` for payload shape guidance
- `query validate --file <payload.json>` for local payload validation
- `query <signal> --last <relative-duration>` to override start/end at runtime (e.g. `5m`, `2h`, `2d`, `1w`)
10. Additional route coverage added:
- trace detail commands: `query trace`, `query trace-waterfall`, `query trace-flamegraph`, `query trace-fields`, `query trace-fields-update`
- dashboard public sharing commands: `dashboard public-create/get/update/delete`
11. Template/schema/validate pattern expanded to additional domains:
- `dashboard template/schema/validate`
- `alerts template/schema/validate`
- `iam template/schema/validate`
12. UX improvements from live usage feedback:
- `query trace-waterfall` and `query trace-flamegraph` no longer require `--file` (default request body is `{}`)
- dashboard create schema now includes nested field hints (`widgets/query`, `layout`, `variables`) for agent payload authoring

Not yet complete:
1. Expand live integration coverage across alerts/iam/system/public sharing flows
2. Apply guidance-first error wrapping consistently for all command paths and document machine-readable error contract
3. Improve template/schema fidelity with tighter endpoint-specific field contracts (from full API specs/types)
4. Docs-intelligence implementation (`docs search`, `docs fetch`) is still deferred
5. Auth hardening (refresh ergonomics, secure secret storage upgrades, CI-first flows)

---

## Global Installation
Preferred install path for local/global usage:

```bash
cd /Users/yuvraj/Workspace/Work/signoz/tools/signozctl
go install ./cmd/signozctl
```

Then run:

```bash
signozctl --help
```

If command is not found, add Go bin to `PATH`:

```bash
export PATH="$HOME/go/bin:$PATH"
```

---

## Local Validation Context (Current)
This plan is grounded in a real local SigNoz environment running right now:
- SigNoz stack command: `docker compose -f deploy/docker/docker-compose.yaml ps`
- Active services: `signoz` and `signoz-clickhouse` in healthy state
- SigNoz UI/API host: `http://localhost:8080`
- Live traffic source: `/Users/yuvraj/Workspace/Work/microservices-monitoring`
- Traffic generation pattern: looped `./scripts/generate-traffic.sh` every 10s

Why this matters:
- We can validate query and dashboard workflows against real traces, not synthetic stubs.
- We can verify agent behavior end-to-end (auth -> query -> dashboard/alerts) during implementation.

---

## Product Scope (Current Vision)
The CLI must support agent-executable operations across:

1. Query
- Metrics query workflows
- Logs query workflows
- Traces query workflows

2. Dashboards
- Create dashboard
- Update dashboard
- Delete dashboard
- Public sharing controls

3. Alerts
- Set/manage alerting resources (rules/channels/routing/downtime as available)

4. Identity and Access
- Invite and RBAC actions
- API keys: list/create/update/revoke personal API keys

5. Observability Navigation APIs
- Service operations
- Dependency graph
- Error tracking APIs (list/count/group/detail)

6. Admin/Platform APIs
- Retention (TTL)
- Apdex settings
- Raw data export
- Disks/usage
- Health/version

---

## Action Coverage Matrix (User UI vs CLI)
| User-visible action area | CLI support target | Expected status |
| --- | --- | --- |
| Query (Metrics, Logs, Traces) | Full query command family with stable output contracts | Doable |
| Dashboard CRUD | Create/update/delete dashboard resources, including panel configs | Doable |
| Alerts | Manage alerting resources exposed by APIs | Doable |
| Invite/RBAC | Invite users and manage role/permission operations exposed by APIs | Doable |
| API Keys | List/create/update/revoke personal API keys | Doable |
| Public Dashboard Sharing | Create/update/revoke public share settings when API-backed | Doable |
| Service Operations and Dependency Graph | Retrieve service topology and operation metadata where endpoints exist | Doable |
| Error Tracking APIs | List/count/group/detail flows for errors | Doable |
| Retention (TTL) and Apdex | Read/update retention and Apdex controls where API-backed | Doable |
| Raw Data Export, Disks/Usage, Health/Version | Operational diagnostics and system visibility commands | Doable |
| UI-only interactions (drag/drop layout editing UX) | Express final state via payloads, not mimic browser gestures | Not directly doable as interaction |
| SSO flows requiring live human IdP steps | Support non-interactive token usage; interactive browser SSO is out of CLI-first scope | Not CLI-first |

---

## Feasibility and Limits (Explicit)
This section clarifies what is currently feasible for CLI automation and what is not.

### Doable (Directly via existing APIs)
1. Query and data retrieval
- Metrics/logs/traces query flows
- Service operations/dependency graph/error APIs where endpoints exist

2. Dashboard operations
- Create/update/delete dashboards
- Public dashboard sharing operations exposed by API

3. Alerting and operational controls
- Rules/channels/routing/downtime flows exposed by API

4. Access and account management
- Invites, user/role flows, preferences
- Personal API key lifecycle operations

5. System/admin operations
- Health/version checks
- Retention (TTL), Apdex, export and usage/disks endpoints where available

### Doable With Constraints
1. API coverage gaps
- Some UI flows use endpoints not yet present in OpenAPI.
- These are still doable using manual client wrappers but must be marked `experimental`.

2. Environment-dependent operations
- Cloud/provider integrations may require external credentials, cloud-side setup, or side effects beyond SigNoz APIs.

3. Version/edition differences
- Some endpoints differ across API versions and deployment editions; CLI must detect and guide accordingly.

### Not Doable (or Out of Scope) for CLI-Only
1. Purely visual interactions
- Drag/drop layout editing, chart builder UX interactions, and other browser-only affordances as UX actions.
- CLI can set equivalent final state via API payloads, but not replicate UI interaction behavior.

2. External interactive auth handshakes
- Browser-driven SSO callback completion flows that require human IdP interaction in real time.

3. Capabilities with no stable backend API
- Any UI feature backed only by internal/private/non-contract endpoints should not be treated as stable CLI support until API contracts are formalized.

### Policy
- If an action is API-backed and permissioned, we treat it as CLI-implementable.
- If an action requires browser interaction or external human approval flows, it is not CLI-first.
- Unsupported actions must return explicit guidance and nearest viable alternative.

---

## Requirements Specification

### Functional Requirements
1. Uniform command surface
- Hierarchical commands by domain (query, dashboard, alert, iam, org, system, etc.)
- Consistent flags, naming, and response schemas

2. API abstraction
- Encapsulate API version differences behind stable CLI contracts
- Prefer OpenAPI-generated clients where available
- Manual wrappers allowed for non-OpenAPI endpoints, clearly marked

3. Built-in docs from command line
- `signozctl --help` for global usage
- `signozctl <group> --help` for group-level help
- `signozctl <group> <command> --help` with examples
- `signozctl docs` command to browse quick references from terminal

4. Guided error management
- Normalize backend errors into actionable guidance
- Show probable cause + suggested next command
- Preserve machine-readable error code for automation

5. Auth and identity
- Login/connect workflow to act on behalf of an account
- Store profile(s) and active context (endpoint, org/workspace)
- Support both bearer/session and API key modes where applicable

6. Automation-friendly outputs
- `--output json|yaml|table|text`
- Stable JSON schemas for agent parsing
- Deterministic exit codes

7. Safety
- Confirmation prompts for destructive actions (with `--yes` override)
- Optional `--dry-run` where meaningful

### Non-Functional Requirements
1. Reliability
- Idempotent semantics where feasible
- Retry/backoff for transient failures

2. Usability
- Predictable command grammar
- Clear examples in help text

3. Security
- No token leakage in logs
- Secure local credential storage strategy

4. Extensibility
- New API groups can be added without breaking existing command contracts

---

## Guiding Principles
1. Agent-first UX
- Every command should be discoverable via help
- Every failure should teach the next correct action

2. Contract stability over backend churn
- CLI command contracts should remain stable even if backend endpoint details evolve

3. Explicitness over magic
- Commands and flags should be unambiguous

4. Progressive disclosure
- Quick defaults for common operations
- Advanced flags for power users/agents

5. Safe by default
- Prevent accidental destructive operations

---

## Documentation Strategy (CLI-Native)
The documentation must be available without leaving the terminal:
- Command-level `--help` is mandatory
- Every command includes at least one realistic example
- Global `docs` command should provide:
  - command catalog
  - common workflows
  - troubleshooting

Repository docs should complement CLI docs, not replace them.

---

## Error Model and Guidance Contract
Each CLI error should include:
1. What failed
2. Why it likely failed
3. How to fix it (suggested command/flag)
4. Raw backend code/details (when `--verbose`)

Example error classes:
- `auth_required`
- `auth_insufficient_role`
- `input_validation`
- `resource_not_found`
- `api_unavailable`
- `rate_limited`

Agents should be able to branch on error class programmatically.

---

## Authentication and Account Connection
The CLI must support a robust account connection model:

1. Connection/profile management
- `signozctl auth login`
- `signozctl auth use <profile>`
- `signozctl auth status`
- `signozctl auth logout`

2. Supported auth modes
- Session/bearer token flow (email/password + org/workspace context as required)
- API key flow (for supported operations)

3. Context binding
- Active endpoint (`--host` or profile)
- Active org/workspace selection
- Optional non-interactive mode for agents/CI

4. Secret handling
- Avoid plain-text token output
- Provide configurable secure storage strategy per environment

---

## Suggested Implementation Direction (Given Current Repo)
- Primary language: **Go** (aligned with existing backend stack and distribution model)
- Binary: `signozctl`
- Framework: `cobra` + config layer (`viper` or equivalent)
- API client source of truth: `docs/api/openapi.yml` + supplemental wrappers for uncovered endpoints

This keeps implementation and maintenance close to existing SigNoz server conventions.

---

## Delivery Shape (High-Level Phases)

### Phase 1: Foundation
- CLI skeleton, auth, profiles/context, output/error framework
- Health/version/system baseline commands

### Phase 2: Core Agent Actions
- Query (metrics/logs/traces)
- Dashboard CRUD + public share controls
- Alerts + key IAM flows (invite/RBAC/API keys)

### Phase 3: Admin + Advanced Operations
- TTL/Apdex
- Error tracking APIs
- Dependency graph/service ops
- Raw export/disks/usage

### Phase 4: Hardening
- Robust tests
- Backward-compatibility policy
- Documentation polish and examples for agent workflows
- Live integration tests against local/docker SigNoz
- Error normalization contract completion

This is intentionally a phased blueprint, not a detailed sprint plan.

---

## TDD Delivery Contract for `signozctl`
The delivery process for this CLI follows strict red-green-refactor.

Rules:
1. No implementation code lands without a failing test first.
2. For each behavior:
- Add a focused failing test (`RED`) that demonstrates expected CLI behavior or error guidance.
- Implement minimal code to pass (`GREEN`).
- Refactor while keeping tests green (`REFACTOR`).
3. Every API command must include:
- success-path tests
- auth/permission failure tests
- invalid-input tests with guidance-first errors
4. Every new command must have `--help` contract tests so discoverability does not regress.

Initial practical test targets for this repository:
- auth profile lifecycle (`auth login/use/status/logout`)
- query traces against local running stack
- create a minimal dashboard from a query result payload
- stable JSON output and exit codes for agent automation
- live integration execution command:
  `SIGNOZCTL_E2E=1 SIGNOZCTL_E2E_PROFILE=local go test ./internal/commands -run TestLive -v`

---

## Later-Stage Requirement (Explicitly Deferred)
Add a docs-intelligence capability to help agents instrument apps correctly:

1. Search relevant docs
- Fetch `https://signoz.io/sitemap.xml`
- Filter SigNoz docs URLs
- Fuzzy-match docs for a natural-language query

2. Fetch relevant docs content
- Retrieve content for `https://signoz.io/docs/...` URLs
- Return normalized Markdown for CLI/agent consumption

Planned command family (later stage):
- `signozctl docs search <query>`
- `signozctl docs fetch <url-or-id>`

Purpose:
- Let agents map intent -> authoritative docs -> correct implementation steps.

---

## Success Criteria
We consider this initiative successful when:
1. Agents can complete real SigNoz workflows without handcrafted API payloads
2. Errors consistently guide correction in one iteration
3. `--help` and `docs` are sufficient for first-time usage from terminal only
4. CLI is stable enough to be the preferred automation interface for SigNoz operations
