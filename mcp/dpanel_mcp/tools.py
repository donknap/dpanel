"""MCP tools bridging to the DPanel panel API.

Every tool:
  1. checks the profile gate (profiles.py)
  2. destructive tools require an explicit confirm=True argument
     (fail-closed human-in-the-loop; frozen decision 8A)
  3. calls the DPanel API via client.post()
  4. redacts sensitive values before returning (frozen decision 9B)
"""

from __future__ import annotations

import json
from typing import Any

from fastmcp import FastMCP

from .client import DPanelApiError, DPanelClient
from .config import Config
from .profiles import ProfileGate
from .redaction import redact

mcp = FastMCP("dpanel-mcp")

_client: DPanelClient | None = None
_gate: ProfileGate | None = None


def bind(config: Config) -> DPanelClient:
    """Create the shared client + gate. Called once at startup."""
    global _client, _gate
    _client = DPanelClient(config)
    _gate = ProfileGate(config)
    return _client


# --------------------------------------------------------------------- utils

def _check(tool: str, confirm: bool = False) -> None:
    if _gate is None:
        raise RuntimeError("server not initialized; call bind() first")
    if not _gate.allowed(tool):
        raise DPanelApiError(_gate.explain_denial(tool))
    if _gate.is_destructive(tool) and not confirm:
        raise DPanelApiError(
            f"{tool} is a destructive operation. Re-send the call with confirm=true "
            f"after reviewing its impact. (fail-closed)"
        )


async def _call(path: str, payload: dict | None = None) -> Any:
    assert _client is not None
    data = await _client.post(path, payload)
    return redact(data)


def _json(data: Any) -> str:
    return json.dumps(data, ensure_ascii=False, default=str, indent=2)


# ===================================================================== system

@mcp.tool
async def dpanel_system_info() -> str:
    """DPanel 系统信息：版本、运行环境、访问地址、安全入口。"""
    _check("dpanel_system_info")
    return _json(await _call("/common/home/info", {}))


@mcp.tool
async def dpanel_system_usage() -> str:
    """DPanel 所在主机资源使用情况（CPU/内存/磁盘）。"""
    _check("dpanel_system_usage")
    return _json(await _call("/common/home/usage", {}))


@mcp.tool
async def dpanel_system_stat_list() -> str:
    """DPanel 统计列表（多主机数据收集）。"""
    _check("dpanel_system_stat_list")
    return _json(await _call("/common/home/get-stat-list", {}))


@mcp.tool
async def dpanel_setting_get(group: str = "", key: str = "") -> str:
    """读取 DPanel 配置项。group 可选 setting/user；key 为配置键。"""
    _check("dpanel_setting_get")
    payload = {"group": group, "key": key} if group or key else {}
    return _json(await _call("/common/setting/get-setting", payload))


# ================================================================ environments

@mcp.tool
async def dpanel_env_list() -> str:
    """列出 Docker 环境（多主机）。"""
    _check("dpanel_env_list")
    return _json(await _call("/common/env/get-list", {}))


@mcp.tool
async def dpanel_env_switch(env_name: str) -> str:
    """切换当前 Docker 环境（多主机）。"""
    _check("dpanel_env_switch")
    return _json(await _call("/common/env/switch", {"name": env_name}))


@mcp.tool
async def dpanel_env_create(name: str, title: str, address: str) -> str:
    """创建 Docker 环境（连接远程 Docker host）。"""
    _check("dpanel_env_create")
    return _json(
        await _call("/common/env/create", {"name": name, "title": title, "address": address})
    )


@mcp.tool
async def dpanel_env_delete(env_name: str, confirm: bool = False) -> str:
    """删除 Docker 环境（破坏性，需 confirm=true）。"""
    _check("dpanel_env_delete", confirm)
    return _json(await _call("/common/env/delete", {"name": env_name}))


# ================================================================== containers

@mcp.tool
async def dpanel_container_list(env_name: str = "local", page: int = 1, page_size: int = 50) -> str:
    """容器列表（含状态、镜像、名称、网络）。env_name 为 Docker 环境名，默认 local。"""
    _check("dpanel_container_list")
    return _json(
        await _call(
            "/app/container/get-list",
            {"envName": env_name, "page": page, "pageSize": page_size},
        )
    )


