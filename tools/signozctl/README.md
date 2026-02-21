# signozctl

`signozctl` is an agent-friendly CLI for SigNoz APIs.

## Build

```bash
cd tools/signozctl
go build ./cmd/signozctl
```

## Core workflows

1. Check server reachability:

```bash
./signozctl system health --host http://localhost:8080 --output json
./signozctl system version --host http://localhost:8080 --output json
```

2. Authenticate and store profile:

```bash
./signozctl auth login \
  --host http://localhost:8080 \
  --email you@example.com \
  --password 'your-password' \
  --profile local \
  --output json
```

Rotate tokens explicitly when needed:

```bash
./signozctl auth refresh --profile local --output json
```

3. Run query payloads:

```bash
./signozctl query traces --file examples/query-traces-v5.json --profile local --output json
./signozctl query logs --file examples/query-logs-v5.json --profile local --output json
./signozctl query metrics --file examples/query-metrics-v5.json --profile local --output json
```

Relative time override without editing payload:

```bash
./signozctl query traces --file examples/query-traces-v5.json --profile local --last 5m --output json
./signozctl query metrics --file examples/query-metrics-v5.json --profile local --last 2h --output json
```

Generate payload templates and schema ideas:

```bash
./signozctl query template --signal traces --output json
./signozctl query schema --signal metrics --output json
./signozctl query validate --file examples/query-traces-v5.json --output json
```

Span-level trace operations:

```bash
./signozctl query trace <trace-id> --profile local --output json
./signozctl query trace-root <trace-id> --profile local --output json
./signozctl query trace-waterfall <trace-id> --profile local --output json
./signozctl query trace-waterfall <trace-id> --expand-all --profile local --output json
./signozctl query trace-flamegraph <trace-id> --profile local --output json
# optional controls without a file:
./signozctl query trace-waterfall <trace-id> --selected-span-id <span-id> --expand-selected --uncollapse-span <span-id> --profile local --output json
./signozctl query trace-flamegraph <trace-id> --selected-span-id <span-id> --profile local --output json
# optional advanced request bodies via file:
./signozctl query trace-waterfall <trace-id> --file examples/trace-waterfall.json --profile local --output json
./signozctl query trace-flamegraph <trace-id> --file examples/trace-flamegraph.json --profile local --output json
```

4. Create dashboard from JSON:

```bash
./signozctl dashboard create --file examples/dashboard-minimal.json --profile local --output json
```

Dashboard public sharing:

```bash
./signozctl dashboard public-create <dashboard-id> --enabled --profile local --output json
./signozctl dashboard public-get <dashboard-id> --profile local --output json
./signozctl dashboard public-update <dashboard-id> --enabled=false --profile local --output json
./signozctl dashboard public-delete <dashboard-id> --profile local --output json
# optional file payload still supported:
./signozctl dashboard public-update <dashboard-id> --file examples/dashboard-public.json --profile local --output json
```

Docs-intelligence from CLI:

```bash
./signozctl docs search "trace waterfall" --limit 5 --output json
./signozctl docs fetch https://signoz.io/docs/traces-management/trace-waterfall --output json
```

Optional encrypted local config:

```bash
export SIGNOZCTL_CONFIG_PASSPHRASE='your-strong-passphrase'
./signozctl auth login --host http://localhost:8080 --email you@example.com --password '...' --profile local
```

Templates, schemas, and local validation for other domains:

```bash
./signozctl dashboard template --resource create --output json
./signozctl dashboard schema --resource public-create --output json
./signozctl dashboard validate --resource create --file examples/dashboard-minimal.json --output json

./signozctl alerts template --resource rule --output json
./signozctl alerts schema --resource channel --output json
./signozctl alerts validate --resource rule --file examples/alerts-rule.json --output json

./signozctl iam template --resource invite --output json
./signozctl iam schema --resource api-key --output json
./signozctl iam validate --resource invite --file examples/iam-invite.json --output json
```

## Implemented command groups

- `auth`: `login`, `status`, `refresh`, `logout`, `use`, `profiles`
- `query`: traces/logs/metrics via `/api/v5/query_range`, plus services/dependency/error commands
- `dashboard`: `list`, `create`, `update`, `delete`
- `alerts`: alerts/rules/channels/route-policies/downtime + test helpers
- `iam`: invite, roles, api-keys, users
- `system`: health/version/usage/disks/raw-export/ttl/apdex
- `docs`: `search`, `fetch` (sitemap search + markdown-like fetch for `/docs/*`)

Run `--help` on every group/command for details.
