# 常用命令入口：make help 查看全部。兼容 macOS 自带 make 3.81 与 Linux GNU make。
# 生产项目名固定为 knownote-prod；测试时可 make xxx PROJECT=其他名 覆盖。
PROJECT ?= knownote-prod
COMPOSE := docker compose -p $(PROJECT) -f docker-compose.prod.yml

.PHONY: help dev check init deploy up update down ps logs health backup

help: ## 显示全部命令
	@grep -E '^[a-z]+:.*##' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  make %-8s %s\n", $$1, $$2}'

dev: ## 本地开发：一键启动数据库、后端、前端
	./scripts/dev.sh

check: ## 本地检查：后端构建/测试 + 前端构建
	./scripts/check.sh

init: ## 服务器首次部署：生成 .env 与 backend/.env（随机密钥+默认管理员），可加 URL=http://服务器IP
	./scripts/init-env.sh

deploy: ## 一键部署：检查配置 → 构建启动 → 等待服务健康 → 健康检查
	@test -f .env || { echo "缺少 .env：请先 cp .env.example .env，填写 POSTGRES_PASSWORD 与 CORS_ORIGINS（或执行 make init）"; exit 1; }
	@test -f backend/.env || { echo "缺少 backend/.env：请先配置（复制你的配置，或 cp backend/.env.example backend/.env 后填写）"; exit 1; }
	$(COMPOSE) up -d --build --wait
	@$(MAKE) --no-print-directory health PROJECT=$(PROJECT)

up: ## 构建并启动，不等待健康（一般用 make deploy）
	$(COMPOSE) up -d --build

update: ## 更新：git pull 后一键部署
	git pull
	@$(MAKE) --no-print-directory deploy PROJECT=$(PROJECT)

down: ## 停止服务（保留数据卷；切勿加 -v）
	$(COMPOSE) down

ps: ## 查看服务状态
	$(COMPOSE) ps

logs: ## 实时查看日志（Ctrl+C 退出）
	$(COMPOSE) logs --tail=100 -f

health: ## 健康检查
	@curl -fsS http://localhost:$${HTTP_PORT:-80}/api/health && echo

backup: ## 备份数据库与上传文件到 backup/<时间戳>/（短暂停止后端，不覆盖旧备份，失败也会重启后端）
	@set -e; d="backup/$$(date +%Y%m%d_%H%M%S)"; mkdir -p "$$d"; chmod 700 backup "$$d"; \
	trap '$(COMPOSE) start backend' EXIT; \
	$(COMPOSE) stop backend; \
	$(COMPOSE) exec -T postgres pg_dump -U knownote -d knownote -Fc > "$$d/database.dump"; \
	docker run --rm --network none -v $(PROJECT)_knownote-data:/source:ro -v "$$PWD/$$d:/backup" alpine:3.22 tar czf /backup/files.tar.gz -C /source .; \
	chmod 600 "$$d/database.dump" "$$d/files.tar.gz"; \
	echo "备份完成：$$d"
