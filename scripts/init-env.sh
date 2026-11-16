#!/usr/bin/env bash
# 生成生产部署所需的两个环境文件，所有密钥现场随机生成，不写入 Git。
# 用法：scripts/init-env.sh                       # 之后手动改 CORS_ORIGINS
#       URL=http://1.2.3.4 scripts/init-env.sh     # 指定访问地址（域名则 https://域名，并自行把 SITE_ADDRESS 改为域名）
#       ADMIN_EMAIL=me@x.com ADMIN_PASSWORD='至少10位' scripts/init-env.sh
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

if [[ -e .env || -e backend/.env ]]; then
  echo "已存在 .env 或 backend/.env，为避免覆盖现有密钥，已停止。如确需重新生成，请先自行备份并移走它们。" >&2
  exit 1
fi
command -v openssl >/dev/null || { echo "需要 openssl 来生成随机密钥" >&2; exit 1; }

rand() { openssl rand -hex 32; }
URL="${URL:-http://localhost}"
ADMIN_EMAIL="${ADMIN_EMAIL:-admin@knownote.local}"
if [[ -z "${ADMIN_PASSWORD:-}" ]]; then
  ADMIN_PASSWORD="$(openssl rand -base64 24 | tr -d '/+=\n' | cut -c1-20)"
  GENERATED=1
else
  GENERATED=0
fi
if [[ ${#ADMIN_PASSWORD} -lt 10 ]]; then
  echo "ADMIN_PASSWORD 至少 10 个字符" >&2
  exit 1
fi

# 用 sed 写入前先拒绝会破坏替换的字符。
case "$ADMIN_PASSWORD$ADMIN_EMAIL$URL" in
  *[\|\&\\]*) echo "管理员邮箱、口令和 URL 不能包含 | & \\ 字符" >&2; exit 1 ;;
esac

umask 077
sed -e "s|^POSTGRES_PASSWORD=.*|POSTGRES_PASSWORD=$(rand)|" \
    -e "s|^CORS_ORIGINS=.*|CORS_ORIGINS=$URL|" .env.example > .env
sed -e "s|^JWT_SECRET=.*|JWT_SECRET=$(rand)|" \
    -e "s|^SETTINGS_ENCRYPTION_KEY=.*|SETTINGS_ENCRYPTION_KEY=$(rand)|" \
    -e "s|^ADMIN_EMAILS=.*|ADMIN_EMAILS=$ADMIN_EMAIL|" \
    -e "s|^ADMIN_PASSWORD=.*|ADMIN_PASSWORD=$ADMIN_PASSWORD|" backend/.env.example > backend/.env
chmod 600 .env backend/.env

echo "已生成 .env 与 backend/.env（权限 600，请勿提交，请另行安全备份）。"
echo "默认管理员账号：$ADMIN_EMAIL"
if [[ $GENERATED -eq 1 ]]; then
  echo "默认管理员口令：$ADMIN_PASSWORD"
  echo "该口令只在此显示一次，同时保存在 backend/.env 的 ADMIN_PASSWORD；首次启动创建账号后，建议登录改密并可删去该行。"
else
  echo "管理员口令：使用你提供的 ADMIN_PASSWORD（保存在 backend/.env）。"
fi
echo "下一步：make up   （仅 IP 访问时确认 .env 里 CORS_ORIGINS 为 http://服务器IP）"