@mcp.tool
async def dpanel_container_detail(md5: str) -> str:
    """容器详情。md5 为容器标识（列表返回的 md5/id 字段）。"""
    _check("dpanel_container_detail")
    return _json(await _call("/app/container/get-detail", {"md5": md5}))


@mcp.tool
async def dpanel_container_stat(md5: str) -> str:
    """容器资源统计（CPU/内存/网络 IO）。"""
    _check("dpanel_container_stat")
    return _json(await _call("/app/container/get-stat-info", {"md5": md5}))


@mcp.tool
async def dpanel_container_process(md5: str) -> str:
    """容器内进程列表。"""
    _check("dpanel_container_process")
    return _json(await _call("/app/container/get-process-info", {"md5": md5}))


@mcp.tool
async def dpanel_container_status(md5: str, operate: str) -> str:
    """容器生命周期控制：operate ∈ start|stop|restart|pause|unpause。"""
    _check("dpanel_container_status")
    return _json(await _call("/app/container/status", {"md5": md5, "operate": operate}))


@mcp.tool
async def dpanel_container_update(md5: str, option: dict) -> str:
    """更新容器配置（重启策略/环境变量/端口等，option 结构与面板一致）。"""
    _check("dpanel_container_update")
    return _json(await _call("/app/container/update", {"md5": md5, "option": option}))


@mcp.tool
async def dpanel_container_upgrade(md5: str, image: str = "") -> str:
    """升级容器（拉新镜像并重建，保留配置）。image 留空则用原镜像名。"""
    _check("dpanel_container_upgrade")
    payload = {"md5": md5}
    if image:
        payload["image"] = image
    return _json(await _call("/app/container/upgrade", payload))


@mcp.tool
async def dpanel_container_copy(md5: str, option: dict) -> str:
    """复制容器（以现有容器为模板创建新容器）。"""
    _check("dpanel_container_copy")
    return _json(await _call("/app/container/copy", {"md5": md5, "option": option}))


@mcp.tool
async def dpanel_container_ignore(md5: str, option: dict) -> str:
    """设置容器忽略项（升级忽略等）。"""
    _check("dpanel_container_ignore")
    return _json(await _call("/app/container/ignore", {"md5": md5, "option": option}))


@mcp.tool
async def dpanel_container_export(md5: str) -> str:
    """导出容器为镜像 tar 信息。"""
    _check("dpanel_container_export")
    return _json(await _call("/app/container/export", {"md5": md5}))


@mcp.tool
async def dpanel_container_commit(md5: str, option: dict) -> str:
    """将容器提交为新镜像。"""
    _check("dpanel_container_commit")
    return _json(await _call("/app/container/commit", {"md5": md5, "option": option}))


@mcp.tool
async def dpanel_container_delete(md5: str, confirm: bool = False) -> str:
    """删除容器（破坏性，需 confirm=true）。"""
    _check("dpanel_container_delete", confirm)
    return _json(await _call("/app/container/delete", {"md5": md5}))


@mcp.tool
async def dpanel_container_prune(confirm: bool = False) -> str:
    """清理停止的容器（破坏性，需 confirm=true）。"""
    _check("dpanel_container_prune", confirm)
    return _json(await _call("/app/container/prune", {}))


# ==================================================================== backups

@mcp.tool
async def dpanel_container_backup_list() -> str:
    """容器备份列表。"""
    _check("dpanel_container_backup_list")
    return _json(await _call("/app/container-backup/get-list", {}))


@mcp.tool
async def dpanel_container_backup_detail(id: int) -> str:
    """备份详情。id 为备份记录 id。"""
    _check("dpanel_container_backup_detail")
    return _json(await _call("/app/container-backup/get-detail", {"id": id}))


@mcp.tool
async def dpanel_container_backup_create(md5: str, option: dict | None = None) -> str:
    """创建容器备份快照。"""
    _check("dpanel_container_backup_create")
    return _json(await _call("/app/container-backup/create", {"md5": md5, "option": option or {}}))


@mcp.tool
async def dpanel_container_backup_delete(id: int, confirm: bool = False) -> str:
    """删除备份（破坏性，需 confirm=true）。"""
    _check("dpanel_container_backup_delete", confirm)
    return _json(await _call("/app/container-backup/delete", {"id": id}))


