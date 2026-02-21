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

Live tail logs via polling:

```bash
./signozctl query logs-tail --file examples/query-logs-v5.json --profile local --interval 2s --last 5m --output json
```

Saved views (traces/logs/metrics explorer):

```bash
./signozctl view template --source-page traces --service-name catalog-node --output json
./signozctl view schema --source-page traces --output json
./signozctl view validate --file examples/view-traces-catalog-node.json --output json
./signozctl view create --profile local --file examples/view-traces-catalog-node.json --output json
./signozctl view create --profile local --name "Catalog Node Saved View" --source-page traces --service-name catalog-node --output json
./signozctl view list --profile local --source-page traces --output json
./signozctl view get <view-id> --profile local --output json
./signozctl view update <view-id> --profile local --file examples/view-traces-catalog-node.json --output json
./signozctl view delete <view-id> --profile local --output json
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

Create an empty view quickly:

```bash
./signozctl dashboard view-create --title "Agent View" --description "created from flags" --profile local --output json
```

Browse and apply templates:

```bash
./signozctl dashboard templates list --output json
./signozctl dashboard templates search host --output json
./signozctl dashboard templates show hostmetrics --output json
./signozctl dashboard templates apply hostmetrics --profile local --output json
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
- `query`: traces/logs/metrics via `/api/v5/query_range`, plus `logs-tail`, services/dependency/error commands
- `view`: saved view CRUD + template/schema/validate for `/api/v1/explorer/views`
- `dashboard`: `list`, `create`, `view-create`, `update`, `delete`, `templates`
- `alerts`: alerts/rules/channels/route-policies/downtime + test helpers
- `iam`: invite, roles, api-keys, users
- `system`: health/version/usage/disks/raw-export/ttl/apdex
- `docs`: `search`, `fetch` (sitemap search + markdown-like fetch for `/docs/*`)

Run `--help` on every group/command for details.
