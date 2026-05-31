# anban

安伴后端（Go）+ 子女端前端。独立第三服务，通过 manager OpenAPI 驱动冻结的 xiaozhi。
架构见 AnBan-C 文档仓：docs/specs/2026-05-28-module-decomposition-design.md。

## 本地起
1. `cp .env.example .env` 填 MANAGER_BASE_URL / MANAGER_API_TOKEN / ANBAN_ACCESS_CODE
2. `cd server && go run ./cmd/anban`
3. 健康检查：`curl http://localhost:8090/health`
