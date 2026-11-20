# Contributing to `agent-proof`

Thank you for helping make autonomous AI systems verifiable and auditable.

## Principles
1. **Deterministic & Fail-Closed:** Any new check or primitive must fail loudly on missing prerequisites or invalid states.
2. **Offline-First:** Core verification paths must never require outbound network calls when `TITAN_OFFLINE=1` is set.
3. **Verifiable Receipts:** Include reproducible test commands and expected `PASS/FAIL` outputs in every Pull Request.

## Submitting Changes
1. Fork the repository and create a feature branch (`git checkout -b feat/your-feature`).
2. Run all local tests and verification scripts.
3. Open a Pull Request describing the threat model or verification capability added.
