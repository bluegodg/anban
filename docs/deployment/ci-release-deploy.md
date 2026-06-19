# 部署：方案 A（本地编+scp）与 方案 B（CI release + 服务器拉取）

两套部署并存：B 是规范流水线（默认），A 是手动保底。两者都**不在生产机上 build**（生产机内存紧，现场 build 会 OOM 拖垮正在跑的服务）。

## 方案 B：push tag → GitHub Actions 编 → 服务器拉 release 二进制（默认）

发布一版：
```bash
# 本地，确认代码 + 测试 OK 后
git push                      # 推代码到 main
git tag v0.1.1                # 打版本 tag（语义化：主.次.修订）
git push origin v0.1.1        # 推 tag → 触发 .github/workflows/release.yml
```
GitHub Actions 在干净 runner 上：checkout → 装 Go（版本读 server/go.mod）→ `go test ./...`（门禁）→ 交叉编译 `anban-linux` → 发布到该 tag 的 Release。

服务器拉取部署（无需 Go、不 build）：
```bash
# 在服务器 ubuntu@101.34.214.149 上
bash ~/anban/deploy-pull.sh            # 拉 latest release
bash ~/anban/deploy-pull.sh v0.1.1     # 拉指定版本
```
`deploy-pull.sh` curl 公网 release 资产 → `anban.new` → 复用 `start.sh`（换二进制+载 env+重启+health）。

> 服务器本地不变的：`anban.env`（密钥，gitignore，不入库）、`anban.db`、childweb 的 `config.js`（公网 baseURL）。这些都不在 release 里，pull 部署不会动它们。

## 方案 A：本地交叉编译 + scp（手动保底）

```bash
bash deploy.sh    # 本地 GOOS=linux 编 anban-linux → scp 到服务器 → start.sh 重启
```
适合：CI 暂时不可用、或想跳过打 tag 直接验证某个本地改动。

## 边界与说明

- **childweb（静态）暂不进 release**：仍用 scp 覆盖 `~/anban/childweb/`（python3 直读，覆盖即生效），注意别覆盖服务器本地的 `config.js`。待 childweb baseURL 改为按 `location` 派生后，可一并打进 release。
- 版本 tag 用语义化版本 `vMAJOR.MINOR.PATCH`。`v0.1.0` = Demo 首版。
- release 资产是**公开可下载**的（仓库公开）；不含任何密钥。
