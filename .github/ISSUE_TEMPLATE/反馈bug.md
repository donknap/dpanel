
name: "🐛 反馈 Bug"
description: "反馈 DPanel 使用过程中遇到的问题"
title: "[BUG] "
labels: ["bug"]

body:
  - type: markdown
    attributes:
      value: |
        感谢你帮助改进 DPanel！

        请尽量完整地填写以下信息，便于快速定位问题。
        提交前请先确认是否已升级到最新版本，并检查是否存在相同的 Issue。

  - type: input
    id: version
    attributes:
      label: "DPanel 版本"
      description: "在【概览】→【系统信息】→【面板信息】中查看。"
      placeholder: "例如：1.11.1"
    validations:
      required: true

  - type: dropdown
    id: install-type
    attributes:
      label: "安装方式"
      description: "请选择当前使用的 DPanel 安装方式。"
      options:
        - "Docker 容器（docker run）"
        - "Docker Compose"
        - "二进制程序"
        - "DPanel Desktop"
        - "其他"
    validations:
      required: true

  - type: textarea
    id: install-command
    attributes:
      label: "安装命令或部署配置"
      description: |
        请提供完整的安装命令或部署配置：
        - Docker：提供 `docker run` 命令。
        - Docker Compose：提供 `compose.yaml` 配置。
        - 二进制程序：提供启动命令和参数。
        - DPanel Desktop / 其他：说明安装来源、安装命令或启动方式。

        **提交前请删除密码、Token、密钥等敏感信息。**
      placeholder: |
        在这里粘贴安装命令或部署配置……
    validations:
      required: true

  - type: textarea
    id: description
    attributes:
      label: "问题描述"
      description: "请描述实际出现的问题，以及你预期的正常结果。"
      placeholder: |
        实际情况：
        预期结果：
    validations:
      required: true

  - type: textarea
    id: reproduce
    attributes:
      label: "复现步骤"
      description: "请按操作顺序描述如何复现问题。"
      placeholder: |
        1. 打开……
        2. 点击……
        3. 出现……
    validations:
      required: true

  - type: textarea
    id: logs
    attributes:
      label: "错误日志"
      description: |
        可在【系统】→【系统日志】中查看，或执行 `docker logs dpanel` 获取日志。
        若容器名称不是 `dpanel`，请替换为实际容器名称。
        请先删除日志中的敏感信息。
      placeholder: "请粘贴相关错误日志（如有）……"
      render: shell
    validations:
      required: false

  - type: textarea
    id: attachments
    attributes:
      label: "截图及补充信息"
      description: "可以直接拖拽或粘贴截图，也可以补充系统环境、相关配置等信息。"
      placeholder: "截图或其他补充说明（如有）……"
    validations:
      required: false

  - type: markdown
    attributes:
      value: |
        ### QQ 群反馈
        如需交流，可加入 DPanel QQ 群：**837583876**。
        为便于跟踪问题和记录解决方案，建议同时保留 GitHub Issue。
