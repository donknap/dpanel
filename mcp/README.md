# dpanel-mcp

DPanel 的 MCP (Model Context Protocol) 服务器。将 DPanel 的 Docker 管理能力暴露为 MCP 工具，供 AI Agent 调用。

参考 [portainer/portainer-mcp](https://github.com/portainer/portainer-mcp) 的架构设计（能力分级、脱敏、fail-closed 破坏性操作确认）。

## 架构

```
MCP Client (Claude/Cursor/Hermes...)
        │  stdio 或 streamable-http
        ▼
dpanel-mcp (Python + FastMCP)
   ├─ profiles.py   能力分级 read-only / read-write / admin
   ├─ redaction.py  敏感值脱敏（密码/token/secret）
   ├─ tools.py      80+ MCP 工具 → DPanel API
   └─ client.py     JWT 登录 + 自动续期
        │  HTTP POST /dpanel/api/*  (Authorization: Bearer JWT)
        ▼
DPanel (donknap/dpanel)
        │
        ▼
Docker Engine (docker.sock)
```

## 快速开始

### 1. 环境变量

```bash
export DPANEL_HOST=http://127.0.0.1:8807        # DPanel 访问地址
export DPANEL_USERNAME=admin                    # DPanel 管理员用户名
export DPANEL_PASSWORD=your-password            # DPanel 密码
export DPANEL_MCP_PROFILE=read-write            # read-only | read-write | admin
```

### 2. stdio 模式（本地单用户）

```bash
uvx --from ./mcp dpanel-mcp
```

Claude Desktop / Cursor 配置：

```json
{
  "mcpServers": {
    "dpanel": {
      "command": "uvx",
      "args": ["--from", "/path/to/dpanel/mcp", "dpanel-mcp"],
      "env": {
        "DPANEL_HOST": "http://127.0.0.1:8807",
        "DPANEL_USERNAME": "admin",
        "DPANEL_PASSWORD": "your-password"
      }
    }
  }
}
```

### 3. streamable-http 模式（容器/团队部署）

```bash
export DPANEL_MCP_TRANSPORT=streamable-http
export DPANEL_MCP_HTTP_HOST=0.0.0.0
export DPANEL_MCP_HTTP_PORT=8090
export DPANEL_MCP_AUTH_TOKEN=shared-secret     # 可选，门禁
uvx --from ./mcp dpanel-mcp
```

客户端配置：`{"url": "http://host:8090/mcp", "headers": {"Authorization": "Bearer shared-secret"}}`

## 工具分组

| 分组 | 工具数 | 说明 |
|---|---|---|
| system | 5 | 系统信息、资源使用、配置、日志 |
| env | 5 | Docker 环境（多主机） |
| container | 14 | 容器列表/详情/统计/生命周期/删除 |
| upgrade | 4 | 容器升级检查/升级/忽略 |
| backup | 5 | 容器备份/恢复 |
| compose | 10 | 项目列表/部署/控制/日志/销毁 |
| image | 11 | 镜像列表/tag/删除/清理 |
| image-build | 6 | 镜像构建任务 |
| network | 8 | 网络列表/创建/连接/删除 |
| volume | 5 | 卷列表/创建/删除 |
| explorer | 10 | 文件管理（volume/container/docker 挂载点） |
| cron | 8 | 计划任务（含模板） |
| store | 5 | 应用商店 |
| registry | 4 | 镜像仓库 |
| notice | 3 | 通知（操作事件流水） |

共 **99 个工具**。

## 能力分级

- **read-only**：仅查询类工具（列表/详情/统计/日志）
- **read-write**：+ 生命周期与创建（start/stop/restart/deploy/create）
- **admin**：+ 破坏性操作（delete/prune/restore/kill），且必须显式传 `confirm=true`

破坏性操作 fail-closed：不传 confirm 直接拒绝，不执行。

## 脱敏

所有工具返回值经递归脱敏：键名命中 password/token/secret/key/credential 等 → `******`；字符串中的 JWT 与 `SECRET=value` 形式的环境变量同样脱敏。

## 鉴权链

1. MCP server 启动后用 DPanel 管理员账号登录（`/common/user/login`，autoLogin=true → 30 天 JWT）
2. JWT 缓存，过期前 5 分钟自动重登
3. 401 时透明重登一次
4. streamable-http 模式下可加 `DPANEL_MCP_AUTH_TOKEN` 门禁（客户端需带 Bearer）

## 版本与兼容性

**对齐 DPanel 1.11.x**（版本号与 DPanel 一致）。所有路由与参数结构均已在真实 DPanel 1.11.0 容器上逐一验证（126 条路由探测 + compose 全生命周期端到端测试）。

| DPanel 版本 | 兼容性 |
|---|---|
| 1.11.x | ✅ 完全兼容（本版本目标） |
| 1.9.x–1.10.x | ⚠️ 部分路由已重命名（image-delete→delete、explorer 迁移至 common/、container-upgrade 独立模块），约 20 个工具不可用 |
| ≤1.8.x | ❌ 未验证 |

DPanel API 变更时同步升版本。遇到 `endpoint not found on DPanel` 错误时，说明 DPanel 版本与本 MCP 不匹配。
