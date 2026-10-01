"""Capability profiles: which tool groups each profile may call.

Profiles (mirroring portainer-mcp):
  - read-only   : inspection only (lists, details, stats, logs)
  - read-write  : read-only + lifecycle operations (start/stop/restart, deploy, create)
  - admin       : read-write + destructive operations (delete, prune, restore)

Destructive tools are additionally gated behind an explicit confirm=true
argument (fail-closed human-in-the-loop) regardless of profile.
"""

from __future__ import annotations

from .config import Config

# tool-name prefix -> minimum profile that may use it
READ_ONLY = "read-only"
READ_WRITE = "read-write"
ADMIN = "admin"

_PROFILE_RANK = {READ_ONLY: 0, READ_WRITE: 1, ADMIN: 2}

# ---------------------------------------------------------------------------
# tool registry: name -> (minimum profile, destructive?)
#
# Destructive tools always require admin AND a confirmation step.
# Aligned to DPanel 1.11.0 routes (see tools.py docstring).
# ---------------------------------------------------------------------------

TOOLS: dict[str, tuple[str, bool]] = {
    # ---- system / home (read-only) ----
    "dpanel_system_info": (READ_ONLY, False),
    "dpanel_system_usage": (READ_ONLY, False),
    "dpanel_system_stat_list": (READ_ONLY, False),
    "dpanel_setting_get": (READ_ONLY, False),
    "dpanel_log_list": (READ_ONLY, False),
    # ---- environments (docker hosts) ----
    "dpanel_env_list": (READ_ONLY, False),
    "dpanel_env_detail": (READ_ONLY, False),
    "dpanel_env_switch": (READ_WRITE, False),
    "dpanel_env_create": (READ_WRITE, False),
    "dpanel_env_delete": (ADMIN, True),
    # ---- containers ----
    "dpanel_container_list": (READ_ONLY, False),
    "dpanel_container_detail": (READ_ONLY, False),
    "dpanel_container_stat": (READ_ONLY, False),
    "dpanel_container_process": (READ_ONLY, False),
    "dpanel_container_check_port": (READ_ONLY, False),
    "dpanel_container_status": (READ_WRITE, False),   # start/stop/restart/pause/unpause
    "dpanel_container_update": (READ_WRITE, False),
    "dpanel_container_copy": (READ_WRITE, False),
    "dpanel_container_export": (READ_ONLY, False),
    "dpanel_container_commit": (READ_WRITE, False),
    "dpanel_container_delete": (ADMIN, True),
    "dpanel_container_prune": (ADMIN, True),
    # ---- container upgrade (1.11.0: app/container-upgrade/*) ----
    "dpanel_container_upgrade_list": (READ_ONLY, False),
    "dpanel_container_upgrade_check": (READ_ONLY, False),
    "dpanel_container_upgrade": (READ_WRITE, False),
    "dpanel_container_upgrade_ignore": (READ_WRITE, False),
    # ---- container backups ----
    "dpanel_container_backup_list": (READ_ONLY, False),
    "dpanel_container_backup_detail": (READ_ONLY, False),
    "dpanel_container_backup_create": (READ_WRITE, False),
    "dpanel_container_backup_delete": (ADMIN, True),
    "dpanel_container_backup_restore": (ADMIN, True),
    # ---- compose projects ----
    "dpanel_compose_list": (READ_ONLY, False),
    "dpanel_compose_task": (READ_ONLY, False),
    "dpanel_compose_log": (READ_ONLY, False),
    "dpanel_compose_get_from_uri": (READ_ONLY, False),
    "dpanel_compose_get_from_git": (READ_ONLY, False),
    "dpanel_compose_create": (READ_WRITE, False),
    "dpanel_compose_deploy": (READ_WRITE, False),
    "dpanel_compose_container_ctrl": (READ_WRITE, False),
    "dpanel_compose_destroy": (ADMIN, True),
    # ---- images ----
    "dpanel_image_list": (READ_ONLY, False),
    "dpanel_image_detail": (READ_ONLY, False),
    "dpanel_image_tag_search": (READ_ONLY, False),
    "dpanel_image_tag_add": (READ_WRITE, False),
    "dpanel_image_tag_sync": (READ_WRITE, False),
    "dpanel_image_tag_push_batch": (READ_WRITE, False),
    "dpanel_image_import": (READ_WRITE, False),
    "dpanel_image_tag_delete": (ADMIN, True),
    "dpanel_image_delete": (ADMIN, True),
    "dpanel_image_prune": (ADMIN, True),
    # ---- image build (1.11.0: app/image-build/*) ----
    "dpanel_image_build_task": (READ_ONLY, False),
    "dpanel_image_build_detail": (READ_ONLY, False),
    "dpanel_image_create_by_dockerfile": (READ_WRITE, False),
    "dpanel_image_build_run": (READ_WRITE, False),
    "dpanel_image_build_delete": (ADMIN, True),
    "dpanel_image_build_prune": (ADMIN, True),
    # ---- networks ----
    "dpanel_network_list": (READ_ONLY, False),
    "dpanel_network_detail": (READ_ONLY, False),
    "dpanel_network_container_list": (READ_ONLY, False),
    "dpanel_network_create": (READ_WRITE, False),
    "dpanel_network_connect": (READ_WRITE, False),
    "dpanel_network_disconnect": (READ_WRITE, False),
    "dpanel_network_delete": (ADMIN, True),
    "dpanel_network_prune": (ADMIN, True),
    # ---- volumes ----
    "dpanel_volume_list": (READ_ONLY, False),
    "dpanel_volume_detail": (READ_ONLY, False),
    "dpanel_volume_create": (READ_WRITE, False),
    "dpanel_volume_delete": (ADMIN, True),
    "dpanel_volume_prune": (ADMIN, True),
    # ---- explorer (1.11.0: common/explorer/*, mountPoint based) ----
    "dpanel_explorer_list": (READ_ONLY, False),
    "dpanel_explorer_content": (READ_ONLY, False),
    "dpanel_explorer_stat": (READ_ONLY, False),
    "dpanel_explorer_path_size": (READ_ONLY, False),
    "dpanel_explorer_export": (READ_ONLY, False),
    "dpanel_explorer_import": (READ_WRITE, False),
    "dpanel_explorer_unzip": (READ_WRITE, False),
    "dpanel_explorer_mkdir": (READ_WRITE, False),
    "dpanel_explorer_chmod": (READ_WRITE, False),
    "dpanel_explorer_delete": (ADMIN, True),
    # ---- cron ----
    "dpanel_cron_list": (READ_ONLY, False),
    "dpanel_cron_detail": (READ_ONLY, False),
    "dpanel_cron_log_list": (READ_ONLY, False),
    "dpanel_cron_template": (READ_ONLY, False),
    "dpanel_cron_create": (READ_WRITE, False),
    "dpanel_cron_run_once": (READ_WRITE, False),
    "dpanel_cron_delete": (ADMIN, True),
    "dpanel_cron_prune_log": (ADMIN, True),
    # ---- app store ----
    "dpanel_store_list": (READ_ONLY, False),
    "dpanel_store_sync": (READ_WRITE, False),
    "dpanel_store_deploy": (READ_WRITE, False),
    "dpanel_store_create": (READ_WRITE, False),
    "dpanel_store_delete": (ADMIN, True),
    # ---- registry ----
    "dpanel_registry_list": (READ_ONLY, False),
    "dpanel_registry_detail": (READ_ONLY, False),
    "dpanel_registry_create": (READ_WRITE, False),
    "dpanel_registry_delete": (ADMIN, True),
    # ---- notices (1.11.0: events merged into notices) ----
    "dpanel_notice_list": (READ_ONLY, False),
    "dpanel_notice_unread": (READ_ONLY, False),
    "dpanel_notice_delete": (ADMIN, True),
}


class ProfileGate:
    """Decide whether the configured profile may call a given tool."""

    def __init__(self, config: Config):
        self.profile = config.profile

    def allowed(self, tool_name: str) -> bool:
        entry = TOOLS.get(tool_name)
        if entry is None:
            return False
        minimum, _ = entry
        return _PROFILE_RANK[self.profile] >= _PROFILE_RANK[minimum]

    def is_destructive(self, tool_name: str) -> bool:
        entry = TOOLS.get(tool_name)
        return bool(entry and entry[1])

    def visible_tools(self) -> list[str]:
        return [name for name in TOOLS if self.allowed(name)]

    def explain_denial(self, tool_name: str) -> str:
        entry = TOOLS.get(tool_name)
        if entry is None:
            return f"unknown tool: {tool_name}"
        minimum, destructive = entry
        hint = f"requires profile >= {minimum} (current: {self.profile})"
        if destructive and _PROFILE_RANK[self.profile] >= _PROFILE_RANK[minimum]:
            return f"{tool_name} is destructive and requires confirmation"
        return f"{tool_name} blocked: {hint}"
