# anban

安伴后端（Go）+ 子女端前端。它是方案 C 里的可选增强服务，通过 xiaozhi manager OpenAPI 驱动冻结的 `xiaozhi-esp32-server-golang`。

只部署 xiaozhi 时，设备应保持原版小智对话能力；再部署本仓库的 `anban` 服务，才增加子女端留言、问候、提醒、画像、状态和视觉等安伴功能。

当前阶段、目标验收和仓库边界见 [真机后阶段对齐与方案 C 部署说明](docs/plans/2026-06-14-真机后阶段对齐与方案C部署说明.md)。
此前设备到手前的阶段说明见 [当前阶段对齐与方案 C 执行说明](docs/plans/2026-06-12-phase-alignment-scheme-c.md)。
部署与设备联调见 [方案 C 部署与联调指南](docs/deployment/方案C部署与联调指南.md)。
设备到手当天的短清单见 [设备到手：方案 C 首日执行单](docs/deployment/设备到手方案C首日执行单.md)。
完整架构见 AnBan-C 文档仓与本仓 `docs/specs/2026-05-28-server-architecture-design.md`。

## 本地起

前提：`xiaozhi-esp32-server-golang` 已先部署，设备能完成原版对话，manager API Token 已签发。

1. `cp .env.example .env` 填 `ANBAN_MANAGER_BASE_URL` / `ANBAN_MANAGER_API_TOKEN` / `ANBAN_ACCESS_CODE`
2. `cd server && go run ./cmd/anban-preflight -device-id <xiaozhi设备ID> --xiaozhi-gate-passed` 做联调守门
3. `cd server && go run ./cmd/anban`
4. 健康检查：`curl http://localhost:8090/health`
