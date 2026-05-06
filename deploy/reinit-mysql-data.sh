#!/usr/bin/env sh
# 删除 MySQL named volume 并重新拉起服务，触发 docker-entrypoint-initdb.d 中的 init_tables.sql（仅空数据卷时执行）。
# 用法：在项目根目录执行  sh deploy/reinit-mysql-data.sh
set -e
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
if docker compose version >/dev/null 2>&1; then
  DC="docker compose"
elif command -v docker-compose >/dev/null 2>&1; then
  DC="docker-compose"
else
  echo "ERROR: Docker Compose is required." >&2
  exit 1
fi

$DC stop devops mysql 2>/dev/null || true
$DC rm -f devops mysql 2>/dev/null || true

PROJECT_NAME="$($DC config --format json | sed -n 's/^[[:space:]]*"name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | sed -n '1p')"
PROJECT_NAME="${PROJECT_NAME:-${COMPOSE_PROJECT_NAME:-jeridevops}}"
MYSQL_VOLUME="${PROJECT_NAME}_mysql_data"
docker volume rm "$MYSQL_VOLUME" 2>/dev/null || true

$DC up -d mysql redis
if $DC wait mysql 2>/dev/null; then
  :
else
  export MYSQL_PWD="${MYSQL_PASSWORD:-devops_local_root}"
  until $DC exec -T mysql mysqladmin ping -h 127.0.0.1 -uroot --silent 2>/dev/null; do
    sleep 2
  done
  unset MYSQL_PWD
fi

$DC up -d --build devops
echo "MySQL 已按 init_tables.sql 重新初始化（已重建 volume: $MYSQL_VOLUME）。默认登录 admin / admin123"
