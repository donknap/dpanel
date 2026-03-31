# DPanel Agent Entry

本仓库的公共规范以 [app/pro/docs/project-spec.md](/Users/renchao/Workspace/dpanel/dpanel/app/pro/docs/project-spec.md) 为准。

## 开始工作前

在分析、设计、修改代码、补测试、写文档之前，先阅读：

1. [app/pro/docs/project-spec.md](/Users/renchao/Workspace/dpanel/dpanel/app/pro/docs/project-spec.md)
2. [app/pro/docs/claude.md](/Users/renchao/Workspace/dpanel/dpanel/app/pro/docs/claude.md)
3. [app/pro/docs/codex.md](/Users/renchao/Workspace/dpanel/dpanel/app/pro/docs/codex.md)

按当前所使用的 AI 工具，优先读取对应的专用文件；若专用文件与公共规范冲突，以公共规范为准。

## 硬规则

- 修改代码前，先给出可选方案并等待确认。
- 默认使用中文沟通、说明和评审。
- 未经确认，不进行实质性代码写入、删除、迁移、重构。
- 不得覆盖、回滚或污染用户已有未提交改动。
- 新发现的稳定约定，需要提示是否沉淀到公共规范。

## 文档结构

- 公共规范主文件：[app/pro/docs/project-spec.md](/Users/renchao/Workspace/dpanel/dpanel/app/pro/docs/project-spec.md)
- Claude 专用说明：[app/pro/docs/claude.md](/Users/renchao/Workspace/dpanel/dpanel/app/pro/docs/claude.md)
- Codex 专用说明：[app/pro/docs/codex.md](/Users/renchao/Workspace/dpanel/dpanel/app/pro/docs/codex.md)

后续新增其它 AI 工具时，继续在 `app/pro/docs/` 下增加对应文件，并统一引用公共规范主文件。
