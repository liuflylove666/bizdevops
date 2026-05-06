#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
COMPOSE_FILE="${SCRIPT_DIR}/docker-compose.yaml"
KIND_CONFIG="${SCRIPT_DIR}/kind-config.yaml"

CLUSTER_NAME="${KIND_CLUSTER_NAME:-devops}"
KIND_NODE_IMAGE="${KIND_NODE_IMAGE:-kindest/node:v1.33.4}"
REGISTRY_CONTAINER_NAME="${REGISTRY_CONTAINER_NAME:-devops-registry}"
REGISTRY_HOST_PORT="${REGISTRY_HOST_PORT:-5001}"
REGISTRY_CONTAINER_PORT="${REGISTRY_CONTAINER_PORT:-5000}"

need_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "缺少命令: $1" >&2
    exit 1
  fi
}

need_cmd docker
need_cmd kind
need_cmd kubectl

echo "[1/5] 启动本地 registry ..."
docker compose -f "${COMPOSE_FILE}" up -d registry

echo "[2/5] 创建或复用 kind 集群 ${CLUSTER_NAME} ..."
if ! kind get clusters | grep -qx "${CLUSTER_NAME}"; then
  kind create cluster --name "${CLUSTER_NAME}" --config "${KIND_CONFIG}" --image "${KIND_NODE_IMAGE}"
else
  echo "kind 集群 ${CLUSTER_NAME} 已存在，跳过创建"
fi

echo "[3/5] 将 registry 接入 kind 网络 ..."
if ! docker network inspect kind >/dev/null 2>&1; then
  echo "kind 网络不存在，集群可能未创建成功" >&2
  exit 1
fi
if ! docker network inspect kind --format '{{json .Containers}}' | grep -q "\"Name\":\"${REGISTRY_CONTAINER_NAME}\""; then
  docker network connect kind "${REGISTRY_CONTAINER_NAME}"
else
  echo "registry 已连接到 kind 网络"
fi

echo "[4/5] 写入 local-registry-hosting ConfigMap ..."
cat <<EOF | kubectl --context "kind-${CLUSTER_NAME}" apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: local-registry-hosting
  namespace: kube-public
data:
  localRegistryHosting.v1: |
    host: "localhost:${REGISTRY_HOST_PORT}"
    help: "https://kind.sigs.k8s.io/docs/user/local-registry/"
EOF

echo "[5/5] 确保常用命名空间存在 ..."
kubectl wait --context "kind-${CLUSTER_NAME}" --for=condition=Ready nodes --all --timeout=180s
kubectl --context "kind-${CLUSTER_NAME}" create namespace devops-build --dry-run=client -o yaml | kubectl --context "kind-${CLUSTER_NAME}" apply -f -

echo
echo "本地 Kubernetes 环境已就绪:"
echo "- kind 集群: ${CLUSTER_NAME}"
echo "- 本地镜像仓库: localhost:${REGISTRY_HOST_PORT}"
echo "- 仓库容器地址: ${REGISTRY_CONTAINER_NAME}:${REGISTRY_CONTAINER_PORT}"
echo
echo "可用验证命令:"
echo "  kubectl cluster-info --context kind-${CLUSTER_NAME}"
echo "  docker pull busybox:1.36"
echo "  docker tag busybox:1.36 localhost:${REGISTRY_HOST_PORT}/demo/busybox:1.36"
echo "  docker push localhost:${REGISTRY_HOST_PORT}/demo/busybox:1.36"
echo "  kubectl run demo --image=localhost:${REGISTRY_HOST_PORT}/demo/busybox:1.36 --restart=Never --command -- sleep 3600"
