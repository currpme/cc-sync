# ccsync

`ccsync` 是一个基于 Go 的 CLI 工具，用来通过 `WebDAV` 同步 `Codex` 和 `Claude Code` 的用户级 `skills`、顶层提示文件以及 `MCP` 配置。

当前版本面向个人开发者，优先解决这几个问题：

- 多台机器之间同步 Codex / Claude Code 的可版本化工作区素材
- 将用户自定义 `skills` 统一备份到 WebDAV
- 在新机器上快速恢复基础工作环境

## 当前能力

已实现的能力：

- 同步 `~/.codex/AGENTS.md` 和 `~/.claude/claude.md`
- 同步用户级 `skills`
- 同步工具基础目录顶层的 `MCP` 配置文件
- 使用 `WebDAV` 作为远端存储
- 支持本地扫描、远端对比、上传、拉取、同步和连通性检查

当前同步原则：

- 忽略 `Codex` 的系统级 `skill` 目录：`.codex/skills/.system`
- 只同步用户主目录下的 `~/.codex` 和 `~/.claude` / `~/.config/claude` / `~/.claude-code`
- 不同步项目目录下的任何配置、`skills` 或 `MCP`

## 项目结构

主要文件：

- [`cmd/ccsync/main.go`](/root/ccsync/cmd/ccsync/main.go)：CLI 入口
- [`internal/app/app.go`](/root/ccsync/internal/app/app.go)：命令分发
- [`internal/adapters/codex.go`](/root/ccsync/internal/adapters/codex.go)：Codex 适配器
- [`internal/adapters/claude.go`](/root/ccsync/internal/adapters/claude.go)：Claude Code 适配器
- [`internal/webdav/webdav.go`](/root/ccsync/internal/webdav/webdav.go)：WebDAV 客户端
- [`internal/syncer/remote.go`](/root/ccsync/internal/syncer/remote.go)：远端存储结构

## 安装与构建

环境要求：

- Go 1.22+
- 可访问的 WebDAV 服务

在项目目录下构建：

```bash
cd /root/ccsync
go build ./cmd/ccsync
```

构建完成后会生成：

```bash
/root/ccsync/ccsync
```

查看版本信息：

```bash
./ccsync version
```

## 发布包构建

项目内置了一个跨平台打包脚本，用来生成 GitHub Release 可直接上传的产物。

发布构建建议使用 Go `1.22+`，与仓库 CI 保持一致。

在项目目录下执行：

```bash
./scripts/build-release.sh 0.0.5
```

脚本会生成这些包：

- Linux `amd64`
- Linux `arm64`
- macOS `amd64`
- macOS `arm64`
- Windows `amd64`
- Windows `arm64`

输出目录：

```bash
dist/0.0.5/
```

目录中会包含：

- 每个平台对应的 `.tar.gz` 或 `.zip`
- `checksums.txt`

构建出的二进制可通过下面命令检查版本元数据：

```bash
./ccsync version
```

## 快速开始

### 1. 初始化配置

执行：

```bash
./ccsync init
```

程序会提示输入：

- WebDAV 地址
- WebDAV 用户名
- WebDAV 密码
- 远端根目录

默认配置文件位置：

```bash
~/.config/ccsync/config.toml
```

### 2. 检查本机和 WebDAV 状态

```bash
./ccsync doctor
```

这个命令会检查：

- 本机是否存在 `~/.codex`
- 本机是否存在 `~/.claude`
- WebDAV 是否可连接
- 远端根目录是否存在

### 3. 扫描本地可同步内容

```bash
./ccsync scan
```

只查看 Codex：

```bash
./ccsync scan --tool codex
```

以 JSON 输出：

```bash
./ccsync scan --format json
```

### 4. 上传到 WebDAV

```bash
./ccsync push
```

只上传 Codex：

```bash
./ccsync push --tool codex
```

### 5. 从 WebDAV 拉取到本地

```bash
./ccsync pull
```

`pull` 会把远端托管内容同步回本地，并清理本地托管范围内已被远端删除的文件。

### 6. 查看本地与远端差异

```bash
./ccsync diff
```

### 7. 交互式同步

```bash
./ccsync sync
```

默认行为：

- 先输出按文件的同步计划
- 先确认是否执行无冲突动作
- 冲突按文件逐项选择 `local` / `remote` / `skip`
- 删除动作需要单独确认

常用模式：

```bash
./ccsync sync --plan
./ccsync sync --prefer local
./ccsync sync --prefer remote
./ccsync sync --yes
./ccsync sync --allow-delete
```

## 命令说明

### `init`

初始化本地配置文件。

```bash
./ccsync init [--config /path/to/config.toml]
```

### `scan`

扫描本机可同步内容，不访问 WebDAV。

