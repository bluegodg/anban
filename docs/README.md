# docs/（编码工作副本）

本目录是从 **AnBan-docs-repo** 拷来的、编码最常用的几份文档。
完整文档（xiaozhi 深读 7 分册、团队开发手册、Git 指南、硬件报告、历史归档等）在权威库：
https://github.com/bluegodg/AnBan-docs-repo

编码入口指南见仓库根的 **AGENTS.md**。

## 这里有什么
- 安伴V0.1产品文档PRD.md — 7 必演功能 + 每条验收判据(③ 段)
- specs/2026-05-28-module-decomposition-design.md — 模块结构 + xiaozhiclient 5 方法 + 依赖纪律（coder 圣经）
- specs/2026-05-28-server-architecture-design.md — C 方案 high-level
- specs/2026-05-29-xiaozhi-full-architecture-map.md — §9 每个域怎么调 manager OpenAPI
- specs/架构图.md — 分层 / 演进 / 进程边界图
- plans/2026-05-29-anban-backend-foundation.md — 地基计划 + 末尾 Roadmap（每个域第一步）
- plans/2026-06-14-真机后阶段对齐与方案C部署说明.md — 真机验证后的当前阶段、方案 C 两进程部署、可插拔边界和下一步 PRD 收口
- plans/2026-06-12-phase-alignment-scheme-c.md — 设备到手前的阶段对齐、目标验收、方案 C 部署顺序与仓库边界
- decisions/ — 服务端选 C 的决策记录
- deployment/README.md — 方案 C 部署入口：先回答“现在是什么阶段、这个仓库是什么、设备到了怎么部署”
- deployment/方案C仓库边界与部署总纲.md — 当前入口文档：仓库边界、两进程拓扑、设备到手后的 Gate A/B/C/D
- deployment/方案C部署与联调指南.md — 设备到手后按 C 方案部署：先纯 xiaozhi，再接安伴增强
- deployment/设备到手方案C首日执行单.md — 现场短清单：设备到了当天按 Gate A/B/C/D 逐项验证