@mcp.tool
async def dpanel_container_backup_restore(id: int, confirm: bool = False) -> str:
    """从备份恢复容器（破坏性，需 confirm=true）。"""
    _check("dpanel_container_backup_restore", confirm)
    return _json(await _call("/app/container-backup/restore", {"id": id}))


# ===================================================================== compose

@mcp.tool
async def dpanel_compose_list() -> str:
    """Compose 项目列表。"""
    _check("dpanel_compose_list")
    return _json(await _call("/app/compose/get-list", {}))


@mcp.tool
async def dpanel_compose_task(name: str) -> str:
    """Compose 项目任务状态（部署/构建进度）。"""
    _check("dpanel_compose_task")
    return _json(await _call("/app/compose/get-task", {"name": name}))


@mcp.tool
async def dpanel_compose_log(name: str, tail: int = 200) -> str:
    """Compose 项目日志。"""
    _check("dpanel_compose_log")
    return _json(await _call("/app/compose/container-log", {"name": name, "tail": tail}))


@mcp.tool
async def dpanel_compose_parse(content: str) -> str:
    """解析 compose YAML 内容（校验 + 结构预览）。"""
    _check("dpanel_compose_parse")
    return _json(await _call("/app/compose/parse", {"content": content}))


@mcp.tool
async def dpanel_compose_get_from_uri(uri: str) -> str:
    """从 Git/URL 拉取 compose 文件内容。"""
    _check("dpanel_compose_get_from_uri")
    return _json(await _call("/app/compose/get-from-uri", {"uri": uri}))


@mcp.tool
async def dpanel_compose_create(name: str, content: str, environment: str = "") -> str:
    """创建 compose 项目。content 为完整 docker-compose YAML。"""
    _check("dpanel_compose_create")
    payload = {"name": name, "content": content}
    if environment:
        payload["environment"] = environment
    return _json(await _call("/app/compose/create", payload))


@mcp.tool
async def dpanel_compose_deploy(name: str, environment: str = "") -> str:
    """部署/更新 compose 项目。environment 为环境变量名（可选）。"""
    _check("dpanel_compose_deploy")
    payload = {"name": name}
    if environment:
        payload["environment"] = environment
    return _json(await _call("/app/compose/container-deploy", payload))


@mcp.tool
async def dpanel_compose_container_ctrl(name: str, operate: str) -> str:
    """Compose 项目容器控制：operate ∈ start|stop|restart|up|down。"""
    _check("dpanel_compose_container_ctrl")
    return _json(await _call("/app/compose/container-ctrl", {"name": name, "operate": operate}))


@mcp.tool
async def dpanel_compose_delete(name: str, confirm: bool = False) -> str:
    """删除 compose 项目（破坏性，需 confirm=true）。"""
    _check("dpanel_compose_delete", confirm)
    return _json(await _call("/app/compose/delete", {"name": name}))


@mcp.tool
async def dpanel_compose_destroy(name: str, confirm: bool = False) -> str:
    """销毁 compose 项目及其容器（破坏性，需 confirm=true）。"""
    _check("dpanel_compose_destroy", confirm)
    return _json(await _call("/app/compose/container-destroy", {"name": name}))


@mcp.tool
async def dpanel_compose_process_kill(name: str, pid: int, confirm: bool = False) -> str:
    """杀死 compose 容器内进程（破坏性，需 confirm=true）。"""
    _check("dpanel_compose_process_kill", confirm)
    return _json(await _call("/app/compose/container-process-kill", {"name": name, "pid": pid}))


# ====================================================================== image

@mcp.tool
async def dpanel_image_list(page: int = 1, page_size: int = 50) -> str:
    """镜像列表。"""
    _check("dpanel_image_list")
    return _json(await _call("/app/image/get-list", {"page": page, "pageSize": page_size}))


@mcp.tool
async def dpanel_image_detail(image_name: str) -> str:
    """镜像详情。image_name 形如 nginx:latest。"""
    _check("dpanel_image_detail")
    return _json(await _call("/app/image/get-detail", {"image": image_name}))


@mcp.tool
async def dpanel_image_check_upgrade() -> str:
    """检查镜像更新（对比本地与远程 digest）。"""
    _check("dpanel_image_check_upgrade")
    return _json(await _call("/app/image/check-upgrade", {}))


