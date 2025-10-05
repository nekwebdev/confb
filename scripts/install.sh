#!/usr/bin/env bash
set -euo pipefail

REPO_OWNER="nekwebdev"
REPO_NAME="confb"

# defaults
PREFIX="${HOME}/.local"           # bin -> ~/.local/bin
CONFIG_DIR="${HOME}/.config/confb"
SYSTEMD_USER_DIR="${HOME}/.config/systemd/user"
INSTALL_SYSTEMD=1
VERSION=""                        # empty = latest
DRY_RUN=0
CURL="${CURL:-curl}"

usage() {
  cat <<EOF
confb installer

Usage:
  install.sh [--version vX.Y.Z] [--prefix DIR] [--no-systemd] [--dry-run]

Options:
  --version vX.Y.Z   Install a specific tag (default: latest release)
  --prefix DIR       Installation prefix (default: \$HOME/.local)
  --no-systemd       Skip creating/enabling systemd user service
  --dry-run          Print actions without executing
EOF
}

log() { printf '%s\n' "$*" >&2; }
die() { printf 'ERROR: %s\n' "$*" >&2; exit 1; }
run() { if [ "$DRY_RUN" -eq 1 ]; then echo "+ $*"; else eval "$@"; fi; }

while [ $# -gt 0 ]; do
  case "$1" in
    --version) VERSION="$2"; shift 2;;
    --prefix)  PREFIX="$2"; shift 2;;
    --no-systemd) INSTALL_SYSTEMD=0; shift;;
    --dry-run) DRY_RUN=1; shift;;
    -h|--help) usage; exit 0;;
    *) die "unknown arg: $1";;
  esac
done

BIN_DIR="${PREFIX}/bin"
CONF_PATH="${CONFIG_DIR}/confb.sample.yaml"
UNIT_PATH="${SYSTEMD_USER_DIR}/confb.service"

# --- detect platform ---
os=$(uname -s)
arch=$(uname -m)

case "$os" in
  Linux)  GOOS="linux" ;;
  Darwin) GOOS="darwin" ;;
  *) die "unsupported OS: $os" ;;
esac

case "$arch" in
  x86_64|amd64) GOARCH="amd64" ;;
  aarch64|arm64) GOARCH="arm64" ;;
  *) die "unsupported arch: $arch" ;;
esac

