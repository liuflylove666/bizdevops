#!/usr/bin/env sh
# 删除 MySQL 持久化目录并重新拉起服务，触发 docker-entrypoint-initdb.d 中的 init_tables.sql（仅空数据目录时执行）。
# 用法：在项目根目录执行  sh deploy/reinit-mysql-data.sh
set -e
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
if command -v docker-compose >/dev/null 2>&1; then
  DC="docker-compose"
else
  DC="docker compose"
fi

$DC stop devops mysql 2>/dev/null || true
$DC rm -f devops mysql 2>/dev/null || true
rm -rf deploy/MySqlData
mkdir -p deploy/MySqlData

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
echo "MySQL 已按 init_tables.sql 重新初始化（若数据目录此前非空，请确认已删除 deploy/MySqlData）。默认登录 admin / admin123"
