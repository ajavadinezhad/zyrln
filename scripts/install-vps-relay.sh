#!/usr/bin/env bash
# Install the Zyrln exit relay on a Linux VPS (binary + systemd + optional firewall).
#
# From your machine (needs ssh/scp; Go only if building locally):
#   ./scripts/install-vps-relay.sh user@YOUR_VPS_IP
#
# Put prebuilt binaries next to this script (no Go needed), or use the zip from:
#   make vps-relay-bundle  →  dist/zyrln-VERSION-vps.zip
#   zyrln-relay-linux-amd64, zyrln-relay-linux-arm64
#
# SSH user can be anything (ubuntu, debian, root, …). Non-root users need sudo
# on the server (you may be prompted for your SSH password and then sudo password).
# Password login is fine — ssh/scp will prompt interactively if you have no key.
#
# Optional env (set before the command):
#   ZYRLN_BUILD=1                   # compile on this machine instead of prebuilt
#   ZYRLN_GO=/path/to/go            # force Go binary if not in PATH
#   ZYRLN_RELAY_KEY=your-secret     # same value as EXIT_RELAY_KEY in Apps Script
#   ZYRLN_RELAY_KEY=auto            # generate a random key and print it at the end
#   ZYRLN_RELAY_PORT=8787          # listen port (default 8787)
#   ZYRLN_UFW=0                     # skip "ufw allow" (default: try if ufw exists)
#
# On the server only (binary already at /usr/local/bin/zyrln-relay):
#   sudo ./scripts/install-vps-relay.sh --local
#
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN_NAME=zyrln-relay
INSTALL_BIN=/usr/local/bin/${BIN_NAME}
ENV_FILE=/etc/zyrln-relay.env
UNIT_NAME=zyrln-relay.service
UNIT_PATH=/etc/systemd/system/${UNIT_NAME}
PORT="${ZYRLN_RELAY_PORT:-8787}"
LISTEN="0.0.0.0:${PORT}"
UFW="${ZYRLN_UFW:-1}"

log() { printf '==> %s\n' "$*"; }
die() { printf 'error: %s\n' "$*" >&2; exit 1; }

GO_CMD=""
GOTOOLCHAIN="${GOTOOLCHAIN:-go1.25.0}"

need_cmd() {
	command -v "$1" >/dev/null 2>&1 || die "missing command: $1 (install it and retry)"
}

# True if path is an executable Go that responds to "version".
go_works() {
	[[ -n "${1:-}" && -x "$1" ]] || return 1
	GOTOOLCHAIN="$GOTOOLCHAIN" "$1" version >/dev/null 2>&1
}

