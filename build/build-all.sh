#!/bin/bash
#===============================================================================
# Uptime-Monitor 多架构一键构建脚本 (V3)
# 一次性为多个 Linux 平台交叉编译(sqlplus 静态驱动), 便于批量分发。
#
# 用法:
#   ./build-all.sh                # 构建 amd64 + arm64
#   ./build-all.sh --arch=amd64   # 仅 amd64
#   ./build-all.sh --arch=amd64,arm64,arm
#===============================================================================
set -euo pipefail

DEFAULT_ARCH="amd64,arm64"
ARCH_LIST="${1#--arch=}"

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "${ROOT_DIR}"

IFS=',' read -ra ARCHES <<< "${ARCH_LIST:-$DEFAULT_ARCH}"

for arch in "${ARCHES[@]}"; do
    echo ""
    echo ">>> 构建 linux/${arch} ..."
    GOOS_TARGET=linux GOARCH_TARGET="${arch}" DRIVER=sqlplus \
        bash build/build.sh
done

echo ""
echo "所有架构构建完成。"