@mcp.tool
async def dpanel_image_template_list() -> str:
    """Dockerfile 模板列表。"""
    _check("dpanel_image_template_list")
    return _json(await _call("/app/image/get-template-list", {}))


@mcp.tool
async def dpanel_image_build_task() -> str:
    """镜像构建任务列表。"""
    _check("dpanel_image_build_task")
    return _json(await _call("/app/image/get-list-build", {}))


@mcp.tool
async def dpanel_image_create_by_dockerfile(name: str, dockerfile: str) -> str:
    """从 Dockerfile 构建镜像。"""
    _check("dpanel_image_create_by_dockerfile")
    return _json(
        await _call("/app/image/create-by-dockerfile", {"name": name, "dockerfile": dockerfile})
    )


@mcp.tool
async def dpanel_image_tag_add(image_name: str, tag: str) -> str:
    """为镜像添加 tag。"""
    _check("dpanel_image_tag_add")
    return _json(await _call("/app/image/tag-add", {"image": image_name, "tag": tag}))


@mcp.tool
async def dpanel_image_tag_remote(image_name: str, registry: str) -> str:
    """将镜像 tag 推送到远程仓库。"""
    _check("dpanel_image_tag_remote")
    return _json(
        await _call("/app/image/tag-remote", {"image": image_name, "registry": registry})
    )


@mcp.tool
async def dpanel_image_tag_sync(image_name: str) -> str:
    """同步镜像 tag 到远程。"""
    _check("dpanel_image_tag_sync")
    return _json(await _call("/app/image/tag-sync", {"image": image_name}))


@mcp.tool
async def dpanel_image_import(image_name: str, path: str) -> str:
    """导入镜像 tar。"""
    _check("dpanel_image_import")
    return _json(await _call("/app/image/import-by-image-tar", {"image": image_name, "path": path}))


@mcp.tool
async def dpanel_image_tag_delete(image_name: str, confirm: bool = False) -> str:
    """删除镜像 tag（破坏性，需 confirm=true）。"""
    _check("dpanel_image_tag_delete", confirm)
    return _json(await _call("/app/image/tag-delete", {"image": image_name}))


@mcp.tool
async def dpanel_image_delete(image_name: str, confirm: bool = False) -> str:
    """删除镜像（破坏性，需 confirm=true）。"""
    _check("dpanel_image_delete", confirm)
    return _json(await _call("/app/image/image-delete", {"image": image_name}))


@mcp.tool
async def dpanel_image_prune(confirm: bool = False) -> str:
    """清理悬空镜像（破坏性，需 confirm=true）。"""
    _check("dpanel_image_prune", confirm)
    return _json(await _call("/app/image/image-prune", {}))


@mcp.tool
async def dpanel_image_build_prune(confirm: bool = False) -> str:
    """清理构建缓存（破坏性，需 confirm=true）。"""
    _check("dpanel_image_build_prune", confirm)
    return _json(await _call("/app/image/build-prune", {}))


# ==================================================================== network

@mcp.tool
async def dpanel_network_list() -> str:
    """网络列表。"""
    _check("dpanel_network_list")
    return _json(await _call("/app/network/get-list", {}))


@mcp.tool
async def dpanel_network_detail(network_name: str) -> str:
    """网络详情。"""
    _check("dpanel_network_detail")
    return _json(await _call("/app/network/get-detail", {"name": network_name}))


@mcp.tool
async def dpanel_network_container_list(network_name: str) -> str:
    """网络上的容器列表。"""
    _check("dpanel_network_container_list")
    return _json(await _call("/app/network/get-container-list", {"name": network_name}))


@mcp.tool
async def dpanel_network_create(name: str, driver: str = "bridge") -> str:
    """创建网络。"""
    _check("dpanel_network_create")
    return _json(await _call("/app/network/create", {"name": name, "driver": driver}))


@mcp.tool
async def dpanel_network_connect(network_name: str, container_md5: str) -> str:
    """将容器接入网络。"""
    _check("dpanel_network_connect")
    return _json(
        await _call(
            "/app/network/connect", {"name": network_name, "md5": container_md5}
        )
    )


