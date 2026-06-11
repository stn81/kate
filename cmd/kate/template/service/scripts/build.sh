#!/usr/bin/env bash
# 构建并打包：编译二进制 + 装配 outputs/{bin,conf,log,run}，配置取 scripts/conf/<env>.ini。
set -eu

#程序名称
APP=kateapp
PROJECT_HOME=$(cd "$(dirname "$0")" && cd .. && pwd -P)
PKG_HOME="$PROJECT_HOME/outputs"

usage() {
    cat <<EOF
Usage: $(basename "$0") [env]
       $(basename "$0") -h | --help

构建 $APP 并打包到 outputs/（bin/conf/log/run），配置取 scripts/conf/<env>.ini。

env:
  dev     开发环境；带调试符号构建（make debug=1，便于 dlv 断点）
  test    测试环境
  prod    生产环境（缺省值）
EOF
}

case "${1:-}" in
    -h|--help|help)
        usage
        exit 0
        ;;
    dev|test|prod)
        APP_ENV=$1
        ;;
    '')
        APP_ENV=prod
        ;;
    *)
        echo "error: unknown env '$1'" >&2
        echo >&2
        usage >&2
        exit 1
        ;;
esac
echo "environment = $APP_ENV"

# GOROOT 未设置时回退到 PATH 里的 go；统一透传给 make，避免 Makefile 解析出 /bin/go。
if [ -n "${GOROOT:-}" ]; then
    GO="$GOROOT/bin/go"
else
    GO=$(command -v go)
fi

cd "$PROJECT_HOME"
"$GO" version

rm -rf "$PKG_HOME"
mkdir -p "$PKG_HOME"/{bin,conf,log,run}
cp "scripts/conf/$APP_ENV.ini" "$PKG_HOME/conf/$APP.ini"

echo 'building started'
echo "-> building $APP ($APP_ENV)"
# dev 环境带调试符号（-N -l），便于 dlv 断点
if [ "$APP_ENV" = "dev" ]; then
    make GO="$GO" debug=1
else
    make GO="$GO"
fi
echo 'building finished'
