# CLI Usage, Distribution, and Builds

## Running and installing

**Before any live connection, load an ArbiFX (AFX) effect instance at least once in the current After Effects session.** Apply the effect to a layer or open a project containing it to initialize the plugin's HTTP listener. Opening AE alone is insufficient; the CLI cannot connect before this initialization. Repeat after restarting AE. When a task needs Start, the skill instructs the AI to locate the target instance and open its window through `ae open-start --id ID`; the user does not need to open it manually. For direct CLI use, run that command before issuing Start commands. Installation checks with `--version` and `doctor --offline` do not require AE, but they do not verify live connectivity.

The distribution contains six standalone binaries: Windows/macOS/Linux on amd64/arm64. They require no third-party runtime dependencies. amd64 means Intel/AMD x64; darwin means macOS. Select the executable matching the target OS and CPU.

On Windows, from the extracted skill directory:

```powershell
.\bin\windows-amd64\arbifx.exe --json doctor --offline
& .\scripts\install.ps1
arbifx --json doctor --target ae
```

The installer copies the executable to `%LOCALAPPDATA%\ArbiFX-CLI\bin` and adds that directory to the current user's PATH. Other already-open terminals need to be restarted. Administrator privileges are not required. Installation is optional; the executable can always be invoked by its absolute path.

On macOS/Linux:

```sh
sh ./scripts/arbifx.sh --json doctor --offline
sh ./scripts/install.sh
export PATH="$HOME/.local/bin:$PATH"
arbifx --json doctor --target ae
```

The installer selects the CPU automatically and copies the executable to `~/.local/bin`, or to a directory passed as its first argument. It does not edit shell startup files. The macOS package has no Apple Developer ID signature or notarization and remains subject to OS execution policies. Linux binaries are built with CGO_ENABLED=0 and do not require a particular glibc installation.

