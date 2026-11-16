# 常用命令入口：make help 查看全部。兼容 macOS 自带 make 3.81 与 Linux GNU make。
COMPOSE := docker compose -f docker-compose.prod.yml

.PHONY: help dev check init up update down ps logs health backup

help: ## 显示全部命令
	@grep -E '^[a-z]+:.*##' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  make %-8s %s\n", $$1, $$2}'

dev: ## 本地开发：一键启动数据库、后端、前端
	./scripts/dev.sh

check: ## 本地检查：后端构建/测试 + 前端构建
	./scripts/check.sh

init: ## 服务器首次部署：生成 .env 与 backend/.env（随机密钥+默认管理员），可加 URL=http://服务器IP
	./scripts/init-env.sh

up: ## 构建并启动（首次较慢，之后有缓存）
	$(COMPOSE) up -d --build

update: ## 拉取最新代码并重新构建启动
	git pull
	$(COMPOSE) up -d --build

down: ## 停止服务（保留数据卷；切勿加 -v）
	$(COMPOSE) down

ps: ## 查看服务状态
	$(COMPOSE) ps

logs: ## 实时查看日志（Ctrl+C 退出）
	$(COMPOSE) logs --tail=100 -f

health: ## 健康检查
	@curl -fsS http://localhost:$${HTTP_PORT:-80}/api/health && echo

backup: ## 备份数据库与上传文件到 backup/（备份期间短暂停止后端）
	mkdir -p backup && chmod 700 backup
	$(COMPOSE) stop backend
	$(COMPOSE) exec -T postgres pg_dump -U knownote -d knownote -Fc > backup/database.dump
	docker run --rm --network none -v knownote-prod_knownote-data:/source:ro -v "$$PWD/backup:/backup" alpine:3.22 tar czf /backup/files.tar.gz -C /source .
	chmod 600 backup/database.dump backup/files.tar.gz
	$(COMPOSE) start backend
