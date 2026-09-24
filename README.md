# agent-proof `":"`

**Your AI agent said it finished. Prove it.**

`agent-proof` runs an agent or command, records what actually happened, verifies machine state, classifies false success, attempts an explicit recovery command when configured, then verifies again.

It does not ask the model whether it succeeded.

## Build

```bash
./bootstrap.sh
```

The binary is written to:

```text
bin/agent-proof
```

## Run a command

```bash
bin/agent-proof run -- sh -lc 'echo hello'
```

## Prove a real build

```bash
bin/agent-proof run --spec spec.example.json
```

A proof run checks the requested machine state after execution. A successful process with failed acceptance checks is classified as:

```text
FALSE_SUCCESS_OR_PARTIAL_EXECUTION
```

## Failure injection

```bash
./run-chaos.sh
```

Included scenarios:

```text
fake-success
hung-agent
partial-execution
recovery
changed-file
```

Example:

```bash
bin/agent-proof chaos fake-success
```

Expected result:

```text
classification: FALSE_SUCCESS_OR_PARTIAL_EXECUTION
result: FAIL
CHAOS RESULT: detected expected failure
```

## Evidence

Every run writes:

```text
.agent-proof/runs/<run-id>/
├── request.json
├── snapshot-before.json
├── stdout.log
├── stderr.log
├── execution.jsonl
├── process.json
├── checks-before.json
├── checks-after.json
├── recovery.json
├── manifest.json
├── proof.json
└── checkpoints/
```

Files only appear when relevant to the run.

Each `proof.json` contains a SHA-256 evidence digest. Verify it with:

```bash
bin/agent-proof verify-proof .agent-proof/runs/<run-id>/proof.json
```

## Checks

Current deterministic checks:

- required file exists
- required file changed from its pre-run hash
- shell acceptance command exits zero
- HTTP endpoint returns 2xx/3xx
- Git working-tree clean/dirty state
- Git `HEAD` exists
- environment variable equals an expected value
- command timeout/hang detection

## Recovery

A spec can provide `recover_command`.

`agent-proof` only runs it after the original execution or acceptance checks fail. It then re-runs the acceptance checks and returns either:

```text
RECOVERED_AND_VERIFIED
```

or a failure classification backed by the second verification pass.

## Spec

```json
{
  "name": "prove-build",
  "command": ["sh", "-lc", "make build"],
  "workdir": ".",
  "timeout_seconds": 300,
  "recover_command": ["sh", "-lc", "./repair.sh"],
  "checks": {
    "files_exist": ["dist/app"],
    "files_changed": ["dist/app"],
    "commands": ["./dist/app --healthcheck"],
    "http": ["http://127.0.0.1:8080/healthz"],
    "git_commit": true,
    "timeout_seconds": 30
  }
}
```

## Result model

```text
agent/model output
        |
        v
execution
        |
        v
machine-state verification
        |
        +---- pass ----------------> VERIFIED_SUCCESS
        |
        +---- fail
                 |
                 v
             classify
                 |
                 v
        optional recovery command
                 |
                 v
          verify machine state
                 |
        +--------+---------+
        |                  |
        v                  v
RECOVERED_AND_VERIFIED   FAIL
```

## What a PASS means

A PASS means the configured acceptance checks were observed to pass after execution. It does not treat model text, agent summaries, or a zero process exit code as proof by themselves.

## License

Apache-2.0.

Built by Titan Universal AI LLC.

`":"`
