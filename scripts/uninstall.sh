#!/usr/bin/env bash
set -euo pipefail

PREFIX="${PREFIX:-$HOME/.local}"
BIN_DIR="$PREFIX/bin"
CONFIG_DIR="$HOME/.config/confb"
MAN_DIR="$HOME/.local/share/man/man1"
BASH_DIR="${BASH_COMPLETION_USER_DIR:-$HOME/.local/share/bash-completion/completions}"
ZSH_DIR="$HOME/.local/share/zsh/site-functions"
FISH_DIR="$HOME/.config/fish/completions"
SYSTEMD_USER_DIR="$HOME/.config/systemd/user"
UNIT_PATH="$SYSTEMD_USER_DIR/confb.service"

PURGE=0
DRY_RUN=0

usage() {
  cat <<EOF
confb uninstall

Usage:
  uninstall.sh [--purge] [--dry-run]

Options:
  --purge     Also remove ~/.config/confb (your configs!)
  --dry-run   Print what would be removed, do not delete
EOF
}

log(){ printf '%s\n' "$*" >&2; }
run(){ if [ "$DRY_RUN" -eq 1 ]; then echo "+ $*"; else eval "$@"; fi; }

while [ $# -gt 0 ]; do
  case "$1" in
    --purge) PURGE=1; shift;;
    --dry-run) DRY_RUN=1; shift;;
    -h|--help) usage; exit 0;;
    *) log "unknown arg: $1"; usage; exit 1;;
  esac
done

log "Uninstalling confb from $BIN_DIR"

# stop/disable systemd unit if present
if command -v systemctl >/dev/null 2>&1 && [ -f "$UNIT_PATH" ]; then
  run "systemctl --user disable --now confb.service || true"
  run "systemctl --user daemon-reload || true"
fi

# remove binary
[ -f "$BIN_DIR/confb" ] && run "rm -f '$BIN_DIR/confb'"

# remove man pages
for f in "$MAN_DIR"/confb*.1.gz; do
  [ -f "$f" ] && run "rm -f '$f'"
done

# remove completions
[ -f "$BASH_DIR/confb" ] && run "rm -f '$BASH_DIR/confb'"
[ -f "$ZSH_DIR/_confb" ] && run "rm -f '$ZSH_DIR/_confb'"
[ -f "$FISH_DIR/confb.fish" ] && run "rm -f '$FISH_DIR/confb.fish'"

# remove systemd unit
[ -f "$UNIT_PATH" ] && run "rm -f '$UNIT_PATH'"

# (optional) purge config directory
if [ "$PURGE" -eq 1 ]; then
  [ -d "$CONFIG_DIR" ] && run "rm -rf '$CONFIG_DIR'"
fi

log "Done."