The skill follows the [Agent Skills specification](https://agentskills.io/specification): root SKILL.md, YAML metadata, references, and scripts. `agents/openai.yaml` is optional Codex UI metadata. Other Agent Skills-compatible tools can read SKILL.md directly. Place the entire `arbifx-http` directory in the target tool's supported skill location, not just SKILL.md. For CLI-only use, copy the matching executable by itself and retain the applicable license notice when redistributing it.

## Command structure

```sh
arbifx --help
arbifx --version
arbifx commands --target start --json
arbifx doctor --target ae --timeout 3 --json
arbifx ae instances --json
arbifx ae resolve --comp "Comp 1" --layer "ArbiFX" --effect-index 0 --json
arbifx ae open-start --id 27 --dry-run --json
arbifx ae project --json
arbifx ae script --code-file inspect.jsx --dry-run --json
```

`resolve` uses exact names and fails on zero or multiple matches. Replace example ID 27 with the actual ID just returned; do not use it after another instances query. The service provides no pagination: instances/project return complete collections. The CLI response limit is 64 MiB.

```sh
arbifx start set-reference --path reference.png --dry-run
arbifx start get-reference
arbifx start set-obj --slot 0 --type obj --path model.obj --dry-run
arbifx start get-obj
arbifx start set-svg --slot 7 --clear --dry-run
arbifx start get-svg
arbifx start set-text --text-file caption.txt --dry-run
arbifx start get-text
arbifx start set-font --font default --dry-run
arbifx start get-font
arbifx start set-tag --tag use_comp_camera --value true --dry-run
arbifx start get-tags
arbifx start set-prompt --prompt-file prompt.txt --parameters-file parameters.txt --dry-run
arbifx start get-prompt
arbifx start get-status
arbifx start get-api-status
arbifx start get-status
arbifx start send --dry-run
arbifx start save-afx --path scene.afx --dry-run
arbifx start load-afx --path scene.afx --dry-run
arbifx start close --dry-run
```

All writes above are previews. Remove `--dry-run` to execute an intended action. Load previews still require an existing file. Underscore aliases such as `start set_prompt` are supported. `parameters` is required text; use `--parameters=` to explicitly clear it. Prefer files for content with spaces or special characters to avoid shell-specific quoting differences.

`--code-file`, `--text-file`, `--prompt-file`, and `--parameters-file` accept UTF-8 files with an optional BOM. `-` means stdin; only one field per request may read stdin. Do not concatenate prompt or script contents into shell code.

## Raw requests

```sh
arbifx request start --body-file request.json --json
arbifx request ae --body-file - --dry-run --json
```

Example file: `{"cmd":"set_prompt","prompt":"Scene prompt","parameters":"Color controls"}`. Raw requests still use POST /command, 127.0.0.1, and the same port, timeout, and error configuration. Known commands reject missing/extra fields, incorrect types, and invalid ranges. Future commands require `--allow-unknown` and are marked as writes in previews.

Raw requests preserve paths and skip AFX filesystem preflight so they can express the server protocol exactly. Named file commands convert paths relative to the client's working directory into absolute paths, avoiding differences in Start's working directory during local use. For remote hosts reached through a tunnel, use raw requests with server-side absolute paths and follow the server's file rules.

## Configuration

Default file: `~/.ArbiFX/config.toml`, where `~` is the user's home directory:

```toml
[local_http]
ae_global_port = 28154
start_port = 28153
```

The reader extracts only these two integer keys in ArbiFX's generated format; it is not a general TOML editor. It does not output other sections or API keys, or modify host configuration. Invalid configured ports fall back to defaults and produce doctor warnings. Invalid CLI/environment ports fail immediately. Valid ports are 1..65535.

Precedence: CLI flags > `ARBIFX_AE_PORT` / `ARBIFX_START_PORT` > configured ports > defaults. CLI overrides affect only the connection destination, not the host listener. Host configuration changes take effect on its next startup.

The default timeout is 30 seconds. `--timeout` specifies a positive HTTP round-trip deadline of at most 86400 seconds. The CLI retries no HTTP requests. A timeout/read failure leaves the operation outcome uncertain; read back state before deciding what to do next.

## Output and exit codes

Default output is indented JSON; `--json` produces compact JSON. Data goes only to stdout. Help/version output is text. CLI-authored descriptions and diagnostics are English; user content and server responses are preserved in their original language.

Successful server envelopes retain the original data:

```json
{"ok":true,"data":{"text":"Hello"}}
```

Input/network/protocol errors:

```json
{"ok":false,"error":{"kind":"input","message":"Missing --parameters."}}
```

Server rejection preserves the original response:

```json
{"ok":false,"error":{"kind":"api","message":"ArbiFX rejected the command.","details":{"response":{"ok":false,"error":"Start is busy."}}}}
```

Preview example:

```json
{"ok":true,"data":{"dry_run":true,"method":"POST","url":"http://127.0.0.1:28153/command","body":{"cmd":"set_svg","slot":0,"path":""},"mutating":true,"effects":["clear"]}}
```

doctor reports the CLI version, platform, configuration sources, lack of authentication requirements, and target probe results. If a target fails, it still returns complete diagnostic data, with top-level `ok:false` and exit 3. Use `--target ae` to exclude a closed Start window from the check. The AE probe accepts both documented running and current online fields; the Start probe keeps using get_tags for compatibility with older hosts. Updated AE hosts include plugin_version and capabilities in status; older hosts do not. Do not confuse CLI and host versions.

`start get-status` (alias `start get_status`) is available in CLI 1.1.0 and requires a Start host that implements `get_status`. It takes no fields, returns the server's current snapshot unchanged, and is read-only even while Start is busy. An older host's unknown-command response exits 4. See [Task status snapshot](http-api.md#task-status-snapshot) for nullable fields and the limits of completion detection.

`start get-api-status` (alias `start get_api_status`, CLI 1.3.0) takes no fields and returns `configured`, `verified`, and `verifying` unchanged. It reads cached state without sending backend requests or exposing credentials. False flags are valid data with exit 0, not a failed HTTP query. Old hosts return the normal API error, exit 4, without automatic fallback. See [API configuration snapshot](http-api.md#api-configuration-snapshot).

| Exit code | Meaning |
|---|---|
| 0 | Success, including a valid empty list or offline preview |
| 2 | Arguments, fields, configuration, or input-file error |
| 3 | Network, timeout, HTTP/JSON protocol error, or unavailable doctor target |
| 4 | API/script error or non-unique instance match |

Returned user content is not additionally redacted. Configuration secrets and authentication headers are not printed. Do not indiscriminately publish script results, file paths, or prompts in logs.

## Building and testing

Released executables have no runtime dependencies. Rebuilding requires Go 1.25+; this distribution was built with 1.27.1. Source code uses only the standard library, without go.sum or module downloads. The builder uses CGO_ENABLED=0, trimpath, and stripped build/VCS metadata to generate six targets. Retain the Go runtime notice in `references/LICENSE-Go.txt` when redistributing.

```powershell
& ./scripts/build.ps1 -Go 'C:\path\to\go.exe' -OutputDir 'C:\output\arbifx-release'
```

```sh
sh ./scripts/build.sh /tmp/arbifx-release
# Or use the platform-independent build entry point directly:
go run ./scripts/release.go -root . -out /tmp/arbifx-release
```

The builder first runs go vet and go test. Once the native-platform executable is built, it reruns the same HTTP interaction tests against that executable. Other architectures are cross-compiled; this is not a claim of native execution on those platforms.

For documentation-only changes, add `-package-only` to `release.go` to regenerate ZIPs and checksums from the six existing executables. This skips compilation and executable tests; do not use it to release source-code changes.

```sh
cd cli
go test -v ./...
```

Tests connect only to random-port HTTP fixtures. Coverage includes all 26 supported commands' method/path/body/UTF-8 lengths, status snapshot preservation (including null IDs, paused tasks and terminal states), dry-run with no connections, no proxies/redirects/retries, Unicode files and stdin, invalid fields, configuration precedence, secret exclusion, instance response compatibility and ambiguity, and API/script/timeout/connection failures. Setting `ARBIFX_TEST_EXE` also verifies independent execution from a temporary working directory with no Go/Python/Node on PATH.

The release directory contains six platform ZIPs, one all-platform ZIP, and binary/ZIP SHA-256 lists. Every archive has an `arbifx-http/` root with the skill, references, source, scripts, and the relevant executable(s). ZIP entries preserve executable permissions for Unix scripts and binaries; apply chmod if the extraction tool does not retain them.

See the [Validation record](validation.md) for this release's actual test scope and native-host limitations.

Source layout: `cli/main.go` handles arguments/workflows; `cli/protocol.go` defines commands and field validation; `cli/transport.go` handles HTTP/errors; `cli/config.go` reads ports; `cli/main_test.go` and `cli/inspection_test.go` contain tests. `scripts/release.go` compiles and archives all platforms, with build.ps1/build.sh as wrappers. Installers copy already-built executables.

CLI 1.2.0 adds `ae preview-frame --path ABSOLUTE.png` and raw optional fields for AE `project`/`status`. See the HTTP reference's AE inspection section. New host status includes a plugin version and capability flags; the CLI does not infer support solely from product version. Extended queries use a single UTF-8 JSON request file, preserving server path semantics.
