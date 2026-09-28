---
name: arbifx-http
description: Control ArbiFX's After Effects plugin and Start window through local HTTP, including instance discovery, AE ExtendScript, project inspection, OBJ/SVG/reference assets, text, fonts, tags, prompts, task submission/status, and AFX save/load. Use when the user asks to operate ArbiFX AE/Start or call its HTTP commands.
metadata:
  version: "1.3.0"
  protocol: "arbifx-local-http-command"
  compatibility: "Standalone Windows/macOS/Linux amd64/arm64 CLI; actual control requires an accessible ArbiFX HTTP service."
---

# ArbiFX HTTP Control

Use the bundled `arbifx` CLI to access 6 AE and 20 Start local HTTP commands. Read the [HTTP reference](references/http-api.md) for protocol details and the [CLI guide](references/cli.md) for arguments, distribution, and builds. Both ports use `POST /command` without a token. Start's outbound generation backend uses a separate V2 API, which remains managed by Start; this skill does not directly administer that backend.

## Entry point and configuration

Check `arbifx --version`. If the command is not installed, select the bundled executable for the current OS and CPU:

| Platform | Standalone executable |
|---|---|
| Windows x64 / ARM64 | `bin/windows-amd64/arbifx.exe` / `bin/windows-arm64/arbifx.exe` |
| macOS Intel / Apple Silicon | `bin/darwin-amd64/arbifx` / `bin/darwin-arm64/arbifx` |
| Linux x64 / ARM64 | `bin/linux-amd64/arbifx` / `bin/linux-arm64/arbifx` |

Resolve these paths relative to this `SKILL.md`; do not assume the current working directory is the skill directory. On macOS/Linux, `sh <skill-directory>/scripts/arbifx.sh ...` selects the executable automatically. Optional installers are `scripts/install.ps1` on Windows and `scripts/install.sh` on macOS/Linux. Running the bundled executable requires no Go, Python, Node, or third-party libraries.

```sh
arbifx --json doctor --target ae
arbifx --json commands
```

Default ports are `28154` for AE and `28153` for Start. The CLI reads only the `[local_http]` port fields in the user's `.ArbiFX/config.toml`. Precedence is `--ae-port/--start-port` > `ARBIFX_AE_PORT/ARBIFX_START_PORT` > configuration > defaults. Use `--config` for another configuration file. Do not read or decrypt API keys to call these local endpoints.

**Connection prerequisite:** The user must load an ArbiFX (AFX) effect instance at least once in the current AE session, by applying the effect to a layer or opening a project containing it. This initializes the plugin and starts its HTTP listener. Merely launching AE is insufficient: until an instance has been loaded, the CLI cannot connect. Repeat this initialization after restarting AE. When the service is unavailable, explain this prerequisite rather than trying to create the first instance through the unavailable HTTP endpoint.

After an effect instance has been loaded, use `doctor --target ae` to check the AE connection. A closed Start window is normal: when the user's task needs Start, locate the target instance and open it automatically with `ae open-start --id ID`. Do not ask the user to open Start manually. The AE endpoint opens the window even when Start's own port is not yet listening. For an inspection-only request, report its state without opening it. `doctor --offline` checks configuration only; it does not prove that a host is online.

## Identify the target before acting

1. Run `arbifx --json ae instances`. When several instances exist, match the composition/layer requested by the user. Use `ae resolve --comp "Composition name" --layer "Layer name" --effect-index 0` when useful. It requires exactly one match; otherwise it returns an error and candidates rather than selecting the first instance.
2. When the requested task needs Start, call `ae open-start --id ID` for the selected instance as part of that task; no separate manual opening step is needed. The current implementation replaces its ID table on every `instances` request, including the one inside `resolve`. Use the newly returned `data.id` or the instance array's `id` immediately; do not query `instances` again between selection and opening or persist IDs for later use. Current indices are **zero-based**; use values returned by the service. After opening succeeds, check readiness with `doctor --target start` before issuing Start commands. If startup takes a moment, use bounded read-only readiness checks rather than repeatedly calling open-start.
3. A Start window remains bound to one AE instance throughout its lifetime. `open-start` fails if another instance is already bound. Close the current window and open another instance only when switching targets is part of the user's task; do not automatically close a user's window after a failure.
4. Read the relevant `get-*` state, then perform the specific modification already authorized by the user. Do not repeatedly request confirmation for the same task. Use `--dry-run` to inspect complex requests or file effects; it is entirely offline and does not validate whether the host will accept the request.
5. Read back changes using the corresponding `get-*` command. Verify file creation after `save-afx`; `load-afx` applies a scene to the currently bound AE effect. Do not call `send` again while a generation task is still running.

```sh
arbifx --json ae resolve --comp "Comp 1" --layer "ArbiFX"
arbifx --json start set-prompt --prompt-file prompt.txt --parameters-file parameters.txt --dry-run
arbifx --json start get-prompt
arbifx --json start get-status
arbifx --json start get-api-status
```

