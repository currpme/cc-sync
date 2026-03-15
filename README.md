# ccsync

`ccsync` 是一个基于 Go 的 CLI 工具，用来通过 `WebDAV` 同步 `Codex` 和 `Claude Code` 的基础配置、用户级/项目级 `skill` 以及 `MCP` 配置。

当前版本面向个人开发者，优先解决这几个问题：

- 多台机器之间同步 Codex / Claude Code 的可版本化配置
- 将用户自定义 `skills` 统一备份到 WebDAV
- 在新机器上快速恢复基础工作环境

## 当前能力

已实现的能力：

- 同步 `Codex` 和 `Claude Code` 的基础配置文件
- 同步用户级 `skills`
- 同步项目级 `skills`
- 同步 `MCP` 配置文件
- 使用 `WebDAV` 作为远端存储
- 支持本地扫描、远端对比、上传、拉取、同步和连通性检查

当前同步原则：

- 不同步 `API Key`、`token`、`password` 等敏感字段
- 不同步认证态、日志、历史记录、缓存、sqlite 状态文件
- 忽略 `Codex` 的系统级 `skill` 目录：`.codex/skills/.system`
- 项目级 `skill` 只扫描你显式配置的 `project_roots`

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
./scripts/build-release.sh 0.0.2
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
dist/0.0.2/
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
- 项目扫描根目录

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
manage_config = true
manage_user_skills = true
manage_project_skills = true
manage_mcp = true
default_mode = "preview"
allow_delete = false

[scan]
project_roots = ["/Users/name/work", "/Users/name/projects"]

[conflict]
default_resolution = "prompt"
```

字段说明：

- `webdav.url`：WebDAV 根地址
- `webdav.username`：WebDAV 用户名
- `webdav.password`：WebDAV 密码或应用密码
- `webdav.password_cmd`：可选，用命令动态获取密码
- `remote.root`：远端同步根目录
- `sync.manage_config`：是否同步基础配置
- `sync.manage_user_skills`：是否同步用户级 `skill`
- `sync.manage_project_skills`：是否同步项目级 `skill`
- `sync.manage_mcp`：是否同步 `MCP`
- `sync.default_mode`：默认同步模式，`preview` 表示先预览计划
- `sync.allow_delete`：是否默认允许删除动作进入计划
- `scan.project_roots`：项目级扫描根目录列表
- `conflict.default_resolution`：冲突默认策略，建议使用 `prompt`

## 远端存储结构

WebDAV 上的目录结构大致如下：

```text
ccsync/
  codex/
    manifest.json
    config/config.toml
    config/config.toml.bak
    skills/user/...
    skills/projects/<project-ref>/...
    mcp/...
  claude/
    manifest.json
    config/settings.json
    config/settings.json.bak
    skills/user/...
    skills/projects/<project-ref>/...
    mcp/...
```

说明：

- 每个工具都有一个 `manifest.json`
- `manifest.json` 保存同步条目元数据
- 文件实际内容按相对路径存放

## 同步范围

### Codex

默认处理：

- `~/.codex/config.toml`
- `~/.codex/config.toml.*` 这类派生配置文件，例如 `config.toml.bak`
- `~/.codex/skills` 下的用户自定义内容
- `~/.codex` 下文件名包含 `mcp` 的配置文件
- 项目目录中的 `.codex/skills`

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

- 配置文件：`config.toml`、`settings.json`、`settings.toml` 及其 `.*` 派生文件，例如 `settings.json.bak`
- `skills/`
- 文件名包含 `mcp` 的配置文件
- 项目目录中的 `.claude/skills`

## 配置写回策略

基础配置不是简单整文件覆盖，而是：

- 对托管字段执行更新
- 尽量保留已有文件中的非托管内容
- 对 `TOML` 和 `JSON` 做基础合并

注意：

- 当前合并逻辑是轻量实现，适合常见配置场景
- 如果目标文件结构非常复杂，建议先 `diff` 再 `pull`

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
- 敏感字段过滤采用关键字过滤，不是语义级解析
- 远端暂未实现历史快照、版本回滚和远端文件枚举命令
- 冲突处理目前是工具级决策，不是逐条目交互式合并
- 项目级 `skill` 需要手动配置 `project_roots` 才会扫描

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
