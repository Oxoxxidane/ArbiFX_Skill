# ArbiFX AE/Start Local HTTP Reference

Contents: transport and responses, five AE commands, eighteen Start commands, lifecycle, and implementation differences.

## Transport and responses

| Target | Default URL | Configuration key |
|---|---|---|
| AE | `http://127.0.0.1:28154/command` | `[local_http] ae_global_port` |
| Start | `http://127.0.0.1:28153/command` | `[local_http] start_port` |

Both targets accept `POST /command` with UTF-8 JSON and a required string `cmd`. No authentication headers are required, but the HTTP implementation requires an accurate `Content-Length`. The body limit is 1 MiB. The CLI supplies the UTF-8 byte length and JSON Content-Type; it uses no proxy, redirects, chunked uploads, or automatic retries.

Server success: `{"ok":true,"data":...}`. Business failures usually still use HTTP 200: `{"ok":false,"error":"message"}`. Always check JSON `ok`; for AE scripts also check `data.error`. See [Output and exit codes](cli.md#output-and-exit-codes) for CLI error envelopes.

## Five AE commands

| cmd / CLI | Required fields | Returned data and behavior |
|---|---|---|
| `status` / `ae status` | None | Current implementation: `{"online":true,"port":28154}`. Older documentation: `{"running":true}`. |
| `instances` / `ae instances` | None | Instance array: `id`, `comp:{id,name}`, `layer:{id,index,name}`, and `effect_index` in the current implementation. |
| `open_start` / `ae open-start` | Nonempty `id:string` | `{"opened":true}`, or `{"opened":false}` when already open for that instance. Does not rebind a different instance. |
| `script` / `ae script` | `code:string` | `{"result":"...","output":"...","error":""}`. Code is a function body supporting return and emit. |
| `project` / `ae project` | None | Current shape: `{"comps":[...]}`. Compositions contain id/name/layers; layers contain id/index/name/enabled/effects; effects contain index/name/match_name/enabled. |

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

## Eighteen Start commands

All fields below are top-level JSON properties. Example: `{"cmd":"set_obj","slot":0,"type":"obj","path":"C:\\asset\\model.obj"}`. CLI names replace `_` with `-` and also accept the original underscore spelling.

| cmd | Required fields | Returned data / behavior |
|---|---|---|
| `set_reference` | `path:string` | `{"file_name":"reference.png","loaded":true}`; immediately synchronizes the reference cache and UI. |
| `get_reference` | None | Same shape; empty name and loaded=false when absent. No path or file content is returned. |
| `set_obj` | `slot:int 0..7`, `type:string`, `path:string` | Currently returns all eight slots. Setting obj retains automatic MTL/texture loading; setting mtl retains associated texture refresh. |
| `get_obj` | None | Eight-slot array. Each slot has slot and obj/mtl/texture1/texture2/texture3; each resource is `{path,loaded}`. |
| `set_svg` | `slot:int 0..7`, `path:string` | Currently returns all eight slots, following the manual validation, persistence, and hot-reload workflow. |
| `get_svg` | None | Eight-slot array, each `{slot,path,loaded}`. |
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

### File rules

- Reference files, OBJ, and SVG: an empty or nonexistent path clears the resource. An existing but invalid, unreadable, oversized, or unloadable file fails and preserves the previous state. Clearing OBJ affects only the specified type. CLI `--clear` explicitly sends `path:""`.
- A reference must be a regular file larger than zero bytes and at most 20 MiB. The host reads the bytes; HTTP carries a path, not a base64 upload.
- Save/Load paths must be nonempty and should explicitly include `.afx`. Save requires an existing parent directory; Load requires an existing regular file. Neither supports clearing. Named CLI commands validate these conditions locally and convert relative paths to absolute paths.
- Save writes a file and may overwrite an existing one; dry-run reports `overwrite_existing_file`. Load changes the currently bound AE effect.
- Save excludes local OBJ/SVG slot resources from .afx. Load first removes package-local slots, then attaches the current Start window's local slots. These are the existing manual behaviors.
- Raw `request` preserves paths and skips local filesystem preflight. Use it when a path must be interpreted by the server's OS. With an SSH-forwarded remote host, do not normalize remote paths using named local file commands.

## Lifecycle and unavailable capabilities

The AE listener exists only after ArbiFX AEX GlobalSetup. To initialize it, load an ArbiFX (AFX) effect instance at least once in the current AE session: apply the effect to a layer or open a project containing it. Launching AE alone does not make the endpoint available, so connections fail until an instance has been loaded. After restarting AE, load an instance again. The Start listener additionally requires its window process to be alive. Port conflicts do not trigger automatic port changes. If multiple AE processes share the same configuration, only the first successful listener is available. A successful connection does not establish which AE process the user intends; inspect instances before operating.

Many Start writes fail while Start is busy; do not assume they can be queued or retried. `send` can also fail when no API key is configured, a pending task cannot be recovered, or the session cannot be written to AE. AE hot-reload work may still be queued after an HTTP response. The protocol has no dedicated Start status, font catalog, task polling/cancellation, window rebinding, complete parameter export, or arbitrary file download endpoint. The CLI does not invent these capabilities.

The macOS/Linux binaries provide client portability. At inspection time, the non-Windows branch of `src/common/src/local_http.cpp` returns `Local HTTP listener is only supported on Windows.`. This does not establish macOS/Linux support for the repository's AE/Start service. If the user has authorized and established an SSH local-port forward to a Windows host, the CLI can use the forwarded localhost ports. It does not establish tunnels or expose a network listener.

## Implementation differences and sources

Inspected on 2026-09-14 against the actual working-tree implementation, including pre-existing uncommitted changes. The client preserves original responses. No ArbiFX project source or protocol contract was changed.

Paths below are relative to the original ArbiFX3D source workspace. That workspace is not required to use this skill.

| Project document / implementation | Purpose |
|---|---|
| `docs/local-http-protocol-and-implementation-plan.zh-CN.md` | Main description of all five AE and eighteen Start HTTP commands and consistency with manual actions. |
| `src/ae_adapter/src/start_launcher.cpp` | `HandleAeGlobalHttpRequest`, `ProcessGlobalHttpRequest`, `ScanAeProject`, `GlobalHttpScriptWrapper`. |
| `apps/start_host/src/main.cpp` | `HandleStartHttpRequest`, `ExecuteStartHttpCommand`, `SelectedFontForUi`. |
| `src/common/src/local_http.cpp` | POST path, Content-Length, body limit, and server platform scope. |
| `config/config.toml.example` | Local port configuration. |
| `backend-api-arbifx-three-v2.zh-CN.md` | Start's outbound backend protocol: `POST /api/v2/key/check`, `POST /api/v2/jobs`, `GET /api/v2/jobs/{job_id}`. These are separate from local control ports. |

Differences from older examples: status uses online/port; instances use nested objects, zero-based indices, and IDs refreshed on each scan; project returns a comps object without open_start IDs on effects; set_obj/set_svg return all slots; font is an id/family object. CLI resolve also accepts the documented flat instance-name shape. Other responses remain unchanged. The distribution includes this distilled reference and does not require the original project paths to exist.
