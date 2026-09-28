# ArbiFX AE/Start Local HTTP Reference

Contents: transport and responses, six AE commands, twenty supported Start commands, task status snapshots, lifecycle, and implementation differences.

## Transport and responses

| Target | Default URL | Configuration key |
|---|---|---|
| AE | `http://127.0.0.1:28154/command` | `[local_http] ae_global_port` |
| Start | `http://127.0.0.1:28153/command` | `[local_http] start_port` |

Both targets accept `POST /command` with UTF-8 JSON and a required string `cmd`. No authentication headers are required, but the HTTP implementation requires an accurate `Content-Length`. The body limit is 1 MiB. The CLI supplies the UTF-8 byte length and JSON Content-Type; it uses no proxy, redirects, chunked uploads, or automatic retries.

Server success: `{"ok":true,"data":...}`. Business failures usually still use HTTP 200: `{"ok":false,"error":"message"}`. Always check JSON `ok`; for AE scripts also check `data.error`. See [Output and exit codes](cli.md#output-and-exit-codes) for CLI error envelopes.

## AE commands

| cmd / CLI | Required fields | Returned data and behavior |
|---|---|---|
| `status` / `ae status` | None; optional `diagnose:boolean`, `timeout_ms:int` through raw JSON | online/port plus plugin_version, process_id, ae_version, capabilities; optional diagnostic. Older hosts may return only running or online/port. |
| `instances` / `ae instances` | None | Instance array: `id`, `comp:{id,name}`, `layer:{id,index,name}`, and `effect_index` in the current implementation. |
| `open_start` / `ae open-start` | Nonempty `id:string` | `{"opened":true}`, or `{"opened":false}` when already open for that instance. Does not rebind a different instance. |
| `script` / `ae script` | `code:string` | `{"result":"...","output":"...","error":""}`. Code is a function body supporting return and emit. |
| `project` / `ae project` | None for legacy view; optional target-specific fields through raw JSON | Legacy `{"comps":[...]}`; detailed comp/layers/properties/keyframes views described below. |
| `preview_frame` / `ae preview-frame` | `path:string`; optional `comp_id`, `time` through raw JSON | Writes a new composition PNG and returns its path, composition/time, resolution factor, and actual image dimensions. |

Current instance example:

```json
{"ok":true,"data":[{"id":"27","comp":{"id":3,"name":"Comp 1"},"layer":{"id":4,"index":0,"name":"ArbiFX"},"effect_index":0}]}
```

Every `instances` request refreshes the ID table and allocates new IDs. `ae resolve` also makes one `instances` request and supports exact composition name, layer name, composition ID, layer ID, layer index, and effect index filters. After obtaining an ID, call `open-start` without scanning again. Resolving an old instance ID is not supported.

All layer/effect indices currently returned are **zero-based**. Do not apply AE UI or ExtendScript one-based indices directly to HTTP values. An instance ID is not a layer index, effect index, or persistent UUID. `project` currently does not return IDs usable by `open_start`.

```sh
arbifx ae script --code "emit('hello'); return app.project.numItems;" --dry-run
arbifx ae script --code-file inspect.jsx
```

Scripts run on the AE main thread, without a sandbox, reliable execution timeout, or forced cancellation. The client does not launch AfterFX or forward scripts through `afterfx -r`.

## Nineteen supported Start commands

All fields below are top-level JSON properties. Example: `{"cmd":"set_obj","slot":0,"type":"obj","path":"C:\\asset\\model.obj"}`. CLI names replace `_` with `-` and also accept the original underscore spelling.

| cmd | Required fields | Returned data / behavior |
|---|---|---|
| `get_status` | None | `{busy,status_text,pending_job}` current snapshot; read-only and available while busy. See below for interpretation. |
| `get_api_status` | None | `{configured,verified,verifying}` cached API configuration/verification flags; no validation or backend request. |
| `set_reference` | `path:string` | `{"file_name":"reference.png","loaded":true}`; immediately synchronizes the reference cache and UI. |
| `get_reference` | None | Same shape; empty name and loaded=false when absent. No path or file content is returned. |
| `set_obj` | `slot:int 0..7`, `type:string`, `path:string` | Returns `{"slots":[...]}` with all eight slots. Setting obj retains automatic MTL/texture loading; setting mtl retains associated texture refresh. |
| `get_obj` | None | `{"slots":[...]}` containing eight slots. Each has slot and obj/mtl/texture1/texture2/texture3; each resource is `{path,loaded}`. |
| `set_svg` | `slot:int 0..7`, `path:string` | Returns `{"slots":[...]}` with all eight slots, following the manual validation, persistence, and hot-reload workflow. |
| `get_svg` | None | `{"slots":[...]}` containing eight slots, each `{slot,path,loaded}`. |
| `set_text` | `text:string` | `{"text":"normalized text"}`; empty text is allowed. |
| `get_text` | None | `{"text":"..."}`. |
| `set_font` | Font ID as `font:string` | Currently `{"font":{"id":"default","family":""}}`. Unknown IDs fail and preserve the previous font. |
| `get_font` | None | Same shape. No font enumeration endpoint exists. |
| `set_tag` | `tag:string`, `value:boolean` | All five boolean tags. |
| `get_tags` | None | use_comp_camera, is_filter, is_alpha, use_comp_light, only_sdf. |
| `set_prompt` | `prompt:string`, `parameters:string` | `{"prompt":"...","parameters":"..."}`. Both fields are required. |
| `get_prompt` | None | Same shape. parameters is control text, not a JSON parameter object. |
| `send` | None | `{}` means validation passed and background work started, not that generation completed. |
| `save_afx` | `path:string` | `{}`; saves an existing scene package. An empty effect may return `Nothing to save.`. |
| `load_afx` | `path:string` | `{}`; loads an existing .afx, applies it to the bound AE instance, and synchronizes the UI. |
| `close` | None | `{"closing":true}`; closes the window normally and subsequently releases the port. |

OBJ type is restricted to `obj`, `mtl`, `texture1`, `texture2`, `texture3`. The tag must be one of the five names above. value must be a JSON boolean, not a string or 0/1.

### API configuration snapshot

```sh
arbifx --json start get-api-status
```

The command and its `get_api_status` alias send exactly `{"cmd":"get_api_status"}` with no additional fields. The raw request form is also supported. It is read-only, including while Start is busy; dry-run never connects.

```json
{"ok":true,"data":{"configured":true,"verified":true,"verifying":false}}
```

| Field | Meaning |
|---|---|
| `configured` | A key is present in current Start memory; not proof that the key, URL, or proxy is usable. |
| `verified` | Cached result of the existing verification workflow; false can mean unverified or failed. |
| `verifying` | An existing verification is currently running; other flags retain their current values. |

All three fields are booleans. False flags are valid successful query results, not CLI errors. The query neither starts verification nor reads/writes configuration files, sends backend requests, changes UI/task state, or returns the key, server URL, or proxy. Flags belong to the current window lifecycle. A cached success does not guarantee current availability or quota. Window startup or user-initiated verification may independently perform network requests. Older hosts return their normal unknown-command error; the CLI does not attempt another verification route.

### Task status snapshot

```sh
arbifx --json start get-status
arbifx --json start get_status
arbifx --json start get-status --dry-run
```

Both spellings send exactly `{"cmd":"get_status"}` to Start. No task ID or other command fields are accepted. The raw `request start` form also recognizes this command without `--allow-unknown`. Dry-run reports `mutating:false` and makes no HTTP request.

Example running response:

```json
{"ok":true,"data":{"busy":true,"status_text":"Working...","pending_job":{"job_id":"job_xxx","status":"running","phase":"generating","progress":45}}}
```

| Field | Type | Meaning |
|---|---|---|
| `busy` | boolean | Current Start busy flag, not a success indicator. |
| `status_text` | string | Existing host status text, passed through unchanged. |
| `pending_job` | object / null | Current pending task summary, or null when absent. |
| `pending_job.job_id` | string / null | Backend job ID; may be null before job creation. |
| `pending_job.status` | string | Existing initial/local or most recently recorded backend task status. |
| `pending_job.phase` | string | Existing task phase. |
| `pending_job.progress` | integer | Most recently recorded progress, 0–100; initially 0. |

An idle response is `{"ok":true,"data":{"busy":false,"status_text":"Idle","pending_job":null}}`. The read executes on the Start UI thread and does not contact the backend, resume generation, or change session/UI/persistence state. The CLI preserves the response; it does not derive a separate completion flag.

Interpretation boundaries:

- `busy:false` alone does not mean success. Network failures can pause a task while retaining `pending_job` and its last running status.
- `pending_job:null` can mean no task, successful application, terminal backend failure, result-reading failure, or application failure.
- `pending_job.status:done` and progress 100 do not prove AE application finished; Start can still be preparing the result.
- Immediately after completion, `status_text` can report `Applied.`, `Task failed.`, or another result. Later operations, such as saving a file, can overwrite it. The endpoint does not retain completed task history across operations or window restarts.
- Older Start versions can return `Unknown command.`. The CLI returns the normal API error (exit 4), preserving that response; it does not fall back to a fabricated status. `doctor` continues to probe `get_tags` for compatibility.

### File rules

- Reference files, OBJ, and SVG: an empty or nonexistent path clears the resource. An existing but invalid, unreadable, oversized, or unloadable file fails and preserves the previous state. Clearing OBJ affects only the specified type. CLI `--clear` explicitly sends `path:""`.
- A reference must be a regular file larger than zero bytes and at most 20 MiB. The host reads the bytes; HTTP carries a path, not a base64 upload.
- Save/Load paths must be nonempty and should explicitly include `.afx`. Save requires an existing parent directory; Load requires an existing regular file. Neither supports clearing. Named CLI commands validate these conditions locally and convert relative paths to absolute paths.
- Save writes a file and may overwrite an existing one; dry-run reports `overwrite_existing_file`. Load changes the currently bound AE effect.
- Save excludes local OBJ/SVG slot resources from .afx. Load first removes package-local slots, then attaches the current Start window's local slots. These are the existing manual behaviors.
- Raw `request` preserves paths and skips local filesystem preflight. Use it when a path must be interpreted by the server's OS. With an SSH-forwarded remote host, do not normalize remote paths using named local file commands.

## Lifecycle and unavailable capabilities

The AE listener exists only after ArbiFX AEX GlobalSetup. To initialize it, load an ArbiFX (AFX) effect instance at least once in the current AE session: apply the effect to a layer or open a project containing it. Launching AE alone does not make the endpoint available, so connections fail until an instance has been loaded. After restarting AE, load an instance again. The Start listener exists while its window process is alive; the AI can start that process through AE's `open_start` command without requiring the user to open the window manually. Port conflicts do not trigger automatic port changes. If multiple AE processes share the same configuration, only the first successful listener is available. A successful connection does not establish which AE process the user intends; inspect instances before operating.

Many Start writes fail while Start is busy; do not assume they can be queued or retried. `send` can also fail when no API key is configured, a pending task cannot be recovered, or the session cannot be written to AE. AE hot-reload work may still be queued after an HTTP response. Use `get_status` for the current snapshot. There is no font catalog, task-history lookup/cancellation, window rebinding, complete parameter export, or arbitrary file download endpoint. The CLI does not invent these capabilities.

The macOS/Linux binaries provide client portability. At inspection time, the non-Windows branch of `src/common/src/local_http.cpp` returns `Local HTTP listener is only supported on Windows.`. This does not establish macOS/Linux support for the repository's AE/Start service. If the user has authorized and established an SSH local-port forward to a Windows host, the CLI can use the forwarded localhost ports. It does not establish tunnels or expose a network listener.

## Implementation differences and sources

Originally inspected on 2026-09-14; `get_status` was checked against the actual host implementation and protocol document on 2026-09-27. The client preserves original responses. This CLI update does not change ArbiFX project source or the host protocol.

Paths below are relative to the original ArbiFX3D source workspace. That workspace is not required to use this skill.

| Project document / implementation | Purpose |
|---|---|
| `docs/local-http-protocol-and-implementation-plan.zh-CN.md` | Host protocol reference, including the `get_status` snapshot contract and consistency with manual actions. |
| `src/ae_adapter/src/start_launcher.cpp` | `HandleAeGlobalHttpRequest`, `ProcessGlobalHttpRequest`, `ScanAeProject`, `GlobalHttpScriptWrapper`. |
| `apps/start_host/src/main.cpp` | `HandleStartHttpRequest`, `ExecuteStartHttpCommand`, `SelectedFontForUi`. |
| `src/common/src/local_http.cpp` | POST path, Content-Length, body limit, and server platform scope. |
| `config/config.toml.example` | Local port configuration. |
| `backend-api-arbifx-three-v2.zh-CN.md` | Start's outbound backend protocol: `POST /api/v2/key/check`, `POST /api/v2/jobs`, `GET /api/v2/jobs/{job_id}`. These are separate from local control ports. |

Differences from older examples: status uses online/port; instances use nested objects, zero-based indices, and IDs refreshed on each scan; project returns a comps object without open_start IDs on effects; set_obj/set_svg return all slots; font is an id/family object. CLI resolve also accepts the documented flat instance-name shape. Other responses remain unchanged. The distribution includes this distilled reference and does not require the original project paths to exist.

## AE inspection and preview (CLI 1.2.0)

These are AE-side commands, independent of Start. Query `ae status` and inspect `capabilities` before relying on the new fields; older hosts lack them. Extended fields use `request ae --body-file request.json` (no `--allow-unknown` needed). Do not supply a fabricated property path: copy it from the properties result. Every returned index is zero-based.

```json
{"cmd":"status","diagnose":true,"timeout_ms":2000}
{"cmd":"project","target":"comp","comp_id":3}
{"cmd":"project","target":"layers","comp_id":3,"offset":0,"limit":200}
{"cmd":"project","target":"properties","comp_id":3,"layer_id":4,"depth":2,"limit":200,"sample_time":1}
{"cmd":"project","target":"keyframes","comp_id":3,"layer_id":4,"property_path":[{"index":5,"match_name":"ADBE Transform Group"},{"index":1,"match_name":"ADBE Position"}]}
{"cmd":"preview_frame","comp_id":3,"time":1,"path":"E:/output/new-frame.png"}
```

Each example above is a separate request file. `comp_id` is optional and defaults to the active composition; an explicit ID must match exactly, without fallback. IDs are positive 32-bit integers.

| project target | Required fields | Optional fields | Result |
|---|---|---|---|
| `comp` | `target` | `comp_id` | Composition ID/name, dimensions, pixel aspect, frame rate, duration/time, display start, work area, resolution factor, and layer count. |
| `layers` | `target` | `comp_id`, `offset`, `limit` | `layers,total,offset,truncated`; IDs/types, timing, switches, parent and source details. |
| `properties` | `target`, `layer_id` | `comp_id`, `property_path`, `depth`, `limit`, `sample_time` | Flat `properties,truncated`; full paths, types, sampled values, expression/error details, and keyframe counts. |
| `keyframes` | `target`, `layer_id`, nonempty `property_path` | `comp_id`, `offset`, `limit` | `keyframes,total,offset,truncated`; time/value, interpolation/ease, and applicable spatial tangent/Bezier/roving details. |

`limit` defaults to 200 and accepts 1..1000. `offset` defaults to 0 and applies only to layers/keyframes. `depth` defaults to 2 and accepts 1..8. A path has at most 32 segments, each containing a zero-based `index` and `match_name`; both are checked. Copy paths from returned properties, and reread them after edits. A group with `children_omitted:true` can be expanded using its path; `truncated:true` means the item limit was reached. Use bounded pagination for layers/keyframes instead of silently ignoring remaining items.

Sampling defaults to the composition's current time and evaluates expressions. Numeric/vector/color values are returned directly; TextDocument is summarized as text/font/font size; Shape includes vertices/tangents/closed state. Unsupported values report `sample.supported:false` with a reason; arbitrary SceneState is not decoded. Explicit sample/preview times must be finite and nonnegative; preview time must be less than composition duration.

preview_frame writes a new absolute PNG path whose parent exists; existing files are rejected. Use AE Preferences > Scripting & Expressions > Allow Scripts to Write Files and Access Network. This setting is not changed by the service. After AE returns, the HTTP worker waits up to 15 seconds for the PNG to finish; it never blocks AE idle on this file wait. The response includes actual PNG width/height, comp_width/comp_height, time and resolution_factor. Read the image after success; this is real AE composition output, not the ArbiFX instance cache. Multiple times are separate requests. The named `ae preview-frame --path ABSOLUTE.png` captures the current composition time; use raw JSON for selectors. A timeout does not cancel rendering. Failed captures can leave incomplete files; inspect and use a fresh path instead of replaying blindly.

status adds plugin_version, process_id, ae_version (null until a successful diagnosis) and capabilities. diagnose:true adds main_thread_responsive, scripting_available, script_file_write_allowed, active_comp, ae_version and elapsed_ms under diagnostic. Failure details include stage/error. Outer `ok:true` means the status request completed, not that the diagnostic passed; inspect diagnostic fields. `timeout_ms` defaults to 2000 and accepts 100..10000. A diagnostic timeout is a failed probe, not proof of a crashed AE. The existing HTTP listener handles requests serially: an in-flight command can delay status; timeout_ms bounds the probe after dispatch, not total network queueing. Ordinary status never invokes AE SDK. Script/preview work continues to execute on AE's idle/main thread.

The named preview command accepts a local output path, converts it to an absolute path, and checks the PNG extension, existing parent, and absence of an existing output, including during dry-run. It never creates directories or overwrites files. Raw requests preserve server-side paths and skip client filesystem checks; the host enforces its absolute-path and filesystem rules. Neither request form changes AE's file-writing preference.
