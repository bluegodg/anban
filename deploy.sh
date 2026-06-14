#!/usr/bin/env bash
# 一键部署 anban 到服务器：交叉编译(linux/amd64) -> scp -> 重启。
# 用法（在 git-bash 里）: bash deploy.sh
# 说明：token 等敏感配置存放在服务器 ~/anban/anban.env，不入库；本脚本不含任何密钥。
set -euo pipefail

SERVER="${ANBAN_SERVER:-ubuntu@101.34.214.149}"
REPO="$(cd "$(dirname "$0")" && pwd)"
OUT="$REPO/.gotmp-go/anban-linux"

echo "[1/3] 交叉编译 linux/amd64 ..."
( cd "$REPO/server" \
  && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
     GOPROXY=https://goproxy.cn,direct GOSUMDB=off \
     GOCACHE="$REPO/.gocache-go" GOTMPDIR="$REPO/.gotmp-go" \
     go build -trimpath -o "$OUT" ./cmd/anban )

echo "[2/3] 上传二进制 -> $SERVER ..."
scp -o StrictHostKeyChecking=accept-new "$OUT" "$SERVER:/home/ubuntu/anban/anban"

echo "[3/3] 重启 anban（服务器执行 ~/anban/start.sh）..."
ssh -o StrictHostKeyChecking=accept-new "$SERVER" 'bash ~/anban/start.sh'

echo "✅ 部署完成。"