# --- resolve version ---
if [ -z "$VERSION" ]; then
  if [ "$DRY_RUN" -eq 1 ]; then
    # No network in dry-run; simulate a tag for display purposes.
    VERSION="<latest>"
    log "[dry-run] assuming VERSION=${VERSION} (no network)"
  else
    api="https://api.github.com/repos/${REPO_OWNER}/${REPO_NAME}/releases/latest"
    VERSION="$($CURL -fsSL "$api" | sed -n 's/.*"tag_name":[[:space:]]*"\([^"]\+\)".*/\1/p' | head -n1)"
    [ -n "$VERSION" ] || die "failed to detect latest release tag"
  fi
fi

TARBALL="${REPO_NAME}_${VERSION}_${GOOS}_${GOARCH}.tar.gz"
BASE_URL="https://github.com/${REPO_OWNER}/${REPO_NAME}/releases/download/${VERSION}"
URL_TGZ="${BASE_URL}/${TARBALL}"
URL_SHA="${BASE_URL}/checksums.txt"

log "Installing ${REPO_NAME} ${VERSION} for ${GOOS}/${GOARCH}"
log "PREFIX=${PREFIX}"

TMP="$(mktemp -d)"
cleanup(){ rm -rf "$TMP"; }
trap cleanup EXIT

cd "$TMP"

# --- download artifacts ---
run "$CURL -fsSLo checksums.txt '$URL_SHA'"
run "$CURL -fsSLo '$TARBALL' '$URL_TGZ'"

# >>> add this early return for dry-run <<<
if [ "$DRY_RUN" -eq 1 ]; then
  log "[dry-run] skipping checksum verification, extraction, and installation."
  log "[dry-run] would install to: ${BIN_DIR}"
  log "[dry-run] would place sample at: ${CONFIG_DIR}/confb.sample.yaml"
  log "[dry-run] would install service at: ${UNIT_PATH}"
  exit 0
fi

# --- verify checksum ---
if [ "$DRY_RUN" -eq 0 ]; then
  if command -v sha256sum >/dev/null 2>&1; then
    grep "  ${TARBALL}\$" checksums.txt | sha256sum -c -
  elif command -v shasum >/dev/null 2>&1; then
    grep "  ${TARBALL}\$" checksums.txt | shasum -a 256 -c -
  else
    die "no sha256sum/shasum found for verification"
  fi
fi

# --- extract and install binary ---
run "mkdir -p '$BIN_DIR'"
run "tar -xzf '$TARBALL'"
# Expect tarball to contain: confb, LICENSE, README.md, systemd/confb.service, confb.sample.yaml
if [ ! -f confb ]; then die "tarball missing 'confb' binary"; fi
run "install -m 0755 confb '${BIN_DIR}/confb'"

# --- config file (create if missing) ---
run "mkdir -p '$CONFIG_DIR'"

if [ ! -f "$CONF_PATH" ]; then
  # Always install sample as confb.sample.yaml (never overwrite user's config)
  run "install -m 0644 confb.sample.yaml '$CONF_PATH'" || {
    log "warning: confb.sample.yaml missing from package; creating minimal example"
    cat > "$TMP/confb.sample.yaml" <<'YAML'
version: 1
targets:
  - name: example
    format: raw
    output: ~/.config/confb/example.out
    sources:
      - path: /etc/hosts
YAML
    run "install -m 0644 '$TMP/confb.sample.yaml' '$CONF_PATH'"
  }
fi

# --- generate & install ALL man pages (root + subcommands) ---
MAN_DIR="$HOME/.local/share/man/man1"
run "mkdir -p '$MAN_DIR'"
log "generating man pages..."
# generate to temp, then gzip all *.1 to user's man1
run "'${BIN_DIR}/confb' man -o '$TMP/man1'"
for f in "$TMP"/man1/*.1; do
  [ -f "$f" ] || continue
  run "gzip -c '$f' > '$MAN_DIR/$(basename "$f").gz'"
done
log "installed man pages to $MAN_DIR (try: man confb, man confb-build)"

# --- install shell completions (best-effort) ---
# Bash
BASH_DIR="${BASH_COMPLETION_USER_DIR:-$HOME/.local/share/bash-completion/completions}"
mkdir -p "$BASH_DIR"
"${BIN_DIR}/confb" completion bash > "${BASH_DIR}/confb" || true

# Zsh
ZSH_DIR="$HOME/.local/share/zsh/site-functions"
mkdir -p "$ZSH_DIR"
"${BIN_DIR}/confb" completion zsh > "${ZSH_DIR}/_confb" || true
# Note for users: ensure fpath contains $HOME/.local/share/zsh/site-functions and compinit has run.

# Fish
FISH_DIR="$HOME/.config/fish/completions"
mkdir -p "$FISH_DIR"
"${BIN_DIR}/confb" completion fish > "${FISH_DIR}/confb.fish" || true

# --- systemd user service ---
if [ "$INSTALL_SYSTEMD" -eq 1 ]; then
  if command -v systemctl >/dev/null 2>&1; then
    run "mkdir -p '$SYSTEMD_USER_DIR'"
    if [ -f systemd/confb.service ]; then
      run "install -m 0644 systemd/confb.service '$UNIT_PATH'"
    else
      # fallback unit
      cat > "$TMP/confb.service" <<EOF
[Unit]
Description=confb daemon

[Service]
ExecStart=${BIN_DIR}/confb run -c ${CONF_PATH}
Restart=on-failure

[Install]
WantedBy=default.target
EOF
      run "install -m 0644 '$TMP/confb.service' '$UNIT_PATH'"
    fi
    run "systemctl --user daemon-reload"
    log "Systemd service installed but not enabled."
  else
    log "systemd not found; skipping service install. You can run:"
    log "  ${BIN_DIR}/confb run -c ${CONF_PATH}"
  fi
else
  log "Skipping systemd setup (--no-systemd)"
fi

# --- final instructions ---
cat <<EOF

✅ confb installed successfully!

Binary:   ${BIN_DIR}/confb
Sample:   ${CONF_PATH}
Service:  ${UNIT_PATH}

Next steps:
  1. Edit the sample file to suit your environment:
       cp ${CONFIG_DIR}/confb.sample.yaml ${CONFIG_DIR}/confb.yaml
       $EDITOR ${CONFIG_DIR}/confb.yaml

  2. Start confb manually or enable it to autostart at login:
       ${BIN_DIR}/confb run -c ${CONFIG_DIR}/confb.yaml --verbose
       # or, if using systemd user services:
       systemctl --user enable --now confb.service

  3. (Optional) To autostart it in a desktop session without systemd:
       Add this line to your startup script:
         ${BIN_DIR}/confb run -c ${CONFIG_DIR}/confb.yaml &

Completions:
  Bash:  ${BASH_DIR}/confb  (restart your shell)
  Zsh:   ${ZSH_DIR}/_confb  (ensure 'fpath+=${ZSH_DIR}'; then 'autoload -U compinit && compinit')
  Fish:  ${FISH_DIR}/confb.fish

EOF
