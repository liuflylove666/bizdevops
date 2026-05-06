#!/usr/bin/env sh
# One-command local bootstrap: create default compose config, build the app image, and start all services.
set -eu

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

if docker compose version >/dev/null 2>&1; then
  DC="docker compose"
elif command -v docker-compose >/dev/null 2>&1; then
  DC="docker-compose"
else
  echo "ERROR: Docker Compose is required. Install Docker Desktop or docker compose plugin first." >&2
  exit 1
fi

if ! docker info >/dev/null 2>&1; then
  echo "ERROR: Docker is not running or current user cannot access Docker." >&2
  exit 1
fi

ENV_FILE="$ROOT/.env"
if [ ! -f "$ENV_FILE" ]; then
  cat >"$ENV_FILE" <<'EOF'
# JeriDevOps local one-command defaults.
# Edit this file before running deploy/start.sh again if local ports or secrets need to change.
COMPOSE_PROJECT_NAME=jeridevops

# Host ports
DEVOPS_HTTP_PORT=80
MYSQL_HOST_PORT=3306
REDIS_HOST_PORT=6379
NACOS_HTTP_PORT=8848
NACOS_GRPC_PORT=9848
NACOS_RAFT_PORT=9849
GITLAB_HTTP_PORT=8929
GITLAB_SSH_PORT=2224
REGISTRY_HOST_PORT=5001

# DevOps app
PORT=8080
LOG_LEVEL=info
JWT_SECRET=devops-local-change-me
SKIP_WEB_BUILD=1

# MySQL / Redis
MYSQL_PASSWORD=devops_local_root
MYSQL_DATABASE=devops
REDIS_PASSWORD=

# Kubernetes builder / registry defaults
K8S_REGISTRY=registry:5000
K8S_REPOSITORY=jeridevops

# GitLab / Nacos local defaults
GITLAB_ROOT_PASSWORD=F8v#Q4z!K7m@N2p%
GITLAB_RUNNER_REGISTRATION_TOKEN=jeridevops-local-runner-token
NACOS_AUTH_IDENTITY_KEY=serverIdentity
NACOS_AUTH_IDENTITY_VALUE=security
NACOS_AUTH_TOKEN=VGhpcy1Jcy1Bbi1JbnRlZ3JhdGVkLU5hY29zLVRva2Vu
EOF
  echo "Created .env with local defaults."
else
  echo "Using existing .env."
fi

if [ ! -f "$ROOT/web/dist/index.html" ]; then
  echo "web/dist was not found; Docker build will compile the frontend inside the image."
fi

$DC config --quiet
$DC up -d --build "$@"
$DC ps

env_value() {
  key="$1"
  default="$2"
  value="$(sed -n "s/^${key}=//p" "$ENV_FILE" | sed -n '1p')"
  if [ -n "$value" ]; then
    printf '%s' "$value"
  else
    printf '%s' "$default"
  fi
}

DEVOPS_HTTP_PORT_VALUE="$(env_value DEVOPS_HTTP_PORT 80)"
NACOS_HTTP_PORT_VALUE="$(env_value NACOS_HTTP_PORT 8848)"
GITLAB_HTTP_PORT_VALUE="$(env_value GITLAB_HTTP_PORT 8929)"
REGISTRY_HOST_PORT_VALUE="$(env_value REGISTRY_HOST_PORT 5001)"

if [ "$DEVOPS_HTTP_PORT_VALUE" = "80" ]; then
  DEVOPS_BASE_URL="http://localhost"
else
  DEVOPS_BASE_URL="http://localhost:$DEVOPS_HTTP_PORT_VALUE"
fi

cat <<EOF

JeriDevOps is starting.
Frontend:      $DEVOPS_BASE_URL
Swagger:       $DEVOPS_BASE_URL/swagger/index.html
Health:        $DEVOPS_BASE_URL/health
Login:         admin / admin123
GitLab:        http://localhost:$GITLAB_HTTP_PORT_VALUE  root / value of GITLAB_ROOT_PASSWORD in .env
Nacos:         http://localhost:$NACOS_HTTP_PORT_VALUE  nacos / nacos
Registry:      localhost:$REGISTRY_HOST_PORT_VALUE

Logs:          docker compose logs -f devops
Stop:          docker compose down
EOF
