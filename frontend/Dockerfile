# 前端容器化：node 构建（vite build）→ nginx 托管静态产物 + 反代 API/agent
# 基础镜像走 DaoCloud 国内加速（无代理环境可拉取；npm 走 npmmirror）
# 本地测试：docker compose -f agent/docker-compose.local.yml up -d --build frontend
FROM docker.m.daocloud.io/library/node:20-alpine AS build
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm install --no-audit --no-fund --registry=https://registry.npmmirror.com
COPY . .
RUN npm run build

FROM docker.m.daocloud.io/library/nginx:alpine
ARG NGINX_CONFIG=nginx.conf
COPY --from=build /app/dist /usr/share/nginx/html
COPY ${NGINX_CONFIG} /etc/nginx/conf.d/default.conf
EXPOSE 80
