#!/bin/bash
#===============================================================================
# Uptime-Monitor Linux 打包构建脚本
#===============================================================================
# 功能:
#   - 编译 Go 程序为 Linux 可执行二进制
#   - 打包配置文件、部署说明、systemd 单元到发布目录
#   - 支持两种驱动模式:
#       sqlplus 驱动: 纯 Go 静态编译(推荐, 部署最简单)
#       godror  驱动: 需 CGO + Oracle Instant Client
#   - 支持交叉编译(在其他平台为本机构建 Linux 版本)
#
# 用法:
#   ./build.sh                # 默认: 静态编译 sqlplus 驱动, 生成 linux/amd64
#   DRIVER=godror ./build.sh  # 使用 godror 原生驱动(需 CGO)
#   GOARCH=arm64 ./build.sh   # 交叉编译 ARM64
#   ./build.sh --local        # 按当前运行平台编译
#===============================================================================
set -euo pipefail

#--------------------------- 可配置变量 ----------------------------------------
APP_NAME="uptime-monitor"
VERSION="${VERSION:-1.0.0}"
DRIVER="${DRIVER:-sqlplus}"          # sqlplus | godror
GOOS_TARGET="${GOOS_TARGET:-linux}"
GOARCH_TARGET="${GOARCH_TARGET:-amd64}"
OUTPUT_DIR="${OUTPUT_DIR:-dist}"
BUILD_MODE="${BUILD_MODE:-cross}"    # cross=交叉编译 | local=本机编译

# 编译选项
if [ "${BUILD_MODE}" = "local" ]; then
    GOOS_BUILD=""
    GOARCH_BUILD=""
else
    GOOS_BUILD="${GOOS_TARGET}"
    GOARCH_BUILD="${GOARCH_TARGET}"
fi

# godror 驱动需要 CGO 及 Oracle Instant Client
if [ "${DRIVER}" = "godror" ]; then
    CGO_ENABLED=1
    echo "[INFO] godror 驱动需要 CGO 与 Oracle Instant Client 开发库"
    echo "[INFO] 请确认已安装: oracle-instantclient-devel 及设置 ORACLE_HOME"
else
    CGO_ENABLED=0
fi

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "${ROOT_DIR}"

#--------------------------- 输出目录结构 ---------------------------------------
STAGE="${OUTPUT_DIR}/${APP_NAME}-${VERSION}-${GOOS_TARGET}-${GOARCH_TARGET}"
BIN_DIR="${STAGE}/bin"
CONF_DIR="${STAGE}/conf"
DOC_DIR="${STAGE}/docs"

echo "================================================================"
echo " Uptime-Monitor 构建"
echo "   版本:   ${VERSION}"
echo "   驱动:   ${DRIVER}"
echo "   平台:   ${GOOS_TARGET}/${GOARCH_TARGET}"
echo "   CGO:    ${CGO_ENABLED}"
echo "================================================================"

#--------------------------- 依赖准备 ------------------------------------------
echo "[1/4] 拉取 Go 依赖..."
go mod tidy

#--------------------------- 编译 ----------------------------------------------
echo "[2/4] 编译二进制..."
rm -rf "${STAGE}"
mkdir -p "${BIN_DIR}"
if [ -n "${GOOS_BUILD}" ]; then
    CGO_ENABLED=${CGO_ENABLED} GOOS=${GOOS_BUILD} GOARCH=${GOARCH_BUILD} \
        go build -trimpath \
        -ldflags "-s -w -X main.version=${VERSION}" \
        -o "${BIN_DIR}/${APP_NAME}" .
else
    CGO_ENABLED=${CGO_ENABLED} \
        go build -trimpath \
        -ldflags "-s -w -X main.version=${VERSION}" \
        -o "${BIN_DIR}/${APP_NAME}" .
fi
echo "   二进制: ${BIN_DIR}/${APP_NAME}"

#--------------------------- 打包配置与文档 ------------------------------------
echo "[3/4] 复制配置与文档..."
mkdir -p "${CONF_DIR}" "${DOC_DIR}"
cp config.yaml.example "${CONF_DIR}/config.yaml.example"
cp docs/deploy.md       "${DOC_DIR}/部署文档.md" 2>/dev/null || true
cp docs/requirement.md  "${DOC_DIR}/需求文档.md" 2>/dev/null || true

# 生成部署用 systemd 单元模板(默认按安装到 /opt 生成路径, 可通过 DEPLOY_DIR 覆盖)
DEPLOY_DIR="${DEPLOY_DIR:-/opt/${APP_NAME}-${VERSION}-${GOOS_TARGET}-${GOARCH_TARGET}}"
cat > "${STAGE}/uptime-monitor.service" <<EOF
[Unit]
Description=Uptime-Monitor (Oracle health check)
After=network.target

[Service]
Type=simple
ExecStart=${DEPLOY_DIR}/bin/${APP_NAME} -c ${DEPLOY_DIR}/conf/config.yaml
Restart=always
RestartSec=10
Environment=UM_DB_PASSWORD=
Environment=UM_PUSH_URL=

[Install]
WantedBy=multi-user.target
EOF

#--------------------------- 生成发布包 ----------------------------------------
echo "[4/4] 打包发布包..."
cd "${OUTPUT_DIR}"
tar -czf "${APP_NAME}-${VERSION}-${GOOS_TARGET}-${GOARCH_TARGET}.tar.gz" \
    "$(basename "${STAGE}")"
echo ""
echo "================================================================"
echo " 构建完成!"
echo "   发布包: ${OUTPUT_DIR}/${APP_NAME}-${VERSION}-${GOOS_TARGET}-${GOARCH_TARGET}.tar.gz"
echo "   目录:   ${STAGE}"
echo "================================================================"
echo ""
echo "部署到目标机器:"
echo "  tar -xzf ${APP_NAME}-${VERSION}-${GOOS_TARGET}-${GOARCH_TARGET}.tar.gz -C /opt/"
echo "  cd /opt/${APP_NAME}-${VERSION}-${GOOS_TARGET}-${GOARCH_TARGET}"
echo "  cp conf/config.yaml.example conf/config.yaml"
echo "  # 编辑 conf/config.yaml 配置数据库与 Push 地址"
echo "  # (godror 模式) export LD_LIBRARY_PATH=<instant client lib>"
echo "  ./bin/${APP_NAME} -c conf/config.yaml"