@mcp.tool
async def dpanel_network_disconnect(network_name: str, container_md5: str) -> str:
    """将容器从网络断开。"""
    _check("dpanel_network_disconnect")
    return _json(
        await _call("/app/network/disconnect", {"name": network_name, "md5": container_md5})
    )


@mcp.tool
async def dpanel_network_delete(network_name: str, confirm: bool = False) -> str:
    """删除网络（破坏性，需 confirm=true）。"""
    _check("dpanel_network_delete", confirm)
    return _json(await _call("/app/network/delete", {"name": network_name}))


@mcp.tool
async def dpanel_network_prune(confirm: bool = False) -> str:
    """清理未使用网络（破坏性，需 confirm=true）。"""
    _check("dpanel_network_prune", confirm)
    return _json(await _call("/app/network/prune", {}))


# ===================================================================== volume

@mcp.tool
async def dpanel_volume_list() -> str:
    """卷列表。"""
    _check("dpanel_volume_list")
    return _json(await _call("/app/volume/get-list", {}))


@mcp.tool
async def dpanel_volume_detail(volume_name: str) -> str:
    """卷详情。"""
    _check("dpanel_volume_detail")
    return _json(await _call("/app/volume/get-detail", {"name": volume_name}))


@mcp.tool
async def dpanel_volume_create(name: str, driver: str = "local") -> str:
    """创建卷。"""
    _check("dpanel_volume_create")
    return _json(await _call("/app/volume/create", {"name": name, "driver": driver}))


@mcp.tool
async def dpanel_volume_delete(volume_name: str, confirm: bool = False) -> str:
    """删除卷（破坏性，需 confirm=true）。"""
    _check("dpanel_volume_delete", confirm)
    return _json(await _call("/app/volume/delete", {"name": volume_name}))


@mcp.tool
async def dpanel_volume_prune(confirm: bool = False) -> str:
    """清理未使用卷（破坏性，需 confirm=true）。"""
    _check("dpanel_volume_prune", confirm)
    return _json(await _call("/app/volume/prune", {}))


# =================================================================== explorer

@mcp.tool
async def dpanel_explorer_list(md5: str, path: str = "/") -> str:
    """容器内文件列表。md5 为容器标识，path 为容器内路径。"""
    _check("dpanel_explorer_list")
    return _json(await _call("/app/explorer/get-path-list", {"md5": md5, "path": path}))


@mcp.tool
async def dpanel_explorer_content(md5: str, file: str) -> str:
    """读取容器内文件内容。"""
    _check("dpanel_explorer_content")
    return _json(await _call("/app/explorer/get-content", {"md5": md5, "file": file}))


@mcp.tool
async def dpanel_explorer_stat(md5: str, file: str) -> str:
    """容器内文件 stat。"""
    _check("dpanel_explorer_stat")
    return _json(await _call("/app/explorer/get-file-stat", {"md5": md5, "file": file}))


@mcp.tool
async def dpanel_explorer_export(md5: str, file_list: list[str]) -> str:
    """导出容器内文件（打包信息）。"""
    _check("dpanel_explorer_export")
    return _json(await _call("/app/explorer/export", {"md5": md5, "fileList": file_list}))


@mcp.tool
async def dpanel_explorer_import(md5: str, file_list: list[dict], dest_path: str) -> str:
    """导入文件到容器。file_list: [{name, path}]。"""
    _check("dpanel_explorer_import")
    return _json(
        await _call(
            "/app/explorer/import", {"md5": md5, "fileList": file_list, "destPath": dest_path}
        )
    )


@mcp.tool
async def dpanel_explorer_unzip(md5: str, file: str, path: str) -> str:
    """解压容器内压缩文件。"""
    _check("dpanel_explorer_unzip")
    return _json(await _call("/app/explorer/unzip", {"md5": md5, "file": file, "path": path}))


@mcp.tool
async def dpanel_explorer_chmod(md5: str, file_list: list[str], mod: str) -> str:
    """修改容器内文件权限。"""
    _check("dpanel_explorer_chmod")
    return _json(await _call("/app/explorer/chmod", {"md5": md5, "fileList": file_list, "mod": mod}))


@mcp.tool
async def dpanel_explorer_delete(md5: str, file_list: list[str], confirm: bool = False) -> str:
    """删除容器内文件（破坏性，需 confirm=true）。"""
    _check("dpanel_explorer_delete", confirm)
    return _json(await _call("/app/explorer/delete", {"md5": md5, "fileList": file_list}))


