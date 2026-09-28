# ArbiFX Skill

[English](README.md) | [简体中文](README.zh-CN.md)

## 让 AI 自动安装

请帮我安装 [https://github.com/Oxoxxidane/ArbiFX_Skill](https://github.com/Oxoxxidane/ArbiFX_Skill) 中的 ArbiFX Skill。

## 简介

让 AI 直接操作 ArbiFX，完成素材设置、提示词输入、任务发送与当前状态回读、AFX 保存载入，也能查询 AE 工程、执行脚本。附带独立 CLI，支持 AE 和 Start 的 26 个 HTTP 命令，包含 API 配置状态、详细工程查询与合成 PNG 导出。

使用前，先在 AE 的图层上添加一次 ArbiFX（AFX）效果，或打开包含该效果的工程，否则无法连接。重启 AE 后需要重新加载。

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

安装后，先在 AE 中加载一次 ArbiFX（AFX）效果实例。需要操作 Start 时，AI 会定位实例并自动打开窗口。手动使用 CLI 时，可用 `arbifx ae open-start --id <实例ID>` 打开：

```sh
arbifx --json doctor --target ae
arbifx --json ae instances
arbifx --json start get-prompt
arbifx --json start get-status
arbifx --json start set-prompt --prompt-file prompt.txt --parameters-file parameters.txt --dry-run
```

确定要执行操作时再去掉 `--dry-run`。两端均通过 `127.0.0.1` 上的 `POST /command` 接收命令：AE 默认端口为 `28154`，Start 默认端口为 `28153`。本机命令无需 token。CLI 只读取本机端口设置，不读取生成后端的登录凭据。

`start get-status` 返回 `busy`、`status_text` 和 `pending_job`，忙碌期间也可查询。它是当前状态快照，不保留历史；`busy:false`、`pending_job:null` 均不能单独证明成功。旧版 Start 若返回未知命令，需要更新宿主。详细含义见 [状态回读说明](references/http-api.md#task-status-snapshot)。

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

构建器会执行源码测试，以 `CGO_ENABLED=0` 编译，并使用隔离的 HTTP 模拟服务验证当前构建平台的可执行文件。已完成的实机运行测试与交叉编译范围见 [验证记录](references/validation.md)。

## 仓库内容

- `README.md` / `README.zh-CN.md`：英文与简体中文使用说明。
- `SKILL.md`：可移植的 AI 操作指令和元数据。
- `agents/openai.yaml`：可选的 Codex 界面元数据。
- `bin/`：六个平台的即用可执行文件。
- `cli/`：Go 源码和 HTTP 行为测试。
- `scripts/`：启动器、安装脚本、构建入口和发布工具。
- `references/`：协议、CLI、验证及 Go 运行库许可文档。

本仓库不包含 ArbiFX 宿主或插件源码、个人配置、登录凭据及场景文件。Go 运行库的再分发声明保留在 [references/LICENSE-Go.txt](references/LICENSE-Go.txt) 中。