# Resolve Go: ZYRLN_GO, PATH, GOROOT, then usual install locations (Linux/macOS/Windows Git Bash).
find_go() {
	local c candidates=() seen="" p brew_prefix

	if [[ -n "${ZYRLN_GO:-}" ]]; then
		go_works "$ZYRLN_GO" || die "ZYRLN_GO is not a working Go: $ZYRLN_GO"
		GO_CMD=$ZYRLN_GO
		return
	fi
	if [[ -n "${GO:-}" && "$GO" == */* ]]; then
		go_works "$GO" || die "GO is not a working Go: $GO"
		GO_CMD=$GO
		return
	fi

	p="$(command -v go 2>/dev/null || true)"
	[[ -n "$p" ]] && candidates+=("$p")
	[[ -n "${GOROOT:-}" ]] && candidates+=("${GOROOT}/bin/go")
	candidates+=(
		/usr/local/go/bin/go
		/usr/bin/go
		/usr/lib/go/bin/go
		/usr/lib/golang/bin/go
		/snap/bin/go
		/opt/homebrew/bin/go
		/usr/local/bin/go
		"${HOME}/go/bin/go"
		"/c/Program Files/Go/bin/go.exe"
		"/c/Program Files (x86)/Go/bin/go.exe"
	)
	if command -v brew >/dev/null 2>&1; then
		brew_prefix="$(brew --prefix go 2>/dev/null || true)"
		[[ -n "$brew_prefix" ]] && candidates+=("${brew_prefix}/bin/go")
	fi

	for c in "${candidates[@]}"; do
		[[ -n "$c" ]] || continue
		case " $seen " in
			*" $c "*) continue ;;
		esac
		seen="$seen $c"
		if go_works "$c"; then
			GO_CMD=$c
			return
		fi
	done

	die "Go not found. Install from https://go.dev/dl/ (adds /usr/local/go/bin to PATH) or set ZYRLN_GO=/path/to/go"
}

setup_go() {
	find_go
	log "Go: $(GOTOOLCHAIN="$GOTOOLCHAIN" "$GO_CMD" version 2>/dev/null | head -1)"
}

remote_arch() {
	local host=$1
	local uname_m
	uname_m="$(ssh -o ConnectTimeout=15 "$host" 'uname -m')"
	case "$uname_m" in
		x86_64|amd64) echo amd64 ;;
		aarch64|arm64) echo arm64 ;;
		*) die "unsupported remote architecture: $uname_m (need amd64 or arm64)" ;;
	esac
}

prebuilt_path() {
	local goarch=$1
	printf '%s/zyrln-relay-linux-%s' "$SCRIPT_DIR" "$goarch"
}

build_binary() {
	local goarch=$1
	local out=$2
	[[ -n "$GO_CMD" ]] || setup_go
	log "building ${BIN_NAME} for linux/${goarch}"
	(
		cd "$ROOT"
		GOOS=linux GOARCH="$goarch" GOTOOLCHAIN="$GOTOOLCHAIN" "$GO_CMD" build -o "$out" ./relay/exit/
	)
}

# Use scripts/zyrln-relay-linux-{amd64,arm64} when present; else build if Go is available.
prepare_binary() {
	local goarch=$1
	local out=$2
	local prebuilt
	prebuilt="$(prebuilt_path "$goarch")"

	if [[ "${ZYRLN_BUILD:-0}" == "1" ]]; then
		build_binary "$goarch" "$out"
		return
	fi

	if [[ -f "$prebuilt" ]]; then
		log "using prebuilt $(basename "$prebuilt")"
		cp "$prebuilt" "$out"
		chmod 755 "$out"
		return
	fi

	if command -v go >/dev/null 2>&1 || [[ -n "${ZYRLN_GO:-}" ]]; then
		build_binary "$goarch" "$out"
		return
	fi

	die "missing prebuilt binary: $(basename "$prebuilt")
Place zyrln-relay-linux-amd64 and zyrln-relay-linux-arm64 next to this script (see: make vps-relay-bundle),
or install Go and retry, or set ZYRLN_BUILD=1 with Go available."
}

write_env_file() {
	local key="${1:-}"
	if [[ -f "$ENV_FILE" && -z "$key" ]]; then
		log "keeping existing ${ENV_FILE}"
		return
	fi
	log "writing ${ENV_FILE}"
	cat >"$ENV_FILE" <<EOF
ZYRLN_RELAY_LISTEN=${LISTEN}
ZYRLN_RELAY_KEY=${key}
EOF
	chmod 600 "$ENV_FILE"
}

install_unit() {
	log "installing systemd unit ${UNIT_PATH}"
	install -m 644 "$ROOT/relay/deploy/zyrln-relay.service" "$UNIT_PATH"
	systemctl daemon-reload
	systemctl enable "$UNIT_NAME"
	systemctl restart "$UNIT_NAME"
}

maybe_ufw() {
	[[ "$UFW" == "1" ]] || return 0
	if command -v ufw >/dev/null 2>&1; then
		log "opening port ${PORT}/tcp with ufw (if enabled)"
		ufw allow "${PORT}/tcp" || true
	else
		log "ufw not found — open port ${PORT}/tcp in your provider firewall if needed"
	fi
}

verify_local() {
	log "checking service"
	systemctl --no-pager --full status "$UNIT_NAME" || true
	curl -sf "http://127.0.0.1:${PORT}/healthz" >/dev/null || die "health check failed on 127.0.0.1:${PORT}"
	log "health check OK (http://127.0.0.1:${PORT}/healthz)"
}

setup_on_host() {
	local relay_key="${1:-}"
	[[ "$(id -u)" -eq 0 ]] || die "run --local as root (sudo ./scripts/install-vps-relay.sh --local)"

	if [[ ! -x "$INSTALL_BIN" ]]; then
		die "binary not found at ${INSTALL_BIN} — run the remote install from your laptop first"
	fi

	write_env_file "$relay_key"
	install_unit
	maybe_ufw
	verify_local

	log "done — use http://YOUR_SERVER_IP:${PORT}/relay in Apps Script EXIT_RELAY_URL"
	if [[ -n "$relay_key" ]]; then
		log "EXIT_RELAY_KEY in Code.gs must match ZYRLN_RELAY_KEY in ${ENV_FILE}"
	fi
}

install_remote() {
	local host=$1
	local relay_key="${ZYRLN_RELAY_KEY:-}"

	need_cmd ssh
	need_cmd scp

	if [[ "$relay_key" == "auto" ]]; then
		relay_key="$(openssl rand -hex 16 2>/dev/null || head -c 16 /dev/urandom | od -An -tx1 | tr -d ' \n')"
		log "generated relay key: ${relay_key}"
	fi

	local goarch tmpbin
	goarch="$(remote_arch "$host")"
	tmpbin="$(mktemp)"
	trap '[[ -n "${tmpbin:-}" ]] && rm -f "$tmpbin"' EXIT
	prepare_binary "$goarch" "$tmpbin"

	log "copying binary to ${host}"
	scp -q "$tmpbin" "${host}:/tmp/${BIN_NAME}"

	log "configuring ${host} (sudo used if SSH user is not root)"
	local relay_key_arg="-"
	[[ -n "$relay_key" ]] && relay_key_arg=$relay_key
	ssh -t "$host" "bash -s" -- "$relay_key_arg" "$PORT" "$UFW" <<'REMOTE'
set -euo pipefail
RELAY_KEY="${1:--}"
[[ "$RELAY_KEY" == "-" ]] && RELAY_KEY=""
RELAY_PORT="${2:-8787}"
UFW="${3:-1}"
BIN_NAME=zyrln-relay
INSTALL_BIN=/usr/local/bin/${BIN_NAME}
ENV_FILE=/etc/zyrln-relay.env
UNIT_NAME=zyrln-relay.service
LISTEN="0.0.0.0:${RELAY_PORT}"

if [[ "$(id -u)" -eq 0 ]]; then
  SUDO=()
else
  command -v sudo >/dev/null 2>&1 || { echo "error: need root or sudo on the VPS" >&2; exit 1; }
  SUDO=(sudo)
fi

"${SUDO[@]}" install -m 755 "/tmp/${BIN_NAME}" "${INSTALL_BIN}"
rm -f "/tmp/${BIN_NAME}"

if [[ -f "${ENV_FILE}" && -z "${RELAY_KEY}" ]]; then
  echo "==> keeping existing ${ENV_FILE}"
else
  echo "==> writing ${ENV_FILE}"
  "${SUDO[@]}" tee "${ENV_FILE}" >/dev/null <<EOF
ZYRLN_RELAY_LISTEN=${LISTEN}
ZYRLN_RELAY_KEY=${RELAY_KEY}
EOF
  "${SUDO[@]}" chmod 600 "${ENV_FILE}"
fi

"${SUDO[@]}" tee "/etc/systemd/system/${UNIT_NAME}" >/dev/null <<'UNIT'
[Unit]
Description=Zyrln Exit Relay
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=/etc/zyrln-relay.env
ExecStart=/usr/local/bin/zyrln-relay
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
UNIT

"${SUDO[@]}" systemctl daemon-reload
"${SUDO[@]}" systemctl enable "${UNIT_NAME}"
"${SUDO[@]}" systemctl restart "${UNIT_NAME}"

if [[ "${UFW}" == "1" ]] && command -v ufw >/dev/null 2>&1; then
  echo "==> ufw allow ${RELAY_PORT}/tcp"
  "${SUDO[@]}" ufw allow "${RELAY_PORT}/tcp" || true
fi

curl -sf "http://127.0.0.1:${RELAY_PORT}/healthz" >/dev/null
echo "==> health check OK"
"${SUDO[@]}" systemctl --no-pager status "${UNIT_NAME}" || true
REMOTE

	log "installed on ${host}"
	log "Apps Script: EXIT_RELAY_URL = http://YOUR_SERVER_IP:${PORT}/relay"
	if [[ -n "$relay_key" ]]; then
		log "Apps Script: EXIT_RELAY_KEY = ${relay_key}"
	fi
}

usage() {
	cat <<EOF
Usage:
  $(basename "$0") user@host          # e.g. ubuntu@1.2.3.4 — build, copy, enable service
  $(basename "$0") --local            # on VPS: sudo ./scripts/install-vps-relay.sh --local

SSH: any username; password or key auth. Non-root needs passwordless sudo or you
will be prompted (ssh -t). Installing to /usr/local/bin and systemd requires root.

Prebuilt (next to this script): zyrln-relay-linux-amd64, zyrln-relay-linux-arm64

Environment:
  ZYRLN_BUILD, ZYRLN_GO, ZYRLN_RELAY_KEY, ZYRLN_RELAY_PORT, ZYRLN_UFW
EOF
	exit 1
}

main() {
	case "${1:-}" in
		-h|--help) usage ;;
		--local)
			local key="${ZYRLN_RELAY_KEY:-}"
			if [[ "$key" == "auto" ]]; then
				key="$(openssl rand -hex 16 2>/dev/null || head -c 16 /dev/urandom | od -An -tx1 | tr -d ' \n')"
				log "generated relay key: ${key}"
			fi
			setup_on_host "$key"
			;;
		"") usage ;;
		*) install_remote "$1" ;;
	esac
}

main "$@"
