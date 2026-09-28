# ArbiFX Skill

[English](README.md) | [简体中文](README.zh-CN.md)

## Ask your AI to install it

If your AI assistant can access local files and run commands, copy and send this prompt:

```text
Please install the ArbiFX Skill from https://github.com/Oxoxxidane/ArbiFX_Skill.
```

## About

Let your AI operate ArbiFX: set assets and prompts, submit tasks and read their current status, save and load AFX files, inspect AE projects, and run scripts. The bundled standalone CLI supports 26 AE and Start HTTP commands, including API configuration status, detailed AE queries, and PNG preview export.

Before connecting, apply an ArbiFX (AFX) effect to a layer in AE or open a project containing it. Without this step, the connection will fail. Load the effect again after restarting AE.

## Install the skill

Clone this repository into a directory named `arbifx-http` inside your AI tool's skill location. For example, with Codex:

```sh
git clone https://github.com/Oxoxxidane/ArbiFX_Skill.git ~/.codex/skills/arbifx-http
```

If that directory already exists, update the existing checkout or use another location; do not overwrite an existing installation blindly. Other tools supporting Agent Skills can use the same folder. Keep `SKILL.md`, references, scripts, and the appropriate binaries together. The skill name is `arbifx-http`, independently of the GitHub repository name.

Start with [SKILL.md](SKILL.md) for agent instructions, [the CLI guide](references/cli.md) for command examples, or [the HTTP reference](references/http-api.md) for the full protocol.

## Run the CLI

The executables are self-contained: no Go, Python, Node, or third-party runtime installation is required.

| System | x64 (amd64) | ARM64 |
|---|---|---|
| Windows | `bin/windows-amd64/arbifx.exe` | `bin/windows-arm64/arbifx.exe` |
| macOS | `bin/darwin-amd64/arbifx` | `bin/darwin-arm64/arbifx` |
| Linux | `bin/linux-amd64/arbifx` | `bin/linux-arm64/arbifx` |

Windows, from the repository directory:

```powershell
.\bin\windows-amd64\arbifx.exe --json doctor --offline
& .\scripts\install.ps1
```

macOS/Linux:

```sh
sh ./scripts/arbifx.sh --json doctor --offline
sh ./scripts/install.sh
export PATH="$HOME/.local/bin:$PATH"
```

After installation, load an ArbiFX (AFX) effect instance once in AE. When a task needs Start, the AI locates the instance and opens the window automatically. For manual CLI use, open it with `arbifx ae open-start --id <instance-id>`:

```sh
arbifx --json doctor --target ae
arbifx --json ae instances
arbifx --json start get-prompt
arbifx --json start get-status
arbifx --json start set-prompt --prompt-file prompt.txt --parameters-file parameters.txt --dry-run
```

Remove `--dry-run` only when you intend to execute the action. Both services use `POST /command` on `127.0.0.1`: AE defaults to port `28154`, Start to `28153`. Local commands require no token. The CLI reads only local port settings, not generation-backend credentials.

**Host support:** the ArbiFX HTTP server inspected for this release is implemented only on Windows. macOS/Linux binaries provide client portability, not a port of the AE plugin or Start host. They can also use localhost ports forwarded to an authorized Windows host. macOS binaries are not Developer ID signed or notarized.

## Build and validate

Rebuilding requires Go 1.25+; the source uses only the standard library. No external Go modules are needed.

```sh
cd cli
go vet ./...
go test -v ./...
```

From the repository root, build all six platforms and package the skill, using an output directory outside the checkout:

```sh
go run ./scripts/release.go -root . -out ../arbifx-http-release
```

The builder runs source tests, compiles with `CGO_ENABLED=0`, and tests the native executable against isolated HTTP fixtures. See [the validation record](references/validation.md) for the distinction between tested native execution and cross-compilation.

## Repository contents

- `README.md` / `README.zh-CN.md`: English and Simplified Chinese guides.
- `SKILL.md`: portable agent instructions and metadata.
- `agents/openai.yaml`: optional Codex UI metadata.
- `bin/`: six ready-to-run executables.
- `cli/`: Go source and HTTP behavior tests.
- `scripts/`: launchers, installers, build wrappers, and release tooling.
- `references/`: protocol, CLI, validation, and Go runtime license documentation.

This repository does not contain the ArbiFX host/plugin source, personal configuration, credentials, or scene files. The Go runtime's redistribution notice is retained in [references/LICENSE-Go.txt](references/LICENSE-Go.txt).
