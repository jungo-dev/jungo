#!/usr/bin/env bash
# Interactively create .env for a fresh clone of this repo.
# Run via `make app-init`.
set -euo pipefail

cd "$(dirname "$0")/.."

if [ -t 1 ] && command -v tput >/dev/null 2>&1; then
	CYAN=$(tput setaf 6) GREEN=$(tput setaf 2) YELLOW=$(tput setaf 3) RED=$(tput setaf 1) BOLD=$(tput bold) RESET=$(tput sgr0)
else
	CYAN="" GREEN="" YELLOW="" RED="" BOLD="" RESET=""
fi

if [ -f .env ]; then
	echo "${RED}Error: .env already exists. Remove or rename it first if you really want to re-run this.${RESET}" >&2
	exit 1
fi

echo "${CYAN}== Jungo app setup ==${RESET}"
echo "Press Enter to accept the default shown in [brackets]."
echo ""

prompt() {
	local question="$1" default="$2" answer
	read -r -p "$question [$default]: " answer
	echo "${answer:-$default}"
}

prompt_secret() {
	local question="$1" default="$2" answer
	read -rs -p "$question [$default]: " answer
	echo "" >&2
	echo "${answer:-$default}"
}

port_in_use() {
	local port="$1"
	if command -v lsof >/dev/null 2>&1; then
		lsof -iTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1 && return 0
		return 1
	elif command -v ss >/dev/null 2>&1; then
		ss -ltn 2>/dev/null | awk '{print $4}' | grep -Eq "[.:]${port}\$" && return 0
		return 1
	elif command -v nc >/dev/null 2>&1; then
		nc -z -w1 127.0.0.1 "$port" >/dev/null 2>&1 && return 0
		return 1
	else
		# Last resort: bash's /dev/tcp. Not compiled into Debian/Ubuntu's bash,
		# so this branch silently never matches there — the lsof/ss checks above
		# already cover that case.
		(exec 3<>"/dev/tcp/127.0.0.1/$port") 2>/dev/null && { exec 3<&-; exec 3>&-; return 0; }
		return 1
	fi
}

prompt_port() {
	local question="$1" default="$2" value confirm
	while true; do
		value=$(prompt "$question" "$default")
		if ! [[ "$value" =~ ^[0-9]+$ ]] || [ "$value" -lt 1 ] || [ "$value" -gt 65535 ]; then
			echo "${RED}  -> Invalid port, must be a number between 1-65535.${RESET}" >&2
			continue
		fi
		if port_in_use "$value"; then
			read -r -p "${YELLOW}  -> Port $value looks already in use. Use it anyway? [y/N]: ${RESET}" confirm
			case "$confirm" in
				[yY]*) ;;
				*) continue ;;
			esac
		fi
		echo "$value"
		return
	done
}

prompt_app_name() {
	local question="$1" default="$2" raw normalized
	while true; do
		raw=$(prompt "$question" "$default")
		normalized=$(printf '%s' "$raw" | tr '[:upper:]' '[:lower:]' | sed -E 's/[^a-z0-9_-]+/-/g; s/^[-_]+//; s/[-_]+$//')
		if [ -z "$normalized" ]; then
			echo "${RED}  -> Invalid name, try again.${RESET}" >&2
			continue
		fi
		if [ "$normalized" != "$raw" ]; then
			echo "${YELLOW}  -> Normalized to: $normalized${RESET}" >&2
		fi
		echo "$normalized"
		return
	done
}

gen_secret() {
	local secret
	if secret=$(openssl rand -hex 24 2>/dev/null) && [ -n "$secret" ]; then
		echo "$secret"
		return
	fi
	if [ -r /dev/urandom ] && command -v od >/dev/null 2>&1; then
		if secret=$(head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n') && [ -n "$secret" ]; then
			echo "$secret"
			return
		fi
	fi
	# Pure-bash fallback, no external tools required (e.g. minimal Git Bash on Windows).
	local i
	secret=""
	for ((i = 0; i < 48; i++)); do
		secret+=$(printf '%x' $((RANDOM % 16)))
	done
	echo "$secret"
}

APP_NAME=$(prompt_app_name "App name (used for docker container/network names)" "jungo")
DB_NAME=$(prompt "Database name" "jungo")
DB_USER=$(prompt "Database user" "postgres")
DB_PASSWORD=$(prompt_secret "Database password" "postgres")
API_SERVER_PORT=$(prompt_port "App port (host)" "8080")
DB_HOST_PORT=$(prompt_port "Database port (host)" "5432")
API_KEY=$(gen_secret)
TRACER_DEBUG_VALUE=$(printf '%04d' $((RANDOM % 10000)))

cp .env.example .env

set_env() {
	local key="$1" value="$2" escaped
	escaped=$(printf '%s' "$value" | sed -e 's/[\/&]/\\&/g')
	sed -i.bak "s/^${key}=.*/${key}=${escaped}/" .env
}

set_env "APP_NAME" "$APP_NAME"
set_env "DB_NAME" "$DB_NAME"
set_env "DB_USER" "$DB_USER"
set_env "DB_PASSWORD" "$DB_PASSWORD"
set_env "API_SERVER_PORT" "$API_SERVER_PORT"
set_env "DB_HOST_PORT" "$DB_HOST_PORT"
set_env "API_KEY" "$API_KEY"
set_env "TRACER_DEBUG_VALUE" "$TRACER_DEBUG_VALUE"
rm -f .env.bak

echo ""
echo "${CYAN}== .env created ==${RESET}"
echo "  APP_NAME=$APP_NAME"
echo "  DB_NAME=$DB_NAME"
echo "  DB_USER=$DB_USER"
echo "  API_SERVER_PORT=$API_SERVER_PORT"
echo "  DB_HOST_PORT=$DB_HOST_PORT"
echo "  API_KEY=$API_KEY"
echo "  TRACER_DEBUG_VALUE=$TRACER_DEBUG_VALUE"
check_tool() {
	local name="$1" install_cmd="$2"
	if command -v "$name" >/dev/null 2>&1; then
		echo "${GREEN}  [OK]      $name${RESET}"
	else
		echo "${YELLOW}  [MISSING] $name${RESET} — install with: $install_cmd"
	fi
}

echo "${CYAN}== CLI tools (needed for migrate-* / sqlc make targets) ==${RESET}"
check_tool "migrate" "go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest"
check_tool "sqlc" "go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest"

echo ""
echo "${BOLD}Next: make app-dev${RESET}"
