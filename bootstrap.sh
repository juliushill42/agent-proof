#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"

command -v go >/dev/null 2>&1 || {
  echo "ERROR: Go 1.22+ is required."
  exit 1
}

echo "[1/5] formatting"
gofmt -w cmd internal

echo "[2/5] tests"
go test ./...

echo "[3/5] vet"
go vet ./...

echo "[4/5] build"
mkdir -p bin
go build -trimpath -ldflags="-s -w" -o bin/agent-proof ./cmd/agent-proof

echo "[5/5] self-proof"
rm -rf .agent-proof/self
bin/agent-proof chaos recovery --evidence .agent-proof/self
PROOF="$(find .agent-proof/self -name proof.json | head -n1)"
bin/agent-proof verify-proof "$PROOF"

echo
echo "READY: $(pwd)/bin/agent-proof"
echo '":"'
