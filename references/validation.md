# Distribution Validation Record

## 1.3.0 — 2026-09-28

- Aligned the command catalog to 6 AE and 20 Start commands. Added the read-only API configuration snapshot and retained existing task-status and AE inspection support.
- Source `go vet` and the full `go test` suite passed. The Windows amd64 executable passed the same suite, including all 26 commands over HTTP, detailed query request/response preservation, invalid field rejection before connection, diagnostic failure inside an otherwise successful status response, PNG path preflight, API status states, old-host errors, dry-run without connections, and no automatic retries.
- Tested the new Windows CLI against the installed Start executable using isolated profiles, synthetic keys, and a local verification server. Unconfigured, verification-in-progress, verified, rejected, and malformed-response states were read correctly. Repeated reads preserved configuration bytes/mtime, prompt and task state, and generated zero additional backend requests. No production key or generation task was used. All test Start processes were closed.
- Live checks passed against the user-opened After Effects 26.0x67 session with ArbiFX 2.1.0 using the distributed Windows amd64 CLI. Diagnostics reported a responsive main thread, available scripting, and enabled script file writing. Legacy project listing, composition details, layers, properties, expression sampling at a specified time, and keyframes all returned successfully; layer and keyframe pagination were checked against their full results.
- Both current-frame and explicit composition/time PNG exports produced readable 640 x 360 images with complete PNG headers and end markers. An existing output was rejected without changing its bytes. Composition metadata remained unchanged after current-frame export; explicit-time export also preserved the active composition, playhead time, resolution factor, and project dirty flag. Checks used the existing project without editing it or submitting generation tasks. Earlier isolated AE startup attempts had exited before opening a listener; validation was completed after the user started AE.
- All six standalone binaries were rebuilt; Windows amd64 was executed, and the other five targets were cross-compiled. The release filename version now comes from the CLI source rather than a separate hard-coded value. No ArbiFX host/plugin source or installation was changed.

## 1.1.0 — 2026-09-27

Toolchain: Go 1.27.1 windows/amd64, standard library only, CGO_ENABLED=0.

- Compared `get_status` with the current Start implementation and host protocol document. The CLI sends a parameter-free read-only command and preserves the full server snapshot.
- Source `go vet` and the full `go test` suite passed. The built Windows amd64 executable passed the same suite, covering all 24 supported commands and standalone execution without Go/Python/Node on PATH.
- Added coverage for `get-status`, `get_status`, and the validated raw request form; null job IDs; Unicode status text; idle, preparing, running, paused, preparing-result, applied, and failed snapshots; dry-run without connections; unexpected-field rejection; and an older host's unknown-command error without retry or fallback.
- The Windows amd64 CLI made 68 successful status requests against real Start processes with isolated profiles and local mock AE/backend services. All five cases passed: idle; creation paused without a job ID; running to backend failure; running to network pause; running to result-reading failure. Repeated reads preserved prompt/TXT and made no extra AE IPC calls. No real generation service or user AE project was used.
- All six Windows/macOS/Linux amd64/ARM64 binaries were rebuilt. Windows amd64 was executed locally; the other five targets were cross-compiled, not run on native hardware. Cross-compilation does not establish native host integration on those platforms.
- Verified PE/ELF/Mach-O architecture headers, binary and archive SHA-256 lists, and all seven ZIPs. Archive contents match the skill tree, platform packages contain the intended executables, Unix executable bits are preserved, and the installed Windows CLI matches its bundled binary. The command catalog is exactly the previous 23 commands plus `get_status`.
- Skill frontmatter validation passed. Updated instructions document snapshot limitations and old-host behavior. Start `doctor` probing remains `get_tags` for compatibility.
- This update changes the standalone CLI/skill distribution only; ArbiFX host source and installed plugin binaries are unchanged.

## Previous validation — 2026-09-14

Date: 2026-09-14. Toolchain: Go 1.27.1 windows/amd64. Standard library only, with CGO_ENABLED=0.

- Source go vet and go test passed.
- The standalone Windows amd64 executable passed tests for all 23 commands against random-port localhost HTTP fixtures, checking POST paths, JSON fields, and UTF-8 Content-Length.
- The same executable passed dry-run/no-connection, Unicode/multiline/file/stdin, configuration precedence, secret exclusion, instance resolution, unknown-command, input-boundary, API/script/network/timeout, proxy bypass, redirect rejection, and no-replay tests.
- The standalone executable passed offline doctor from a temporary working directory with no Python/Node/Go on PATH. It does not depend on the source directory.
- Read-only status and instances requests succeeded against the local AE host, confirming online/port, nested comp/layer objects, and zero-based indices. Start was not running during that check; live Start writes, submission, save, and load were not performed. Their coverage comes from HTTP fixtures.
- Windows ARM64, macOS Intel/ARM64, and Linux amd64/ARM64 binaries were cross-compiled. They have not been run on native machines with those OS/CPU combinations. Successful compilation does not establish host integration on those platforms.
- The project's HTTP listener currently has a Windows-only server implementation. Cross-platform client binaries do not change server platform support. The original ArbiFX3D project was not modified by this task.

The English edition updates skill instructions, references, UI metadata, and CLI-authored messages. Unicode test fixtures intentionally retain non-English input to verify content preservation. Rebuilds rerun source and native-executable tests. Update statements about live AE/Start and other native platforms only after performing those checks.

## 1.2.0 — 2026-09-28

- Added AE preview_frame plus optional raw project/status query fields; existing named commands retain their arguments.
- go test and go vet passed, including raw query validation, and the Windows amd64 executable passed the full interaction suite. All six binaries cross-compiled successfully; only Windows amd64 was executed.
- Previous host integration evidence was recorded in the original ArbiFX3D workspace. CLI validation is distinct from AE renderer validation.
