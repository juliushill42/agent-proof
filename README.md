# 🛡️ agent-proof

**Deterministic execution receipts and cryptographic verification for autonomous AI agents.**

[![License](https://shields.io)](LICENSE)
[![Verification](https://shields.io)](#quickstart)
[![Offline First](https://shields.io)](#architecture)

> **"Verifiable evidence instead of promises."**  
> Autonomous agents execute code, call tools, and mutate state—yet most systems rely on mutable text logs to prove what happened. `agent-proof` generates tamper-evident cryptographic receipts for every step an agent takes.

---

## ⚡ Why `agent-proof`?

* **Tamper-Evident Execution Receipts:** Binds inputs, environment state, model artifacts, and outputs into verifiable cryptographic commitments (Ristretto255 / Pedersen commitments).
* **Zero-Trust & Offline-First (`TITAN_OFFLINE=1`):** Runs completely air-gapped with zero external API dependencies. Built for sovereign, edge, and regulated enterprise environments.
* **Strict Fail-Closed Audits:** Every step must exit `0` and pass prerequisite checks. If a single verification check fails, the overall receipt fails (`OVERALL RESULT: FAIL`).
* **Drop-In Harness Compatibility:** Wraps existing agent loops, CLI tools, and local inference runtimes (`.gguf` models) without rewriting core agent logic.

---

## 🚀 Quickstart

Clone the repository and run the local verification suite:

```bash
git clone https://github.com
cd agent-proof

# Inspect available scripts and tests
ls -la
```

### Environment Controls

| Variable | Purpose | Example |
| :--- | :--- | :--- |
| `TITAN_ROOT` | Target project directory | `TITAN_ROOT=/path/to/dir` |
| `TITAN_OFFLINE` | Enforce air-gapped execution (forbid network) | `TITAN_OFFLINE=1` |
| `TITAN_MODEL` | Optional local model artifact to hash into receipt | `TITAN_MODEL=/path/to/model.gguf` |

---

## 🏗️ Verification Lifecycle

1. **Pre-Execution Snapshot:** Hashes the target workspace, environment configuration, and optional model artifact.
2. **Fail-Closed Execution:** Runs target agent steps under strict shell/runtime policies (`set -euo pipefail`).
3. **Cryptographic Receipt:** Outputs a deterministic receipt binding inputs, execution status, and state transitions so auditors can verify runs without exposing proprietary data.

---

## 🤝 Contributing

We welcome security researchers, cryptography engineers, and agent builders. See [CONTRIBUTING.md](CONTRIBUTING.md) to get started.

---

## 🏢 Open Ecosystem & Enterprise Licensing

`agent-proof` is part of the **TitanU AI** open ecosystem (**Prove. Attract. Monetize.**):
* **Open Source:** Core verification primitives and receipt schemas are open for developers to inspect, audit, and extend.
* **Enterprise & Commercial:** Need multi-node attestation, compliance reporting, or custom integration into your proprietary agent stack? Connect with **TitanU Ai LLC** at [titanuai.com](https://titanuai.com) or via [LinkedIn](https://linkedin.com).