# ======================================================================= cron

@mcp.tool
async def dpanel_cron_list() -> str:
    """计划任务列表。"""
    _check("dpanel_cron_list")
    return _json(await _call("/common/cron/get-list", {}))


@mcp.tool
async def dpanel_cron_detail(id: int) -> str:
    """计划任务详情。"""
    _check("dpanel_cron_detail")
    return _json(await _call("/common/cron/get-detail", {"id": id}))


@mcp.tool
async def dpanel_cron_log_list(id: int) -> str:
    """计划任务执行日志。"""
    _check("dpanel_cron_log_list")
    return _json(await _call("/common/cron/get-log-list", {"id": id}))


@mcp.tool
async def dpanel_cron_create(name: str, spec: str, type: str, option: dict) -> str:
    """创建计划任务。spec 为 cron 表达式，type 见面板文档。"""
    _check("dpanel_cron_create")
    return _json(
        await _call(
            "/common/cron/create", {"name": name, "spec": spec, "type": type, "option": option}
        )
    )


@mcp.tool
async def dpanel_cron_run_once(id: int) -> str:
    """立即执行一次计划任务。"""
    _check("dpanel_cron_run_once")
    return _json(await _call("/common/cron/run-once", {"id": id}))


@mcp.tool
async def dpanel_cron_delete(id: int, confirm: bool = False) -> str:
    """删除计划任务（破坏性，需 confirm=true）。"""
    _check("dpanel_cron_delete", confirm)
    return _json(await _call("/common/cron/delete", {"id": id}))


@mcp.tool
async def dpanel_cron_prune_log(id: int, confirm: bool = False) -> str:
    """清理计划任务日志（破坏性，需 confirm=true）。"""
    _check("dpanel_cron_prune_log", confirm)
    return _json(await _call("/common/cron/prune-log", {"id": id}))


# ====================================================================== store

@mcp.tool
async def dpanel_store_list() -> str:
    """应用商店应用列表。"""
    _check("dpanel_store_list")
    return _json(await _call("/common/store/get-list", {}))


@mcp.tool
async def dpanel_store_sync() -> str:
    """同步应用商店数据。"""
    _check("dpanel_store_sync")
    return _json(await _call("/common/store/sync", {}))


@mcp.tool
async def dpanel_store_deploy(name: str, option: dict) -> str:
    """从应用商店部署应用。"""
    _check("dpanel_store_deploy")
    return _json(await _call("/common/store/deploy", {"name": name, "option": option}))


@mcp.tool
async def dpanel_store_create(name: str, title: str, description: str = "") -> str:
    """添加自定义应用商店源。"""
    _check("dpanel_store_create")
    return _json(
        await _call(
            "/common/store/create",
            {"name": name, "title": title, "description": description},
        )
    )


@mcp.tool
async def dpanel_store_delete(name: str, confirm: bool = False) -> str:
    """删除应用商店源（破坏性，需 confirm=true）。"""
    _check("dpanel_store_delete", confirm)
    return _json(await _call("/common/store/delete", {"name": name}))


# ================================================================ event/notice

@mcp.tool
async def dpanel_event_list() -> str:
    """DPanel 事件列表。"""
    _check("dpanel_event_list")
    return _json(await _call("/common/event/get-list", {}))


@mcp.tool
async def dpanel_notice_list() -> str:
    """通知列表。"""
    _check("dpanel_notice_list")
    return _json(await _call("/common/notice/get-list", {}))


@mcp.tool
async def dpanel_notice_unread() -> str:
    """未读通知。"""
    _check("dpanel_notice_unread")
    return _json(await _call("/common/notice/unread", {}))


@mcp.tool
async def dpanel_event_prune(confirm: bool = False) -> str:
    """清理事件（破坏性，需 confirm=true）。"""
    _check("dpanel_event_prune", confirm)
    return _json(await _call("/common/event/prune", {}))


@mcp.tool
async def dpanel_notice_delete(id: int, confirm: bool = False) -> str:
    """删除通知（破坏性，需 confirm=true）。"""
    _check("dpanel_notice_delete", confirm)
    return _json(await _call("/common/notice/delete", {"id": id}))
