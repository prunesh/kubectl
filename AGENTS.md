# AGENTS.md — prunesh/kubectl

## Plugin identity

| Field    | Value                        |
|----------|------------------------------|
| ID       | `prunesh/kubectl`            |
| Command  | `kubectl`                    |
| Module   | `github.com/prunesh/kubectl` |
| Manifest | `prunesh.toml`               |

## Project structure

```
cmd/main.go          stdin/v1 binary entry point — do not change the protocol wiring
filter/kubectl.go    all filter logic lives here
kubectl_test.go      table-driven tests for FilterOutput and Rewrite
prunesh.toml         plugin manifest — id, command, platforms, core version constraint
```

There are no other packages. Keep it that way — the zero-dependency rule applies (see below).

## stdin/v1 protocol

`cmd/main.go` reads one JSON object from stdin and writes one JSON object to stdout. Do not touch this wiring unless the protocol version itself changes in prunesh-core.

**Request** (core → plugin):
```json
{ "operation": "rewrite|filter_output", "args": ["..."], "output": "...", "exit_code": 0 }
```

**Response** (plugin → core):
```json
{ "args": ["..."], "changed": true, "output": "..." }
```

`changed: false` short-circuits processing. `exit_code` is -1 when unknown.

## Filter rules

All logic lives in `filter/kubectl.go`. Two entry points:

- **`Rewrite(args []string) ([]string, bool)`**: rewrite the command arguments before execution. Return `nil, false` when no change is needed.
- **`FilterOutput(args []string, output string, exitCode int) string`**: filter stdout after execution. Return the original `output` unchanged when no filtering applies.

### Invariants to preserve

- Non-zero `exitCode` always passes through — never suppress error output.
- `-o json` / `-o yaml` / `--output=json` / `--output=yaml` requests pass through unchanged — structured output is for programmatic use.
- Status fields that indicate degraded or error conditions (`CrashLoopBackOff`, `Error`, `Pending`) must never be dropped.
- When truncating, always append a human-readable notice explaining what was dropped.

## Zero-dependency rule

`go.mod` must have **no `require` entries**. The plugin uses only the Go standard library. If a new filtering need arises that seems to require a dependency, implement it manually — the stdlib `strings` and `fmt` packages are sufficient for all text filtering.

## Checking the prunesh-core version constraint

`prunesh.toml` declares the minimum prunesh-core version this plugin requires:

```toml
[prunesh-core-version]
version = "0.16.0"
constraint = "min"
```

### When to update the constraint

Update `prunesh-core-version.version` only when the plugin relies on a feature or protocol change introduced in a newer core release. Do not bump it speculatively.

### How to check the current installed core version

```bash
prunesh version
```

### How to check the latest published core version

From inside this module (requires network):

```bash
GOPROXY=https://proxy.golang.org go list -m -json github.com/prunesh/prunesh@latest
```

This prints a JSON object — `Version` is the latest released tag.

### Version update checklist

1. Run `prunesh version` — note the running version.
2. Check `prunesh.toml` — note the declared minimum.
3. If the running version is higher **and** you are using a feature that requires it, update `version` in `prunesh.toml`.
4. If the running version satisfies the declared minimum, no change needed.
5. `constraint` is always `"min"` for official plugins — never change it to `"exact"`.

## Testing

```bash
go test ./...
```

Tests live in `kubectl_test.go` at the module root. Use table-driven tests. Cover:
- Passthrough cases (non-zero exit, `-o json`/`-o yaml`, unknown subcommands).
- Truncation boundaries (exactly at the limit, one over, one under).
- Specific patterns this filter strips or rewrites.

## Before committing

- `go test ./...` passes.
- `go build ./cmd/...` succeeds.
- `go mod tidy` leaves `go.mod` unchanged (no new deps introduced).
- If `prunesh-core-version` changed, verify it with `prunesh version`.
