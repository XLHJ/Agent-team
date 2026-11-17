# Phase-2 CLI signature table

New agents are added by editing YAML only — no Go changes.

## Fields

| Field | Meaning |
|---|---|
| name | signature id, becomes the `cli:<name>:` instance prefix |
| display_name | panel label |
| executables | file names to look up on PATH (PATHEXT is honored on Windows) |
| version_args / version_regex | `--version` probe; confidence=high when matched |
| methods | takeover priority order: native > stdio > pty |
| approval_mode | native_forward / tool_hook / "" (no approval capability) |
| args | extra launch args for the structured-stdio channel |

## Shipped entries

| File | Agent | Methods | Approval |
|---|---|---|---|
| claude-code.yaml | Claude Code | stdio (`-p --output-format stream-json --input-format stream-json --verbose`) | tool_hook |
| codex.yaml | Codex CLI | stdio (`exec --json -`) | tool_hook |
| hermes.yaml | Hermes CLI | native, stdio, pty | native_forward |
| dsh.yaml | DSH CLI | pty (placeholder until Phase-3 adapter) | none |
| zcode.yaml | ZCode CLI | pty (placeholder until Phase-3 adapter) | none |
| fakeagent.yaml | Fake Agent (self-test) | stdio | none |

## Windows notes

- npm `.cmd`/`.bat` shims are resolved to the real `node <cli.js>` entry when
  possible (avoids BatBadBut quoting issues); fallback is `cmd /c <shim>`.
- PTY uses ConPTY via go-pty; the CLI runs inside a pseudo terminal.
- Kill semantics: process-group kill (posix) / `taskkill /T /F` (windows).
