#!/usr/bin/env bash
# ★ 在【服务器】上运行（不是本地）。方案 B：从 GitHub Release 拉取 CI 编好的二进制并部署。
# 仓库是公开的 → 直接 curl 公网下载 release 资产，无需 gh CLI、无需 token。
# 用法：bash deploy-pull.sh [tag]     不带参数 = latest（最新 release）
#       bash deploy-pull.sh v0.1.1
# 对比 deploy.sh：那个在本地交叉编译再 scp；这个在服务器只下载成品二进制再重启。
set -euo pipefail

REPO="bluegodg/anban"
ASSET="anban-linux"
DIR="/home/ubuntu/anban"
TAG="${1:-latest}"

if [ "$TAG" = "latest" ]; then
  URL="https://github.com/$REPO/releases/latest/download/$ASSET"
else
  URL="https://github.com/$REPO/releases/download/$TAG/$ASSET"
fi

echo "[1/2] 下载 release 二进制（$TAG）：$URL"
# --http1.1：GitHub release 资产走 HTTP/2 偶发 "stream not closed cleanly (PROTOCOL_ERROR)"，强制 1.1 规避。
# -sS：静默但保留错误信息（避免进度条刷屏）。
curl -fL --http1.1 -sS --retry 3 -o "$DIR/anban.new" "$URL"
chmod +x "$DIR/anban.new"

echo "[2/2] 复用 start.sh：换二进制 + 载 anban.env + 重启 + 健康检查"
bash "$DIR/start.sh"

echo "✅ 已从 GitHub Release（$TAG）部署。"
