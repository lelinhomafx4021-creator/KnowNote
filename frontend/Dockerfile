FROM node:24-alpine AS build
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
ARG VITE_USE_MOCK=false
ARG VITE_API_BASE_URL=/api
RUN npm run build

FROM caddy:2-alpine
COPY --from=build /app/dist /srv
# 与仓库根目录 deploy/Caddyfile 保持一致，支持独立前端构建上下文。
COPY deploy/Caddyfile /etc/caddy/Caddyfile
