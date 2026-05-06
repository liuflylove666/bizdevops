# syntax=docker/dockerfile:1.7

#********************************************************************
# 项目名称：DevOps
# 文件名称：Dockerfile
# 创建时间：2026-04-14 22:33:28
#
# 系统用户：jerion
# 作　　者：Jerion
# 联系邮箱：416685476@qq.com
# 功能描述：单镜像生产构建，多阶段编译 Vue3 前端与 Go 后端服务，
#           运行时使用 Nginx 统一入口并通过 supervisord 管理多进程
#           端口 80：前端页面 + API 代理
#********************************************************************

############################
# 1) web builder (Vue 3)
############################
FROM node:22-alpine AS web-builder

WORKDIR /src/web

# 开发构建默认复用宿主机已生成的 web/dist，避免 Docker Desktop 内部 Vite 构建卡顿。
# 如需在镜像内强制重建前端：SKIP_WEB_BUILD=0 docker compose build devops
ARG SKIP_WEB_BUILD=1

# 优先复制依赖描述文件以利用构建缓存；仅在需要镜像内构建前端时安装依赖。
COPY web/package.json web/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm \
    if [ "$SKIP_WEB_BUILD" != "1" ]; then \
      npm ci --registry=https://registry.npmmirror.com; \
    fi

COPY web/ ./
RUN --mount=type=cache,target=/root/.npm \
    if [ "$SKIP_WEB_BUILD" = "1" ] && [ -f dist/index.html ]; then \
      echo "Using prebuilt web/dist from build context"; \
    else \
      if [ ! -d node_modules ]; then npm ci --registry=https://registry.npmmirror.com; fi; \
      npm run build; \
    fi

############################
# 2) server builder (Go)
############################
FROM golang:1.25-alpine AS server-builder

WORKDIR /src/server

# Go 依赖下载代理（构建时可用 --build-arg 覆盖）
ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=${GOPROXY}

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# 只复制 Go 编译所需路径，避免前端改动导致 server-builder 缓存整层失效，
# 并减小 COPY 层体积（配合仓库根 .dockerignore）。
COPY cmd ./cmd
COPY internal ./internal
COPY pkg ./pkg
COPY docs/docs.go ./docs/docs.go

# 默认尝试构建静态二进制；如项目依赖 CGO，可在构建时传入 CGO_ENABLED=1
ARG CGO_ENABLED=0
ENV CGO_ENABLED=${CGO_ENABLED}
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -o devops ./cmd/server/main.go

############################
# 3) runtime
############################
FROM node:22-alpine AS runtime

ENV TZ=Asia/Shanghai

# 安装 Nginx、supervisord、docker-cli 及运行时依赖；移除默认站点配置
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories \
 && apk add --no-cache nginx supervisor ca-certificates tzdata curl docker-cli git && \
    rm -f /etc/nginx/http.d/default.conf

WORKDIR /app

# Go 二进制
COPY --from=server-builder /src/server/devops /app/devops

# Vue 3 前端静态文件
COPY --from=web-builder /src/web/dist /usr/share/nginx/html

# Nginx 反向代理配置
COPY deploy/nginx.conf /etc/nginx/http.d/default.conf

# supervisord 多进程管理配置
COPY deploy/supervisord.conf /etc/supervisor/conf.d/supervisord.conf

# 容器启动脚本
COPY deploy/entrypoint.sh /entrypoint.sh
RUN sed -i 's/\r$//' /entrypoint.sh && chmod +x /entrypoint.sh

# 持久化挂载点：生产环境通过 -v /host/path:/app/data 挂载
RUN mkdir -p /app/data/logs && \
    ln -s /app/data/logs /app/logs

# 端口 80：前端页面 + API 代理
EXPOSE 80

HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
  CMD curl -fs http://127.0.0.1/health || exit 1

ENTRYPOINT ["/entrypoint.sh"]
