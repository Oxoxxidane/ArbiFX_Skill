# ArbiFX Skill

[English](README.md) | [简体中文](README.zh-CN.md)

用于控制 ArbiFX 的 [Agent Skill](https://agentskills.io/specification) 和独立命令行客户端，覆盖全部 **5 条 AE 端 + 18 条 Start 端**本机 HTTP 命令。

> **连接前必须先做：**在当前 After Effects 会话中，至少加载一次 ArbiFX（AFX）效果实例，例如将该效果添加到图层，或打开包含该效果的工程。这一步会初始化插件并启动 HTTP 监听服务。**只打开 AE 不够；没有加载过效果实例，CLI 就连接不上。**重启 AE 后需要再次加载实例。使用 Start 端命令时，还需要打开 Start 窗口。

CLI 支持实例发现与定位、工程检查、ExtendScript 执行、参考文件、OBJ/SVG 素材、文本、字体、标签、提示词、生成任务提交以及 AFX 保存与载入。提供 JSON 输出、离线预览和明确的错误码，不自动重试请求。

## 让 AI 自动安装

如果你的 AI 助手可以访问本机文件并执行命令，可以直接复制下面这段话发给它：

```text
请帮我安装 https://github.com/Oxoxxidane/ArbiFX_Skill 中的 ArbiFX Skill，将它放到当前 AI 工具的个人 Skill 目录，文件夹命名为 arbifx-http。请阅读 SKILL.md 和安装说明，根据我的操作系统和 CPU 选择仓库中已经编译好的 CLI，并安装 arbifx 命令，无需重新编译。如果已经安装，请在更新时保留本地修改。完成后运行 arbifx --version 和 arbifx --json doctor --offline 验证安装，并告诉我安装路径和使用方法。请提醒我：测试实际连接前，必须在当前 AE 会话中至少加载一次 ArbiFX（AFX）效果实例，否则 HTTP 服务尚未启动，会连接不上。离线验证不检查 AE 是否能连接。
```

## 安装 Skill

将本仓库克隆到 AI 工具的 Skill 目录下，并将文件夹命名为 `arbifx-http`。以 Codex 为例：

```sh
git clone https://github.com/Oxoxxidane/ArbiFX_Skill.git ~/.codex/skills/arbifx-http
```

如果目标目录已经存在，请更新已有仓库或使用其他位置，不要直接覆盖现有安装。其他支持 Agent Skills 的工具也可以使用同一文件夹。请保留 `SKILL.md`、参考文档、脚本及对应平台的可执行文件。Skill 名称为 `arbifx-http`，与 GitHub 仓库名称相互独立。

AI 操作指令见 [SKILL.md](SKILL.md)，命令示例见 [CLI 使用说明](references/cli.md)，完整协议见 [HTTP 接口参考](references/http-api.md)。

## 运行 CLI

可执行文件已独立封装，无需安装 Go、Python、Node 或第三方运行时。

| 系统 | x64（amd64） | ARM64 |
|---|---|---|
| Windows | `bin/windows-amd64/arbifx.exe` | `bin/windows-arm64/arbifx.exe` |
| macOS | `bin/darwin-amd64/arbifx` | `bin/darwin-arm64/arbifx` |
| Linux | `bin/linux-amd64/arbifx` | `bin/linux-arm64/arbifx` |

Windows：在仓库目录中运行：

```powershell
.\bin\windows-amd64\arbifx.exe --json doctor --offline
& .\scripts\install.ps1
```

macOS/Linux：

```sh
sh ./scripts/arbifx.sh --json doctor --offline
sh ./scripts/install.sh
export PATH="$HOME/.local/bin:$PATH"
```

安装后，先在 AE 中加载一次 ArbiFX（AFX）效果实例，再执行下面的实际连接命令。使用 Start 命令前还需要打开对应的 Start 窗口：

```sh
arbifx --json doctor --target ae
arbifx --json ae instances
arbifx --json start get-prompt
arbifx --json start set-prompt --prompt-file prompt.txt --parameters-file parameters.txt --dry-run
```

确定要执行操作时再去掉 `--dry-run`。两端均通过 `127.0.0.1` 上的 `POST /command` 接收命令：AE 默认端口为 `28154`，Start 默认端口为 `28153`。本机命令无需 token。CLI 只读取本机端口设置，不读取生成后端的登录凭据。

**宿主支持范围：**本次核对的 ArbiFX HTTP 服务端目前仅在 Windows 上实现。macOS/Linux 可执行文件提供跨平台客户端能力，并不包含 AE 插件或 Start 宿主的移植版。客户端也可使用经授权转发至 Windows 宿主的 localhost 端口。macOS 可执行文件尚未进行 Developer ID 签名或公证。

## 构建与验证

重新编译需要 Go 1.25+。源码仅使用标准库，不依赖外部 Go 模块。

```sh
cd cli
go vet ./...
go test -v ./...
```

回到仓库根目录，构建全部六个平台并打包 Skill；输出目录必须位于仓库目录之外：

```sh
go run ./scripts/release.go -root . -out ../arbifx-http-release
```

构建器会执行源码测试，以 `CGO_ENABLED=0` 编译，并使用隔离的 HTTP 模拟服务验证当前构建平台的可执行文件。已完成的实机运行测试与交叉编译范围见 [验证记录](references/validation.md)。仓库中的可执行文件校验值见 [SHA256SUMS.txt](SHA256SUMS.txt)。

## 仓库内容

- `README.md` / `README.zh-CN.md`：英文与简体中文使用说明。
- `SKILL.md`：可移植的 AI 操作指令和元数据。
- `agents/openai.yaml`：可选的 Codex 界面元数据。
- `bin/`：六个平台的即用可执行文件。
- `cli/`：Go 源码和 HTTP 行为测试。
- `scripts/`：启动器、安装脚本、构建入口和发布工具。
- `references/`：协议、CLI、验证及 Go 运行库许可文档。
- `SHA256SUMS.txt`：仓库中可执行文件的校验值。

本仓库不包含 ArbiFX 宿主或插件源码、个人配置、登录凭据及场景文件。Go 运行库的再分发声明保留在 [references/LICENSE-Go.txt](references/LICENSE-Go.txt) 中。
