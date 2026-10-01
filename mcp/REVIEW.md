# dpanel-mcp 代码 Review 报告

**Review 对象**：`leavrcn/dpanel` 分支 `feat/mcp-server`（commit `5476dcec`），上游 PR #325
**Review 方法**：主代理地面真值核查（源码 grep + live DPanel 1.11.0 容器实测 84 条路由 + 参数绑定验证）
**Review 日期**：2026-10-02

---

## 结论

**当前 PR 不可合并。** 发现 2 个 Blocker、5 个 Important、若干 Minor。核心问题：MCP 是基于 fork 时 master 分支（DPanel ~1.9.x）的 API 写的，而 live 1.11.0 已发生大量路由重命名/迁移，**89 个工具中 22 个调用的端点在 1.11.0 上不存在（返回 SPA HTML）**，另有多个工具参数结构与 live 不符。

---

## Blocker（必须修复）

### B1. 22 个工具指向不存在的路由（1.11.0 实测 404→SPA HTML）

实测方法：对 live 1.11.0 实例逐一 POST 我们 tools.py 里的 84 条路径，JSON 信封=存在，HTML=不存在。

**已确认失效（22 条）**：

| 我们调用的路径 | 1.11.0 真实路径 | 证据 |
|---|---|---|
| `/app/image/image-delete` | `/app/image/delete` | live 返回 `Md5为必填字段`（路由存在，参数为 `md5` 数组） |
| `/app/image/image-prune` | `/app/image/prune` | live 返回 `{"code":200,"data":"success"}` |
| `/app/image/build-prune` | `/app/image-build/prune` | live 返回 success |
| `/app/image/get-list-build` | `/app/image-build/get-list` | live 返回 `{"list":[]}` |
| `/app/image/check-upgrade` | 无独立路由（并入 `/app/container-upgrade/*`） | 二进制 strings 无此路径 |
| `/app/image/create-by-dockerfile` | `/app/image-build/create` | 二进制 strings |
| `/app/image/get-template-list` | 无（模板功能并入 image-build） | 二进制 strings |
| `/app/image/tag-remote` | `/app/image/tag-push-batch` | 二进制 strings |
| `/app/container/upgrade` | `/app/container-upgrade/upgrade` | live 验证：`md5` 必填 |
| `/app/container/ignore` | `/app/container-upgrade/ignore` | live 验证：`md5` 必填 |
| `/app/explorer/*`（8 条全部） | `/common/explorer/*` | live 验证：`MountPoint为必填字段` |
| `/app/compose/delete` | 无此路由（删除并入 `container-destroy`） | live 404 |
| `/app/compose/parse` | 无此路由 | live 404 |
| `/app/compose/container-process-kill` | 无此路由 | live 404 |
| `/common/event/get-list` | 无（1.11.0 移除或改名，二进制无字符串） | live 404 |
| `/common/event/prune` | 同上 | live 404 |

**影响**：Agent 调用这 22 个工具会收到整段 SPA HTML（`<!DOCTYPE html>...`）作为工具返回值——既不是可解析的错误，也不提示路由不存在。`_unwrap` 会抛 `non-JSON response`，但错误信息只有前 200 字符的 HTML，Agent 无法自愈。

### B2. 多个工具的参数结构与 live 1.11.0 不符

实测证据（live 1.11.0）：

| 工具 | 我们发送 | live 实际要求 |
|---|---|---|
| `dpanel_compose_task` | `{"name": ...}` | `{"id": ...}`（`Id为必填字段`；name 查询报 index out of range） |
| `dpanel_compose_log` | `{"name", "tail"}` | `{"id", "lineTotal"}`，且 lineTotal 枚举 `50/100/200/500/1000/5000/-1` |
| `dpanel_compose_container_ctrl` | `{"name", "operate"}` | `{"id", "op"}`，op 枚举 `start/restart/stop/pause/unpause/ls` |
| `dpanel_compose_deploy` | `{"name", "environment"}` | `{"id", "environment", "deployServiceName", "createPath", "removeOrphans"}` |
| `dpanel_compose_create` | `{"name", "content"}` | `{"name", "type", "yaml", "title", "id", "yamlOverride", "remoteUrl", "environment", "deployBackground"}`（type 必填） |
| `dpanel_compose_destroy` | `{"name"}` | `{"id"}` |
| `dpanel_image_delete` | `{"image": "nginx:latest"}` | `{"md5": ["sha256:..."]}`（镜像 digest 数组） |
| `dpanel_image_detail` | `{"image"}` | `{"image"}`（需复核 get-detail 的字段名） |
| `dpanel_container_list` | `{"page", "pageSize"}` | `{"md5", "siteTitle"}`（无分页参数！） |
| `dpanel_container_backup_create` | `{"md5", "option"}` | 需复核（1.11.0 backup 参数结构） |
| `dpanel_system_usage` | `{}` | `/common/home/usage` 存在但 1.11.0 新增 `/common/panel/usage`（含 diskUsage） |

