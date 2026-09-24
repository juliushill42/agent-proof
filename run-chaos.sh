#!/usr/bin/env bash
set -euo pipefail
BIN="${1:-./bin/agent-proof}"
for s in fake-success hung-agent partial-execution recovery changed-file; do
  echo
  echo "=== $s ==="
  "$BIN" chaos "$s" --evidence .agent-proof/chaos
done