## Preserve these operational semantics

- For reference files, OBJ, and SVG, **an empty or nonexistent path clears the resource**. Verify the path when the user intends to load a file. Use `--clear` for explicit clearing. OBJ clearing affects the selected type, not the whole slot. Asset `slot` is `0..7`.
- `set-prompt` requires both `prompt` and `parameters`. To change only one, read `get-prompt` and preserve the other; do not silently replace it with an empty string. Prefer UTF-8 files or stdin for multiline text, quotes, and non-ASCII content.
- Fonts use IDs. `get-font` returns `font.id` and `font.family`; `default` is a valid default ID. The protocol has no font catalog endpoint, so do not invent system font IDs.
- Use `start get-api-status` to inspect API setup when needed. It reads only `configured`, `verified`, and `verifying` from Start's current memory; it neither exposes the key nor validates it or contacts the backend. `configured:false` means no key is set. `verified:false` can mean unverified or a previous failure; it is not proof of an invalid key. `verifying:true` means an existing verification is running. Even `verified:true` is a cached result, not a current availability or quota guarantee. See [API configuration snapshot](references/http-api.md#api-configuration-snapshot).
- `send` submits a generation task using the current Start configuration. Execute it when the user requests generation/submission; merely editing a prompt or inspecting state does not authorize submission. Success means the task started. Use `start get-status` to read the current snapshot, including while Start is busy. It returns `busy`, `status_text`, and `pending_job` (null or `{job_id,status,phase,progress}`); `job_id` can be null before creation. There is no cancellation or task-history command.
- `get-status` is read-only and does not contact the generation backend or resume a task. Neither `busy:false` nor `pending_job:null` alone proves success. A paused task can retain pending data, and both success and terminal failure clear it. A pending `status:done` may still be preparing/applying the result. Inspect `status_text` for current feedback such as `Applied.` or `Task failed.`; later operations can overwrite it, and completed task history is not retained. See [Task status snapshot](references/http-api.md#task-status-snapshot) before interpreting results. An older Start may reject the command; report that it needs an updated host, rather than infer completion from another query.
- ArbiFX generation usually takes **5–15 minutes**. Wait patiently after submission; this is the expected generation time, not the HTTP response timeout. Do not treat a long wait as failure or submit the task again. The range is an estimate, not a deadline: taking more than 15 minutes alone does not prove failure. Use spaced, bounded `get-status` checks and available Start UI feedback; do not turn a missing pending task into an automatic resubmission.
- `script` accepts an ExtendScript **function body**. Use `emit(value)` for output and `return` for a string result. This is not modern browser JavaScript: do not depend on ES6, a global JSON object, DOM APIs, or capturing `$.writeln`. Execute code consistent with the user's task; do not modify a project merely to test connectivity.
- Scripts can change the project, and AE provides no reliable forced cancellation. A CLI timeout or disconnect does not mean the operation failed to execute. Read back state first; never automatically replay writes such as `script`, `send`, load, or save.
- Plain `project` returns the legacy structure. New AE hosts support detailed comp/layers/properties/keyframes targets through raw request JSON; see the HTTP reference. Arbitrary SceneState is not exported. Ordinary Start write success means UI updates and persistence/hot-reload work have been queued; it does not prove AE rendering has finished.
- For detailed AE reads and PNG export, first inspect `ae status` capabilities. These operations do not need Start. Use returned composition/layer IDs and property paths, bounded `offset`/`limit` queries, and `children_omitted`/`truncated` flags; do not invent paths or treat a partial result as complete. `ae preview-frame --path frame.png` writes a new PNG and refuses existing paths; raw JSON can select a composition/time. Read the image only after success. A diagnostic response with outer `ok:true` still requires inspecting `diagnostic`; probe timeouts are not proof that AE crashed. See [AE inspection and preview](references/http-api.md#ae-inspection-and-preview-cli-120).

## Raw requests and results

```sh
arbifx --json request start --body-file request.json --dry-run
```

The request file contains a complete JSON object, for example `{"cmd":"get_tags"}`. Use `--body-file -` for stdin. Known commands still validate their fields; future commands require explicit `--allow-unknown`. Raw requests do not convert file paths; use them only with the intended server-side path semantics.

Successful HTTP responses retain `{"ok":true,"data":...}`. Errors return `ok:false`, `error.kind/message/details`, and a nonzero exit code. A nonempty script `data.error` is also a failure even when HTTP and the outer `ok` succeed. Diagnostics exclude configuration secrets, but returned prompts, script results, and paths may contain user content; handle them as required by the task.

When documentation differs from the implementation, consult [Implementation differences and sources](references/http-api.md#implementation-differences-and-sources) before hard-coding an old response shape. Cross-platform CLI distribution does not imply that the current ArbiFX host service supports every platform.