```bash
./ccsync scan [--tool codex|claude|all] [--format table|json]
```

### `doctor`

检查本地环境和 WebDAV 连接。

```bash
./ccsync doctor [--tool codex|claude|all]
```

### `diff`

对比本地扫描结果和 WebDAV 远端镜像。

```bash
./ccsync diff [--tool codex|claude|all]
```

### `push`

将本地托管内容上传到 WebDAV。

```bash
./ccsync push [--tool codex|claude|all]
```

### `pull`

从 WebDAV 下载托管内容并写回本地。

```bash
./ccsync pull [--tool codex|claude|all]
```

### `sync`

执行本地与远端对比，先生成按文件计划，再根据确认结果执行同步。

```bash
./ccsync sync [--tool codex|claude|all] [--prefer local|remote] [--plan] [--yes] [--allow-delete|--no-delete]
```

## 配置文件说明

示例：

```toml
[webdav]
url = "https://webdav.example.com/dav"
username = "demo"
password = "app-password"
password_cmd = ""

[remote]
root = "ccsync"

[sync]
manage_config = false
manage_instructions = true
manage_user_skills = true
manage_project_skills = false
manage_mcp = true
default_mode = "preview"
allow_delete = false

[scan]
project_roots = []

[conflict]
default_resolution = "prompt"
```

字段说明：

- `webdav.url`：WebDAV 根地址
- `webdav.username`：WebDAV 用户名
- `webdav.password`：WebDAV 密码或应用密码
- `webdav.password_cmd`：可选，用命令动态获取密码；只在运行时解析，不会被迁移逻辑回写成明文密码
- `remote.root`：远端同步根目录
- `sync.manage_config`：遗留字段，当前版本不再同步工具配置文件
- `sync.manage_instructions`：是否同步 `AGENTS.md` / `claude.md`
- `sync.manage_user_skills`：是否同步用户级 `skill`
- `sync.manage_project_skills`：遗留字段，当前版本不再同步项目级 `skill`
- `sync.manage_mcp`：是否同步 `MCP`
- `sync.default_mode`：默认同步模式，`preview` 表示先预览计划
- `sync.allow_delete`：是否默认允许删除动作进入计划
- `scan.project_roots`：遗留字段，当前版本忽略
- `conflict.default_resolution`：冲突默认策略，建议使用 `prompt`

## 远端存储结构

WebDAV 上的目录结构大致如下：

```text
ccsync/
  codex/
    manifest.json
    instructions/AGENTS.md
    skills/user/...
    mcp/...
  claude/
    manifest.json
    instructions/claude.md
    skills/user/...
    mcp/...
```

说明：

- 每个工具都有一个 `manifest.json`
- `manifest.json` 保存同步条目元数据
- 文件实际内容按相对路径存放

## 同步范围

### Codex

默认处理：

- `~/.codex/AGENTS.md`（如果存在）
- `~/.codex/skills` 下的用户自定义内容
- `~/.codex` 顶层文件名包含 `mcp` 的配置文件

忽略：

- `~/.codex/skills/.system`
- `auth.json`
- `history.jsonl`
- `logs_*.sqlite`
- `state_*.sqlite`
- `sessions/`
- `log/`
- 其他非托管运行时状态

### Claude Code

当前会尝试发现以下基础目录之一：

- `~/.claude`
- `~/.config/claude`
- `~/.claude-code`

默认处理：

- 顶层提示文件：`claude.md`（如果存在）
- `skills/`（例如 `~/.claude/skills`、`~/.config/claude/skills` 或 `~/.claude-code/skills`）
- 基础目录顶层文件名包含 `mcp` 的配置文件

## 配置写回策略

当前托管文件都按文件整体写回：

- `AGENTS.md`
- `claude.md`
- `skills/` 下的用户文件
- 顶层 `mcp*.(json|toml|yaml|yml)` 文件

注意：

- 不再同步 `config.toml`、`settings.json` 等工具配置文件
- 远端清单中的项目级条目和不受支持的路径会在本地侧被忽略

## 已验证内容

已经验证：

- `go build ./cmd/ccsync`
- `go test ./...`
- 本机 `Codex` 扫描
- 使用 123 盘 WebDAV 做连通性检查
- `push --tool codex`
- `diff --tool codex`

## 当前限制

当前版本仍有这些限制：

- 还没有真实 Claude Code 环境下的完整兼容性验证
- 远端暂未实现历史快照、版本回滚和远端文件枚举命令
- 冲突处理目前是工具级决策，不是逐条目交互式合并

## 建议用法

建议第一次使用按这个顺序：

```bash
./ccsync init
./ccsync doctor
./ccsync scan
./ccsync push --tool codex
./ccsync diff --tool codex
```

确认远端结构正确后，再开始同步更多目录和更多工具。
