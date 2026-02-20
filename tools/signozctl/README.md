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

3. Run query payloads:

```bash
./signozctl query traces --file examples/query-traces-v5.json --profile local --output json
./signozctl query logs --file examples/query-logs-v5.json --profile local --output json
./signozctl query metrics --file examples/query-metrics-v5.json --profile local --output json
```

4. Create dashboard from JSON:

```bash
./signozctl dashboard create --file examples/dashboard-minimal.json --profile local --output json
```

## Implemented command groups

- `auth`: `login`, `status`, `logout`, `use`, `profiles`
- `query`: traces/logs/metrics via `/api/v5/query_range`, plus services/dependency/error commands
- `dashboard`: `list`, `create`, `update`, `delete`
- `alerts`: alerts/rules/channels/route-policies/downtime + test helpers
- `iam`: invite, roles, api-keys, users
- `system`: health/version/usage/disks/raw-export/ttl/apdex
- `docs`: deferred placeholders (`search`, `fetch`) per `AGENTS.md`

Run `--help` on every group/command for details.
