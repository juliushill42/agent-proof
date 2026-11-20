# Agent Proof

**Verify what an agent actually did—not just what it said it did.**

[![Go](https://img.shields.io/badge/Go-1.22%2B-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![License](https://img.shields.io/badge/License-Apache--2.0-blue.svg)](LICENSE)
[![Interactive Lab](https://img.shields.io/badge/Interactive-Agent%20Proof%20Lab-65e5e0)](https://proof.movingwithclarity.com)

> **Explore it first:** [proof.movingwithclarity.com](https://proof.movingwithclarity.com) — interactive WebGL visualization, WebGPU capability check, scenario explorer, spec builder, and local execution guide.

Agent Proof is a Go command-line tool for running a command against explicit acceptance checks and writing a structured JSON record of the observed process and resulting state. It is designed to catch false success, partial execution, timeouts, and failed recovery.

## What it does

- Executes a configured command with a timeout.
- Captures exit status, duration, stdout, stderr, and process metadata.
- Checks configured files, file changes, shell predicates, HTTP status, Git state, and environment values.
- Classifies timeouts, execution failures, false success / partial execution, verified success, and recovered success.
- Optionally runs a recovery command, then repeats the configured checks.
- Writes proof JSON, logs, checkpoints, and a SHA-256 digest.

**Integrity boundary:** the SHA-256 digest detects changes when compared with a trusted copy of the original digest. The current format is not a digital signature, remote attestation, Pedersen commitment, or zero-knowledge proof. Anyone who can rewrite the proof can also recompute its digest. Use an independently stored or signed digest when you need an external trust anchor.

## Interactive lab

Open **[Agent Proof Lab](https://proof.movingwithclarity.com)** to:

1. Explore the verification lifecycle in an animated WebGL scene.
2. Check whether the browser supports WebGPU compute; WebGL remains the visual fallback.
3. Walk through the five built-in chaos scenarios.
4. Generate and copy a starter JSON spec.
5. Follow the commands to run the real Go engine locally.

The browser scenarios are explanatory simulations. They do not execute shell commands or produce Go evidence in the browser. Real execution and evidence generation happen through the Go CLI on a machine you control.

## Quickstart

Requirements: Go 1.22+ and a shell.

    git clone https://github.com/juliushill42/agent-proof.git
    cd agent-proof
    go test ./...
    go build -o ./bin/agent-proof ./cmd/agent-proof

    # Detect a false-success run (the expected scenario result is FAIL).
    ./bin/agent-proof chaos fake-success --evidence .agent-proof/demo

    # Run the included sample spec and write evidence.
    ./bin/agent-proof run --spec spec.example.json --evidence .agent-proof/runs

    # List recorded runs.
    ./bin/agent-proof list --evidence .agent-proof/runs

The run command prints the exact path to the generated proof.json. Verify it with:

    ./bin/agent-proof verify-proof .agent-proof/runs/<RUN_ID>/proof.json

## Run a command directly

    ./bin/agent-proof run -- sh -lc 'mkdir -p dist && printf "built\\n" > dist/app.txt'

For meaningful acceptance, use a spec with checks that match the artifact or state your workflow must produce. A direct run uses a minimal successful shell predicate; it does not establish that your application is correct.

## Spec format

See [spec.example.json](spec.example.json). Supported check fields:

| Field | What it checks |
|---|---|
| files_exist | Each configured path exists after execution. |
| files_changed | The file state differs from the pre-execution snapshot. |
| commands | Each shell predicate exits successfully. |
| http | Each URL returns an HTTP status from 200 through 399. |
| git_clean | Git working-tree cleanliness matches the requested boolean. |
| git_commit | Git HEAD resolves to a commit. |
| env_equals | Named environment variables equal their configured values. |
| timeout_seconds | Timeout applied to individual command checks. |

The run-level timeout_seconds controls the primary and recovery commands. See the source and tests for exact behavior.

## Chaos scenarios

    ./bin/agent-proof chaos fake-success
    ./bin/agent-proof chaos hung-agent
    ./bin/agent-proof chaos partial-execution
    ./bin/agent-proof chaos recovery
    ./bin/agent-proof chaos changed-file

The first three scenarios are designed to produce a failed proof; the CLI reports detection of the expected failure. Recovery and changed-file scenarios are designed to produce passing proofs.

## Evidence layout

Each run is written beneath the selected evidence directory in a timestamped run folder. It includes proof.json, request and snapshot data, process output, check results, and checkpoints. The proof includes the observed result, classification, host metadata, and SHA-256 digest.

## Contributing

Issues and pull requests are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md). Run go test ./... before submitting changes.

## Project

Agent Proof is part of the TitanU open ecosystem.

- Source: [github.com/juliushill42/agent-proof](https://github.com/juliushill42/agent-proof)
- Interactive experience: [proof.movingwithclarity.com](https://proof.movingwithclarity.com)
- Moving With Clarity: [movingwithclarity.com](https://movingwithclarity.com)
- License: [Apache-2.0](LICENSE)
