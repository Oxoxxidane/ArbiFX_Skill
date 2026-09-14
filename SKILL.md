---
name: arbifx-http
description: Control ArbiFX's After Effects plugin and Start window through local HTTP, including instance discovery, AE ExtendScript, project inspection, OBJ/SVG/reference assets, text, fonts, tags, prompts, task submission, and AFX save/load. Use when the user asks to operate ArbiFX AE/Start or call its HTTP commands.
metadata:
  version: "1.0.0"
  protocol: "arbifx-local-http-command"
  compatibility: "Standalone Windows/macOS/Linux amd64/arm64 CLI; actual control requires an accessible ArbiFX HTTP service."
---

# ArbiFX HTTP Control

Use the bundled `arbifx` CLI to access all 5 AE and 18 Start local HTTP commands. Read the [HTTP reference](references/http-api.md) for protocol details and the [CLI guide](references/cli.md) for arguments, distribution, and builds. Both ports use `POST /command` without a token. Start's outbound generation backend uses a separate V2 API, which remains managed by Start; this skill does not directly administer that backend.

## Entry point and configuration

Check `arbifx --version`. If the command is not installed, select the bundled executable for the current OS and CPU:

| Platform | Standalone executable |
|---|---|
| Windows x64 / ARM64 | `bin/windows-amd64/arbifx.exe` / `bin/windows-arm64/arbifx.exe` |
| macOS Intel / Apple Silicon | `bin/darwin-amd64/arbifx` / `bin/darwin-arm64/arbifx` |
| Linux x64 / ARM64 | `bin/linux-amd64/arbifx` / `bin/linux-arm64/arbifx` |

Resolve these paths relative to this `SKILL.md`; do not assume the current working directory is the skill directory. On macOS/Linux, `sh <skill-directory>/scripts/arbifx.sh ...` selects the executable automatically. Optional installers are `scripts/install.ps1` on Windows and `scripts/install.sh` on macOS/Linux. Running the bundled executable requires no Go, Python, Node, or third-party libraries.

```sh
arbifx --json doctor
arbifx --json commands
```

Default ports are `28154` for AE and `28153` for Start. The CLI reads only the `[local_http]` port fields in the user's `.ArbiFX/config.toml`. Precedence is `--ae-port/--start-port` > `ARBIFX_AE_PORT/ARBIFX_START_PORT` > configuration > defaults. Use `--config` for another configuration file. Do not read or decrypt API keys to call these local endpoints.

An AE connection failure may mean the ArbiFX effect module has not been initialized. Start's port does not exist while its window is closed. Do not repeatedly open windows or submit tasks to recover connectivity. Use `doctor --target ae` to narrow diagnostics. `doctor --offline` checks configuration only; it does not prove that a host is online.

## Identify the target before acting

1. Run `arbifx --json ae instances`. When several instances exist, match the composition/layer requested by the user. Use `ae resolve --comp "Composition name" --layer "Layer name" --effect-index 0` when useful. It requires exactly one match; otherwise it returns an error and candidates rather than selecting the first instance.
2. The current implementation replaces its ID table on every `instances` request, including the one inside `resolve`. Use the newly returned `data.id` or the instance array's `id` immediately with `ae open-start --id ID`. Do not query `instances` again between these steps or persist IDs for later use. Current indices are **zero-based**; use values returned by the service.
3. A Start window remains bound to one AE instance throughout its lifetime. `open-start` fails if another instance is already bound. Close the current window and open another instance only when switching targets is part of the user's task; do not automatically close a user's window after a failure.
4. Read the relevant `get-*` state, then perform the specific modification already authorized by the user. Do not repeatedly request confirmation for the same task. Use `--dry-run` to inspect complex requests or file effects; it is entirely offline and does not validate whether the host will accept the request.
5. Read back changes using the corresponding `get-*` command. Verify file creation after `save-afx`; `load-afx` applies a scene to the currently bound AE effect. Do not call `send` again while a generation task is still running.

```sh
arbifx --json ae resolve --comp "Comp 1" --layer "ArbiFX"
arbifx --json start set-prompt --prompt-file prompt.txt --parameters-file parameters.txt --dry-run
arbifx --json start get-prompt
```

## Preserve these operational semantics

- For reference files, OBJ, and SVG, **an empty or nonexistent path clears the resource**. Verify the path when the user intends to load a file. Use `--clear` for explicit clearing. OBJ clearing affects the selected type, not the whole slot. Asset `slot` is `0..7`.
- `set-prompt` requires both `prompt` and `parameters`. To change only one, read `get-prompt` and preserve the other; do not silently replace it with an empty string. Prefer UTF-8 files or stdin for multiline text, quotes, and non-ASCII content.
- Fonts use IDs. `get-font` returns `font.id` and `font.family`; `default` is a valid default ID. The protocol has no font catalog endpoint, so do not invent system font IDs.
- `send` submits a generation task using the current Start configuration. Execute it when the user requests generation/submission; merely editing a prompt or inspecting state does not authorize submission. Success means the task started. This protocol exposes no job ID, polling, or cancellation command, so do not claim generation is complete.
- `script` accepts an ExtendScript **function body**. Use `emit(value)` for output and `return` for a string result. This is not modern browser JavaScript: do not depend on ES6, a global JSON object, DOM APIs, or capturing `$.writeln`. Execute code consistent with the user's task; do not modify a project merely to test connectivity.
- Scripts can change the project, and AE provides no reliable forced cancellation. A CLI timeout or disconnect does not mean the operation failed to execute. Read back state first; never automatically replay writes such as `script`, `send`, load, or save.
- `project` returns structure and enabled flags, not complete effect parameters or SceneState. Ordinary Start write success means UI updates and persistence/hot-reload work have been queued; it does not prove AE rendering has finished.

## Raw requests and results

```sh
arbifx --json request start --body-file request.json --dry-run
```

The request file contains a complete JSON object, for example `{"cmd":"get_tags"}`. Use `--body-file -` for stdin. Known commands still validate their fields; future commands require explicit `--allow-unknown`. Raw requests do not convert file paths; use them only with the intended server-side path semantics.

Successful HTTP responses retain `{"ok":true,"data":...}`. Errors return `ok:false`, `error.kind/message/details`, and a nonzero exit code. A nonempty script `data.error` is also a failure even when HTTP and the outer `ok` succeed. Diagnostics exclude configuration secrets, but returned prompts, script results, and paths may contain user content; handle them as required by the task.

When documentation differs from the implementation, consult [Implementation differences and sources](references/http-api.md#implementation-differences-and-sources) before hard-coding an old response shape. Cross-platform CLI distribution does not imply that the current ArbiFX host service supports every platform.