**影响**：即使路由存在，参数错也会导致 500（`Id为必填字段`）或语义错误（compose 用 name 查不到任务）。

---

## Important（应修复）

### I1. `_unwrap` 对 SPA HTML 响应无路由级诊断

404 时 DPanel 返回 SPA HTML（HTTP 200）。当前抛 `non-JSON response (HTTP 200): <!DOCTYPE...`，Agent 看到的是 HTML 片段。应检测 `<!DOCTYPE html>` 并给出「路由不存在/版本不匹配，请检查 DPanel 版本 ≥1.11」的明确错误。

### I2. `dpanel_container_list` 无分页但传了分页参数

live 1.11.0 `get-list` 只接受 `md5`/`siteTitle` 过滤，无 page/pageSize。我们发 `{"page":1,"pageSize":50}` 会被忽略（gin 绑定宽松），返回全量列表——功能上能用但工具描述与实际行为不符（Agent 以为有分页）。且全量返回可能很大（本机 34KB/16 容器，大主机上 MB 级）。

### I3. compose 工具的 `name` vs `id` 语义混乱

1.11.0 compose 全线用 `id`（字符串），`name` 只是展示字段。我们的工具全部用 `name`。`get-list` 返回的 `list[].id` 全是 `"0"`（实测），说明 id 语义在 1.11.0 又有变化（可能是 name 本身或行 id）。需要以 live 行为为准重新设计参数。

### I4. `dpanel_explorer_*` 的 mountPoint 语义完全变了

1.11.0 explorer 需要 `mountPoint`（`volume:<name>` / `container:<md5>` / `docker:<env>`）+ `path`，不再直接接受 `md5`。我们的 8 个 explorer 工具参数全错。

### I5. 版本对齐声明与实际不符

README 声称「版本对齐 DPanel 版本（当前 1.9.2）」，但 pyproject 也是 1.9.2，而上游 master 已是 1.11.0。PR 描述说「will bump to match」——但工具集本身是按 1.9.x API 写的，对 1.11.x 用户 22 个工具直接坏掉。**必须声明兼容矩阵：本 MCP 基于 DPanel 1.9.x API，1.11.x 需适配**，或者直接按 1.11.0 重写路由层。

---

## Minor

- M1. `server.py` 的 streamable-http 分支里 `mcp._mcp_server`（noqa B018）是死代码，且 auth middleware 定义后未挂载（`app = mcp.http_app(...)` 后没有 add_middleware）——**实际上 bearer gate 没生效**（实测无 Authorization 也返回 200）。这其实接近 Important：README 声称有门禁但实际没有。
- M2. `tools.py` 中 `dpanel_container_status` 的 operate 枚举与 live 一致（start/stop/restart/pause/unpause）✅，但 `dpanel_compose_container_ctrl` 的 op 枚举文档写的是 `start|stop|restart|up|down`，live 是 `start|restart|stop|pause|unpause|ls`。
- M3. `redaction.py` 的 `_ENV_RE` 会把 `MYSQL_USER=root` 之外的 `*_KEY=`（如 `PATH_KEY=x`）也误伤，但可接受；`_is_sensitive_key` 的 `norm.endswith(k)` 会把 `monkey` 匹配到 `key`——`{"monkey": "..."}` 会被脱敏。低危但值得收紧为全词匹配。
- M4. `client.py` 的 `_decode_jwt_exp` 用 `float(exp)`，若 token 无 exp 返回 None 则回退假设 24h/30d——OK，但 autoLogin=false 时 DPanel 实际 24h，假设一致 ✅。
- M5. `tests/test_e2e.py` 的 `[7]` 检查注释说「expected API-level error, not gate error」但没断言错误内容不含「blocked: requires profile」——gate 与 API 错误未区分验证。
- M6. `pyproject.toml` `license = {text = "MIT"}`——上游 DPanel license 是 NOASSERTION（非标准 MIT），PR 里声明 MIT 需要确认与上游兼容。

---

## 验证证据存档

- live 实测：62 条路由存在 / 22 条返回 SPA HTML（`/tmp/our_paths.txt` 84 条逐一 POST）
- 参数绑定实测：compose 系列 `Id为必填字段`、image-delete `Md5为必填字段`、explorer `MountPoint为必填字段`
- mountPoint 格式实测：`volume:dpanel-mcp-test-data2` 返回真实文件列表 ✅
- 二进制 strings 交叉验证：`app/image/delete`、`app/container-upgrade/*`、`common/explorer/*` 均在 1.11.0 二进制中
- 测试容器与 volume 已清理

## 建议修复顺序

1. **决策**：目标版本对齐 1.11.0（推荐，上游活跃线）还是声明仅支持 1.9.x
2. 若对齐 1.11.0：按上表重写 22 条路由 + 修正参数结构（B1+B2），加路由探测 smoke test 到 CI
3. 修 M1（http auth gate 实际未挂载）
4. 修 I1（HTML 响应诊断）
5. 更新 README 兼容矩阵
