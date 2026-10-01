"""MCP tools bridging to the DPanel panel API (aligned to DPanel 1.11.0).

Every tool:
  1. checks the profile gate (profiles.py)
  2. destructive tools require an explicit confirm=True argument
     (fail-closed human-in-the-loop; frozen decision 8A)
  3. calls the DPanel API via client.post()
  4. redacts sensitive values before returning (frozen decision 9B)

Route/parameter contract below was verified against a live DPanel 1.11.0
container (empty-body probes expose required fields; happy paths exercised
end-to-end: compose create -> deploy -> ctrl stop -> destroy).
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
    """DPanel 系统信息：版本、运行环境、Docker 环境状态、数据挂载。"""
    _check("dpanel_system_info")
    return _json(await _call("/common/home/info", {}))


@mcp.tool
async def dpanel_system_usage() -> str:
    """DPanel 所在主机资源使用情况（CPU/内存/磁盘/面板占用）。"""
    _check("dpanel_system_usage")
    return _json(await _call("/common/panel/usage", {}))


@mcp.tool
async def dpanel_system_stat_list() -> str:
    """DPanel 多主机统计列表。"""
    _check("dpanel_system_stat_list")
    return _json(await _call("/common/home/get-stat-list", {}))


@mcp.tool
async def dpanel_setting_get(group_name: str, name: str) -> str:
    """读取 DPanel 配置项。group_name 如 setting；name 如 DPanelInfo/docker。"""
    _check("dpanel_setting_get")
    return _json(
        await _call("/common/setting/get-setting", {"groupName": group_name, "name": name})
    )


@mcp.tool
async def dpanel_log_list(page: int = 1, page_size: int = 50) -> str:
    """DPanel 系统日志列表。"""
    _check("dpanel_log_list")
    return _json(await _call("/common/log/get-list", {"page": page, "pageSize": page_size}))


# ================================================================ environments

@mcp.tool
async def dpanel_env_list() -> str:
    """列出 Docker 环境（多主机），含当前环境。"""
    _check("dpanel_env_list")
    return _json(await _call("/common/env/get-list", {}))


@mcp.tool
async def dpanel_env_detail(name: str) -> str:
    """Docker 环境详情。"""
    _check("dpanel_env_detail")
    return _json(await _call("/common/env/get-detail", {"name": name}))


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
async def dpanel_container_list(md5: str = "", site_title: str = "") -> str:
    """容器列表（含状态、镜像、名称、网络）。可按 md5（容器ID）或站点名过滤。注意：API 不分页，返回全量。"""
    _check("dpanel_container_list")
    payload: dict = {}
    if md5:
        payload["md5"] = md5
    if site_title:
        payload["siteTitle"] = site_title
    return _json(await _call("/app/container/get-list", payload))


@mcp.tool
async def dpanel_container_detail(md5: str) -> str:
    """容器详情。md5 为容器 ID（完整或前缀）。"""
    _check("dpanel_container_detail")
    return _json(await _call("/app/container/get-detail", {"md5": md5}))


@mcp.tool
async def dpanel_container_stat(id: str) -> str:
    """容器资源统计（CPU/内存/网络 IO）。id 为容器 ID。"""
    _check("dpanel_container_stat")
    return _json(await _call("/app/container/get-stat-info", {"id": id}))


@mcp.tool
async def dpanel_container_process(id: str) -> str:
    """容器内进程列表。id 为容器 ID。"""
    _check("dpanel_container_process")
    return _json(await _call("/app/container/get-process-info", {"id": id}))


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
async def dpanel_container_copy(md5: str, copy_name: str) -> str:
    """复制容器。copy_name 为新容器名。"""
    _check("dpanel_container_copy")
    return _json(await _call("/app/container/copy", {"md5": md5, "copyName": copy_name}))


@mcp.tool
async def dpanel_container_export(md5: str) -> str:
    """导出容器为镜像 tar。"""
    _check("dpanel_container_export")
    return _json(await _call("/app/container/export", {"md5": md5}))


@mcp.tool
async def dpanel_container_commit(md5: str, option: dict) -> str:
    """将容器提交为新镜像。"""
    _check("dpanel_container_commit")
    return _json(await _call("/app/container/commit", {"md5": md5, "option": option}))


@mcp.tool
async def dpanel_container_check_port(port: int = 0) -> str:
    """检查主机端口占用情况。"""
    _check("dpanel_container_check_port")
    payload: dict = {}
    if port:
        payload["port"] = port
    return _json(await _call("/app/container/check-port", payload))


@mcp.tool
async def dpanel_container_delete(
    md5: str, delete_image: bool = False, delete_volume: bool = False,
    delete_link: bool = False, confirm: bool = False,
) -> str:
    """删除容器（破坏性，需 confirm=true）。可选同时删除镜像/卷/链接。"""
    _check("dpanel_container_delete", confirm)
    return _json(
        await _call(
            "/app/container/delete",
            {
                "md5": md5,
                "deleteImage": delete_image,
                "deleteVolume": delete_volume,
                "deleteLink": delete_link,
            },
        )
    )


@mcp.tool
async def dpanel_container_prune(confirm: bool = False) -> str:
    """清理停止的容器（破坏性，需 confirm=true）。"""
    _check("dpanel_container_prune", confirm)
    return _json(await _call("/app/container/prune", {}))


# ============================================================ container upgrade

@mcp.tool
async def dpanel_container_upgrade_list() -> str:
    """容器升级检查列表（含可升级镜像 digest 对比）。"""
    _check("dpanel_container_upgrade_list")
    return _json(await _call("/app/container-upgrade/get-list", {}))


@mcp.tool
async def dpanel_container_upgrade_check(container_id: str) -> str:
    """检查单个容器镜像更新。container_id 为容器 ID。"""
    _check("dpanel_container_upgrade_check")
    return _json(await _call("/app/container-upgrade/check", {"containerId": container_id}))


@mcp.tool
async def dpanel_container_upgrade(
    md5: str, image_tag: str = "", enable_bak: bool = True,
) -> str:
    """升级容器（拉新镜像并重建，保留配置）。image_tag 留空用原镜像。"""
    _check("dpanel_container_upgrade")
    return _json(
        await _call(
            "/app/container-upgrade/upgrade",
            {"md5": md5, "imageTag": image_tag, "enableBak": enable_bak},
        )
    )


@mcp.tool
async def dpanel_container_upgrade_ignore(md5: str, image_id: str = "") -> str:
    """忽略容器升级（加入忽略清单）。"""
    _check("dpanel_container_upgrade_ignore")
    payload: dict = {"md5": md5}
    if image_id:
        payload["imageId"] = image_id
    return _json(await _call("/app/container-upgrade/ignore", payload))


# ==================================================================== backups

@mcp.tool
async def dpanel_container_backup_list() -> str:
    """容器备份列表。"""
    _check("dpanel_container_backup_list")
    return _json(await _call("/app/container-backup/get-list", {}))


@mcp.tool
async def dpanel_container_backup_detail(id: str) -> str:
    """备份详情。id 为备份记录 id。"""
    _check("dpanel_container_backup_detail")
    return _json(await _call("/app/container-backup/get-detail", {"id": id}))


@mcp.tool
async def dpanel_container_backup_create(id: str) -> str:
    """创建容器备份快照。id 为容器 ID。"""
    _check("dpanel_container_backup_create")
    return _json(await _call("/app/container-backup/create", {"id": id}))


@mcp.tool
async def dpanel_container_backup_delete(id: str, confirm: bool = False) -> str:
    """删除备份（破坏性，需 confirm=true）。"""
    _check("dpanel_container_backup_delete", confirm)
    return _json(await _call("/app/container-backup/delete", {"id": id}))


@mcp.tool
async def dpanel_container_backup_restore(id: str, confirm: bool = False) -> str:
    """从备份恢复容器（破坏性，需 confirm=true）。"""
    _check("dpanel_container_backup_restore", confirm)
    return _json(await _call("/app/container-backup/restore", {"id": id}))


# ===================================================================== compose

@mcp.tool
async def dpanel_compose_list() -> str:
    """Compose 项目列表（含运行容器概览）。注意：dangling 项目 id=0，仅可查看。"""
    _check("dpanel_compose_list")
    return _json(await _call("/app/compose/get-list", {}))


@mcp.tool
async def dpanel_compose_task(id: str) -> str:
    """Compose 项目详情与任务状态。id 为项目 id（数字字符串，列表返回）。"""
    _check("dpanel_compose_task")
    return _json(await _call("/app/compose/get-task", {"id": id}))


@mcp.tool
async def dpanel_compose_log(id: str, line_total: int = 50, download: bool = False) -> str:
    """Compose 项目日志。line_total ∈ 50|100|200|500|1000|5000|-1。download=true 返回全量文本。"""
    _check("dpanel_compose_log")
    return _json(
        await _call(
            "/app/compose/container-log",
            {"id": id, "lineTotal": line_total, "download": download},
        )
    )


@mcp.tool
async def dpanel_compose_get_from_uri(uri: str) -> str:
    """从 URL 拉取 compose 文件内容。"""
    _check("dpanel_compose_get_from_uri")
    return _json(await _call("/app/compose/get-from-uri", {"uri": uri}))


@mcp.tool
async def dpanel_compose_get_from_git(uri: str, name: str) -> str:
    """从 Git 仓库拉取 compose 文件。"""
    _check("dpanel_compose_get_from_git")
    return _json(await _call("/app/compose/get-from-git", {"uri": uri, "name": name}))


@mcp.tool
async def dpanel_compose_create(
    name: str,
    yaml: str,
    compose_type: str = "text",
    title: str = "",
    id: str = "",
    environment: list[dict] | None = None,
) -> str:
    """创建/更新 compose 项目。compose_type ∈ text|remoteUrl|outPath（storagePath/store 不可手动建）。
    yaml 为完整 docker-compose 内容（text 类型）。传 id 为更新。返回项目 id。"""
    _check("dpanel_compose_create")
    payload: dict = {"name": name, "type": compose_type, "yaml": yaml}
    if title:
        payload["title"] = title
    if id:
        payload["id"] = id
    if environment:
        payload["environment"] = environment
    return _json(await _call("/app/compose/create", payload))


@mcp.tool
async def dpanel_compose_deploy(
    id: str,
    environment: list[dict] | None = None,
    deploy_service_name: list[str] | None = None,
    remove_orphans: bool = False,
) -> str:
    """部署/更新 compose 项目。id 为项目 id。deploy_service_name 可指定只部署部分服务。"""
    _check("dpanel_compose_deploy")
    payload: dict = {"id": id}
    if environment is not None:
        payload["environment"] = environment
    if deploy_service_name is not None:
        payload["deployServiceName"] = deploy_service_name
    if remove_orphans:
        payload["removeOrphans"] = True
    return _json(await _call("/app/compose/container-deploy", payload))


@mcp.tool
async def dpanel_compose_container_ctrl(id: str, op: str) -> str:
    """Compose 项目控制：op ∈ start|stop|restart|pause|unpause|ls。"""
    _check("dpanel_compose_container_ctrl")
    return _json(await _call("/app/compose/container-ctrl", {"id": id, "op": op}))


@mcp.tool
async def dpanel_compose_destroy(
    id: str,
    delete_image: bool = False,
    delete_volume: bool = False,
    delete_data: bool = False,
    delete_path: bool = False,
    confirm: bool = False,
) -> str:
    """销毁 compose 项目（破坏性，需 confirm=true）。delete_data=true 同时删除项目记录；
    delete_path=true 删除 compose 文件目录。"""
    _check("dpanel_compose_destroy", confirm)
    return _json(
        await _call(
            "/app/compose/container-destroy",
            {
                "id": id,
                "deleteImage": delete_image,
                "deleteVolume": delete_volume,
                "deleteData": delete_data,
                "deletePath": delete_path,
            },
        )
    )


# ====================================================================== image

@mcp.tool
async def dpanel_image_list() -> str:
    """镜像列表。"""
    _check("dpanel_image_list")
    return _json(await _call("/app/image/get-list", {}))


@mcp.tool
async def dpanel_image_detail(md5: str) -> str:
    """镜像详情。md5 为镜像 sha256 digest（如 sha256:abc123...）。"""
    _check("dpanel_image_detail")
    return _json(await _call("/app/image/get-detail", {"md5": md5}))


@mcp.tool
async def dpanel_image_tag_search(keyword: str) -> str:
    """在 Docker Hub 搜索镜像。"""
    _check("dpanel_image_tag_search")
    return _json(await _call("/app/image/tag-search", {"keyword": keyword}))


@mcp.tool
async def dpanel_image_tag_add(md5: str, tag: str) -> str:
    """为镜像添加 tag。md5 为镜像 digest，tag 形如 repo:tag。"""
    _check("dpanel_image_tag_add")
    return _json(await _call("/app/image/tag-add", {"md5": md5, "tag": tag}))


@mcp.tool
async def dpanel_image_tag_sync(tag: str, sync_type: str = "pull") -> str:
    """同步镜像 tag。sync_type ∈ pull|push。tag 形如 repo:tag。"""
    _check("dpanel_image_tag_sync")
    return _json(await _call("/app/image/tag-sync", {"tag": tag, "type": sync_type}))


@mcp.tool
async def dpanel_image_tag_push_batch(md5: str, registry_server_address: str) -> str:
    """批量推送镜像 tag 到远程仓库。"""
    _check("dpanel_image_tag_push_batch")
    return _json(
        await _call(
            "/app/image/tag-push-batch",
            {"md5": md5, "registryServerAddress": registry_server_address},
        )
    )


@mcp.tool
async def dpanel_image_import_by_image_tar() -> str:
    """导入镜像 tar（从 DPanel 存储目录）。"""
    _check("dpanel_image_import")
    return _json(await _call("/app/image/import-by-image-tar", {}))


@mcp.tool
async def dpanel_image_tag_delete(tag: str, confirm: bool = False) -> str:
    """删除镜像 tag（破坏性，需 confirm=true）。"""
    _check("dpanel_image_tag_delete", confirm)
    return _json(await _call("/app/image/tag-delete", {"tag": tag}))


@mcp.tool
async def dpanel_image_delete(md5: list[str], confirm: bool = False) -> str:
    """删除镜像（破坏性，需 confirm=true）。md5 为镜像 sha256 digest 数组。"""
    _check("dpanel_image_delete", confirm)
    return _json(await _call("/app/image/delete", {"md5": md5}))


@mcp.tool
async def dpanel_image_prune(confirm: bool = False) -> str:
    """清理悬空镜像（破坏性，需 confirm=true）。"""
    _check("dpanel_image_prune", confirm)
    return _json(await _call("/app/image/prune", {}))


# ================================================================ image build

@mcp.tool
async def dpanel_image_build_list() -> str:
    """镜像构建任务列表。"""
    _check("dpanel_image_build_task")
    return _json(await _call("/app/image-build/get-list", {}))


@mcp.tool
async def dpanel_image_build_detail(id: str) -> str:
    """镜像构建任务详情。"""
    _check("dpanel_image_build_detail")
    return _json(await _call("/app/image-build/get-detail", {"id": id}))


@mcp.tool
async def dpanel_image_build_create(tags: str, option: dict | None = None) -> str:
    """创建镜像构建任务（从 Dockerfile）。tags 为目标镜像 tag。"""
    _check("dpanel_image_create_by_dockerfile")
    payload: dict = {"tags": tags}
    if option:
        payload.update(option)
    return _json(await _call("/app/image-build/create", payload))


@mcp.tool
async def dpanel_image_build_run(id: str) -> str:
    """执行镜像构建任务。"""
    _check("dpanel_image_build_run")
    return _json(await _call("/app/image-build/build", {"id": id}))


@mcp.tool
async def dpanel_image_build_delete(id: str, confirm: bool = False) -> str:
    """删除镜像构建任务（破坏性，需 confirm=true）。"""
    _check("dpanel_image_build_delete", confirm)
    return _json(await _call("/app/image-build/delete", {"id": id}))


@mcp.tool
async def dpanel_image_build_prune(confirm: bool = False) -> str:
    """清理构建缓存（破坏性，需 confirm=true）。"""
    _check("dpanel_image_build_prune", confirm)
    return _json(await _call("/app/image-build/prune", {}))


# ==================================================================== network

@mcp.tool
async def dpanel_network_list() -> str:
    """网络列表。"""
    _check("dpanel_network_list")
    return _json(await _call("/app/network/get-list", {}))


@mcp.tool
async def dpanel_network_detail(name: str) -> str:
    """网络详情。"""
    _check("dpanel_network_detail")
    return _json(await _call("/app/network/get-detail", {"name": name}))


@mcp.tool
async def dpanel_network_container_list(name: str) -> str:
    """网络上的容器列表。"""
    _check("dpanel_network_container_list")
    return _json(await _call("/app/network/get-container-list", {"name": name}))


@mcp.tool
async def dpanel_network_create(name: str, driver: str = "bridge") -> str:
    """创建网络。driver ∈ bridge|macvlan|ipvlan|overlay。"""
    _check("dpanel_network_create")
    return _json(await _call("/app/network/create", {"name": name, "driver": driver}))


@mcp.tool
async def dpanel_network_connect(name: str, container_name: str) -> str:
    """将容器接入网络。"""
    _check("dpanel_network_connect")
    return _json(
        await _call("/app/network/connect", {"name": name, "containerName": container_name})
    )


@mcp.tool
async def dpanel_network_disconnect(name: str, container_name: str) -> str:
    """将容器从网络断开。"""
    _check("dpanel_network_disconnect")
    return _json(
        await _call("/app/network/disconnect", {"name": name, "containerName": container_name})
    )


@mcp.tool
async def dpanel_network_delete(name: str, confirm: bool = False) -> str:
    """删除网络（破坏性，需 confirm=true）。"""
    _check("dpanel_network_delete", confirm)
    return _json(await _call("/app/network/delete", {"name": name}))


@mcp.tool
async def dpanel_network_prune(confirm: bool = False) -> str:
    """清理未使用网络（破坏性，需 confirm=true）。"""
    _check("dpanel_network_prune", confirm)
    return _json(await _call("/app/network/prune", {}))


# ===================================================================== volume

@mcp.tool
async def dpanel_volume_list() -> str:
    """卷列表（含使用状态）。"""
    _check("dpanel_volume_list")
    return _json(await _call("/app/volume/get-list", {}))


@mcp.tool
async def dpanel_volume_detail(name: str) -> str:
    """卷详情。"""
    _check("dpanel_volume_detail")
    return _json(await _call("/app/volume/get-detail", {"name": name}))


@mcp.tool
async def dpanel_volume_create(name: str, driver: str = "local") -> str:
    """创建卷。"""
    _check("dpanel_volume_create")
    return _json(await _call("/app/volume/create", {"name": name, "driver": driver}))


@mcp.tool
async def dpanel_volume_delete(name: list[str], confirm: bool = False) -> str:
    """删除卷（破坏性，需 confirm=true）。name 为卷名数组。"""
    _check("dpanel_volume_delete", confirm)
    return _json(await _call("/app/volume/delete", {"name": name}))


@mcp.tool
async def dpanel_volume_prune(confirm: bool = False) -> str:
    """清理未使用卷（破坏性，需 confirm=true）。"""
    _check("dpanel_volume_prune", confirm)
    return _json(await _call("/app/volume/prune", {}))


# =================================================================== explorer

def _mp(mount_point: str) -> str:
    """Validate explorer mountPoint format: volume:<name> | container:<md5> | docker:<env>."""
    if not any(mount_point.startswith(p) for p in ("volume:", "container:", "docker:")):
        raise DPanelApiError(
            "mountPoint must be 'volume:<name>', 'container:<md5>' or 'docker:<env>' "
            "(e.g. 'volume:dpanel-data', 'container:abc123', 'docker:local')"
        )
    return mount_point


@mcp.tool
async def dpanel_explorer_list(mount_point: str, path: str = "/") -> str:
    """文件列表。mount_point ∈ volume:<卷名> | container:<容器ID> | docker:<环境名>。"""
    _check("dpanel_explorer_list")
    return _json(
        await _call("/common/explorer/get-path-list", {"mountPoint": _mp(mount_point), "path": path})
    )


@mcp.tool
async def dpanel_explorer_content(mount_point: str, file: str) -> str:
    """读取文件内容。"""
    _check("dpanel_explorer_content")
    return _json(
        await _call("/common/explorer/get-content", {"mountPoint": _mp(mount_point), "file": file})
    )


@mcp.tool
async def dpanel_explorer_stat(mount_point: str, path: str) -> str:
    """文件 stat 信息。"""
    _check("dpanel_explorer_stat")
    return _json(
        await _call("/common/explorer/get-file-stat", {"mountPoint": _mp(mount_point), "path": path})
    )


@mcp.tool
async def dpanel_explorer_path_size(mount_point: str, path: str) -> str:
    """目录大小统计。"""
    _check("dpanel_explorer_path_size")
    return _json(
        await _call("/common/explorer/get-path-size", {"mountPoint": _mp(mount_point), "path": path})
    )


@mcp.tool
async def dpanel_explorer_export(mount_point: str, file_list: list[str]) -> str:
    """导出文件（打包）。"""
    _check("dpanel_explorer_export")
    return _json(
        await _call(
            "/common/explorer/export",
            {"mountPoint": _mp(mount_point), "fileList": file_list},
        )
    )


@mcp.tool
async def dpanel_explorer_import(mount_point: str, file_list: list[dict], dst_path: str) -> str:
    """导入文件。file_list: [{name, path}]。"""
    _check("dpanel_explorer_import")
    return _json(
        await _call(
            "/common/explorer/import",
            {"mountPoint": _mp(mount_point), "fileList": file_list, "dstPath": dst_path},
        )
    )


@mcp.tool
async def dpanel_explorer_unzip(mount_point: str, file: str, path: str) -> str:
    """解压压缩文件。"""
    _check("dpanel_explorer_unzip")
    return _json(
        await _call(
            "/common/explorer/unzip",
            {"mountPoint": _mp(mount_point), "file": file, "path": path},
        )
    )


@mcp.tool
async def dpanel_explorer_mkdir(mount_point: str, dst_path: str) -> str:
    """新建目录。"""
    _check("dpanel_explorer_mkdir")
    return _json(
        await _call("/common/explorer/mkdir", {"mountPoint": _mp(mount_point), "dstPath": dst_path})
    )


@mcp.tool
async def dpanel_explorer_permission(
    mount_point: str, file_list: list[str], mod: str = "", owner: str = "",
) -> str:
    """修改文件权限/所有者。mod 如 644；owner 为用户名。"""
    _check("dpanel_explorer_chmod")
    payload: dict = {"mountPoint": _mp(mount_point), "fileList": file_list}
    if mod:
        payload["mod"] = mod
    if owner:
        payload["owner"] = owner
    return _json(await _call("/common/explorer/permission", payload))


@mcp.tool
async def dpanel_explorer_delete(mount_point: str, file_list: list[str], confirm: bool = False) -> str:
    """删除文件（破坏性，需 confirm=true）。"""
    _check("dpanel_explorer_delete", confirm)
    return _json(
        await _call(
            "/common/explorer/delete",
            {"mountPoint": _mp(mount_point), "fileList": file_list},
        )
    )


# ======================================================================= cron

@mcp.tool
async def dpanel_cron_list() -> str:
    """计划任务列表。"""
    _check("dpanel_cron_list")
    return _json(await _call("/common/cron/get-list", {}))


@mcp.tool
async def dpanel_cron_detail(id: str) -> str:
    """计划任务详情。"""
    _check("dpanel_cron_detail")
    return _json(await _call("/common/cron/get-detail", {"id": id}))


@mcp.tool
async def dpanel_cron_log_list(id: str) -> str:
    """计划任务执行日志。"""
    _check("dpanel_cron_log_list")
    return _json(await _call("/common/cron/get-log-list", {"id": id}))


@mcp.tool
async def dpanel_cron_template() -> str:
    """计划任务模板列表（10 个官方模板）。"""
    _check("dpanel_cron_template")
    return _json(await _call("/common/cron/template", {}))


@mcp.tool
async def dpanel_cron_create(
    title: str, spec: str, trigger_type: str, option: dict,
) -> str:
    """创建计划任务。trigger_type ∈ cron|event|manual。spec 为 cron 表达式（trigger=cron 时）。
    option 结构参考 dpanel_cron_template 返回的模板。"""
    _check("dpanel_cron_create")
    return _json(
        await _call(
            "/common/cron/create",
            {"title": title, "spec": spec, "triggerType": trigger_type, "option": option},
        )
    )


@mcp.tool
async def dpanel_cron_run_once(id: str) -> str:
    """立即执行一次计划任务。"""
    _check("dpanel_cron_run_once")
    return _json(await _call("/common/cron/run-once", {"id": id}))


@mcp.tool
async def dpanel_cron_delete(id: str, confirm: bool = False) -> str:
    """删除计划任务（破坏性，需 confirm=true）。"""
    _check("dpanel_cron_delete", confirm)
    return _json(await _call("/common/cron/delete", {"id": id}))


@mcp.tool
async def dpanel_cron_prune_log(id: str, confirm: bool = False) -> str:
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
async def dpanel_store_sync(name: str, store_type: str, url: str) -> str:
    """同步应用商店数据。"""
    _check("dpanel_store_sync")
    return _json(
        await _call("/common/store/sync", {"name": name, "type": store_type, "url": url})
    )


@mcp.tool
async def dpanel_store_deploy(store_id: str, option: dict) -> str:
    """从应用商店部署应用。store_id 为商店应用 id。"""
    _check("dpanel_store_deploy")
    return _json(await _call("/common/store/deploy", {"storeId": store_id, "option": option}))


@mcp.tool
async def dpanel_store_create(
    name: str, title: str, store_type: str, url: str, apps: list,
) -> str:
    """添加应用商店源。"""
    _check("dpanel_store_create")
    return _json(
        await _call(
            "/common/store/create",
            {"name": name, "title": title, "type": store_type, "url": url, "apps": apps},
        )
    )


@mcp.tool
async def dpanel_store_delete(id: str, confirm: bool = False) -> str:
    """删除应用商店源（破坏性，需 confirm=true）。"""
    _check("dpanel_store_delete", confirm)
    return _json(await _call("/common/store/delete", {"id": id}))


# ================================================================== registry

@mcp.tool
async def dpanel_registry_list() -> str:
    """镜像仓库列表。"""
    _check("dpanel_registry_list")
    return _json(await _call("/common/registry/get-list", {}))


@mcp.tool
async def dpanel_registry_detail(id: str) -> str:
    """镜像仓库详情。"""
    _check("dpanel_registry_detail")
    return _json(await _call("/common/registry/get-detail", {"id": id}))


@mcp.tool
async def dpanel_registry_create(title: str, server_address: str, option: dict | None = None) -> str:
    """添加镜像仓库。"""
    _check("dpanel_registry_create")
    payload: dict = {"title": title, "serverAddress": server_address}
    if option:
        payload.update(option)
    return _json(await _call("/common/registry/create", payload))


@mcp.tool
async def dpanel_registry_delete(id: str, confirm: bool = False) -> str:
    """删除镜像仓库（破坏性，需 confirm=true）。"""
    _check("dpanel_registry_delete", confirm)
    return _json(await _call("/common/registry/delete", {"id": id}))


# ================================================================ event/notice

@mcp.tool
async def dpanel_notice_list(page: int = 1, page_size: int = 20) -> str:
    """通知列表（操作事件流水）。"""
    _check("dpanel_notice_list")
    return _json(await _call("/common/notice/get-list", {"page": page, "pageSize": page_size}))


@mcp.tool
async def dpanel_notice_unread() -> str:
    """未读通知（action=new）。"""
    _check("dpanel_notice_unread")
    return _json(await _call("/common/notice/unread", {"action": "new"}))


@mcp.tool
async def dpanel_notice_delete(id: str, confirm: bool = False) -> str:
    """删除通知（破坏性，需 confirm=true）。"""
    _check("dpanel_notice_delete", confirm)
    return _json(await _call("/common/notice/delete", {"id": id}))
