#!/usr/bin/env bash
# cadencefmt installer for macOS / Linux.
#
# Downloads the latest cadencefmt-lsp binary from GitHub Releases, installs
# the cadencefmt VS Code extension via the first available editor command
# (code / cursor / codium / code-insiders), and is idempotent so re-running
# upgrades both. Windows is not currently supported by this installer.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/janezpodhostnik/cadencefmt/main/install.sh | bash
#
# Environment overrides:
#   PREFIX     install directory for the binary (default: $HOME/.local/bin)
#   VERSION    pin a specific tag, e.g. v0.1.0 (default: latest)
#   EDITOR_CMD force a specific editor command (default: auto-detect)
#   SKIP_VSIX  set to 1 to skip the extension install
#   SKIP_LSP   set to 1 to skip the binary install

set -euo pipefail

REPO="janezpodhostnik/cadencefmt"
PREFIX="${PREFIX:-$HOME/.local/bin}"

log() { printf 'cadencefmt: %s\n' "$*"; }
err() { printf 'cadencefmt: error: %s\n' "$*" >&2; }

# Platform detection ----------------------------------------------------------

case "$(uname -s)" in
  Linux*)  goos=linux ;;
  Darwin*) goos=darwin ;;
  *)
    err "unsupported OS: $(uname -s). Only macOS and Linux are supported."
    exit 1
    ;;
esac
case "$(uname -m)" in
  x86_64|amd64)  goarch=amd64 ;;
  aarch64|arm64) goarch=arm64 ;;
  *)
    err "unsupported architecture: $(uname -m)."
    exit 1
    ;;
esac
log "platform: $goos/$goarch"

# Resolve version -------------------------------------------------------------

if [ -z "${VERSION:-}" ]; then
  VERSION=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
    | grep -E '"tag_name"' | head -1 | cut -d'"' -f4)
  if [ -z "$VERSION" ]; then
    err "could not resolve latest release. Pass VERSION=v0.1.0 to override."
    exit 1
  fi
fi
log "version: $VERSION"

base="https://github.com/$REPO/releases/download/$VERSION"

# Install cadencefmt-lsp ------------------------------------------------------

if [ "${SKIP_LSP:-0}" != "1" ]; then
  mkdir -p "$PREFIX"
  url="$base/cadencefmt-lsp-$goos-$goarch"
  log "downloading $url"
  curl -fsSL "$url" -o "$PREFIX/cadencefmt-lsp"
  chmod +x "$PREFIX/cadencefmt-lsp"
  log "installed cadencefmt-lsp to $PREFIX/cadencefmt-lsp"

  # Also install cadencefmt (CLI) — same release, same prefix
  url="$base/cadencefmt-$goos-$goarch"
  log "downloading $url"
  curl -fsSL "$url" -o "$PREFIX/cadencefmt"
  chmod +x "$PREFIX/cadencefmt"
  log "installed cadencefmt to $PREFIX/cadencefmt"

  case ":$PATH:" in
    *":$PREFIX:"*) ;;
    *) log "note: $PREFIX is not on PATH — add it to your shell's rc file." ;;
  esac
fi

# Install VS Code extension ---------------------------------------------------

if [ "${SKIP_VSIX:-0}" != "1" ]; then
  editor="${EDITOR_CMD:-}"
  if [ -z "$editor" ]; then
    for cmd in code cursor codium code-insiders; do
      if command -v "$cmd" >/dev/null 2>&1; then
        editor="$cmd"
        break
      fi
    done
  fi
  if [ -z "$editor" ]; then
    err "no editor command found (tried: code, cursor, codium, code-insiders)."
    err "skipping extension install. Pass EDITOR_CMD=<cmd> or SKIP_VSIX=1 to silence."
    exit 1
  fi

  vsix=$(mktemp -t cadencefmt.XXXXXX).vsix
  trap 'rm -f "$vsix"' EXIT
  url="$base/cadencefmt.vsix"
  log "downloading $url"
  curl -fsSL "$url" -o "$vsix"
  log "installing extension via: $editor"
  "$editor" --install-extension "$vsix" --force
fi

log "done."
