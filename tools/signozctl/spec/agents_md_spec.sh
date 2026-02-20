#!/usr/bin/env bash
set -euo pipefail

DOC="tools/signozctl/AGENTS.md"

assert_contains() {
  local pattern="$1"
  local message="$2"
  if ! rg -q "$pattern" "$DOC"; then
    echo "FAIL: $message"
    exit 1
  fi
}

test_file_exists() {
  [[ -f "$DOC" ]] || {
    echo "FAIL: missing $DOC"
    exit 1
  }
}

test_required_sections() {
  assert_contains '^## Why We Are Doing This' "missing purpose section"
  assert_contains '^## End Goal' "missing end-goal section"
  assert_contains '^## Product Scope' "missing product scope section"
  assert_contains '^## Feasibility and Limits \(Explicit\)' "missing feasibility section"
  assert_contains '^## Requirements Specification' "missing requirements section"
  assert_contains '^## Authentication and Account Connection' "missing auth section"
  assert_contains '^## Documentation Strategy \(CLI-Native\)' "missing docs strategy section"
  assert_contains '^## Error Model and Guidance Contract' "missing error model section"
  assert_contains '^## Later-Stage Requirement \(Explicitly Deferred\)' "missing deferred docs-intelligence section"
}

test_local_validation_context() {
  assert_contains '^## Local Validation Context \(Current\)' "missing local validation context section"
  assert_contains 'docker compose -f deploy/docker/docker-compose.yaml ps' "missing local docker compose context"
  assert_contains '/Users/yuvraj/Workspace/Work/microservices-monitoring' "missing local traffic source context"
}

test_action_coverage_matrix() {
  assert_contains '^## Action Coverage Matrix \(User UI vs CLI\)' "missing action coverage matrix section"
  assert_contains '\| Query \(Metrics, Logs, Traces\)' "missing query matrix row"
  assert_contains '\| Dashboard CRUD' "missing dashboard matrix row"
  assert_contains '\| Alerts' "missing alerts matrix row"
  assert_contains '\| Invite/RBAC' "missing invite/rbac matrix row"
  assert_contains '\| API Keys' "missing api keys matrix row"
  assert_contains '\| Public Dashboard Sharing' "missing public sharing matrix row"
  assert_contains '\| Service Operations and Dependency Graph' "missing service/dependency matrix row"
  assert_contains '\| Error Tracking APIs' "missing error tracking matrix row"
  assert_contains '\| Retention \(TTL\) and Apdex' "missing ttl/apdex matrix row"
  assert_contains '\| Raw Data Export, Disks/Usage, Health/Version' "missing system matrix row"
}

test_tdd_execution_contract() {
  assert_contains '^## TDD Delivery Contract for `signozctl`' "missing TDD contract section"
  assert_contains 'red-green-refactor' "missing red-green-refactor rule"
  assert_contains 'No implementation code lands without a failing test first' "missing explicit no-code-before-test rule"
}

main() {
  test_file_exists
  test_required_sections
  test_local_validation_context
  test_action_coverage_matrix
  test_tdd_execution_contract
  echo "PASS: $DOC satisfies required planning contract"
}

main "$@"
