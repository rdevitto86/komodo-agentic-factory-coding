#!/usr/bin/env bash
# Installs komodo on macOS, Linux or WSL2; running it again updates it in place.
# Run: ./install.sh from a komodo checkout, or bash install.sh once downloaded.
# It names any missing prerequisite, builds with Go from the checkout or downloads the pinned
# release and verifies its SHA-256, links komodo into KOMODO_BIN_DIR, runs komodo install --global,
# then komodo init and komodo doctor inside a repo. Doctor's findings are reported, never fatal.
# Environment: KOMODO_VERSION, KOMODO_RELEASE_URL, KOMODO_BIN_DIR (default ~/.local/bin).
set -euo pipefail

KOMODO_VERSION="${KOMODO_VERSION:-v1.0.0-beta.2}"
KOMODO_RELEASE_URL="${KOMODO_RELEASE_URL:-https://github.com/rdevitto86/komodo-agentic-factory-coding/releases/download}"
KOMODO_BIN_DIR="${KOMODO_BIN_DIR:-$HOME/.local/bin}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/komodo-install.XXXXXX")"
OS=""
ARCH=""
BINARY=""

cleanup() {
  rm -rf "$WORK_DIR"
}
trap cleanup EXIT

usage() {
  printf 'usage: %s [-h]\n' "$0" >&2
  printf 'Installs or updates komodo. KOMODO_VERSION, KOMODO_RELEASE_URL and KOMODO_BIN_DIR change the defaults.\n' >&2
}

say() {
  printf 'install: %s\n' "$*"
}

die() {
  printf 'install: %s\n' "$*" >&2
  exit 1
}

# Sets OS and ARCH from uname, or dies on an unsupported platform.
detect_platform() {
  case "$(uname -s)" in
    Darwin) OS=darwin ;;
    Linux) OS=linux ;;
    *) die "$(uname -s) is not supported here; on native Windows run install.ps1" ;;
  esac
  case "$(uname -m)" in
    x86_64 | amd64) ARCH=amd64 ;;
    arm64 | aarch64) ARCH=arm64 ;;
    *) die "$(uname -m) has no komodo build" ;;
  esac
}

# Checks for git and go on PATH, printing an install hint for whichever is missing.
check_prerequisites() {
  local missing=0
  if ! command -v git >/dev/null 2>&1; then
    missing=1
    if [ "$OS" = darwin ]; then
      printf 'install: git is missing; run xcode-select --install, or brew install git\n' >&2
    else
      printf 'install: git is missing; install it with your package manager, such as sudo apt install git\n' >&2
    fi
  fi
  if ! command -v claude >/dev/null 2>&1; then
    missing=1
    printf 'install: Claude Code is missing; run npm install -g @anthropic-ai/claude-code, or see %s\n' \
      "https://docs.claude.com/en/docs/claude-code/setup" >&2
  fi
  if [ "$missing" -ne 0 ]; then
    exit 1
  fi
}

check_wsl_filesystem() {
  local repo="$1"
  if [ -r /proc/version ] && grep -qi microsoft /proc/version; then
    case "$repo" in
      /mnt/*) die "$repo is on the Windows filesystem; clone it under your Linux home, such as ~/src, and run this again" ;;
    esac
  fi
}

sha256_of() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{ print $1 }'
  else
    shasum -a 256 "$1" | awk '{ print $1 }'
  fi
}

build_binary() {
  local name="komodo"
  say "building $name from $SCRIPT_DIR"
  mkdir -p "$SCRIPT_DIR/bin"
  (cd "$SCRIPT_DIR" && go build -o "bin/$name" ./cmd/komodo)
  BINARY="$SCRIPT_DIR/bin/$name"
}

# Downloads the release binary for OS and ARCH, verifying it before use.
download_binary() {
  local name="komodo-$OS-$ARCH"
  local base="$KOMODO_RELEASE_URL/$KOMODO_VERSION"
  local dir="$HOME/.komodo/bin"
  local want got
  if ! command -v curl >/dev/null 2>&1; then
    die "curl is missing; install it with your package manager, or install Go from https://go.dev/dl and run this from a checkout"
  fi
  say "downloading $name $KOMODO_VERSION"
  curl -fsSL -o "$WORK_DIR/$name" "$base/$name" || die "could not download $base/$name"
  curl -fsSL -o "$WORK_DIR/SHA256SUMS" "$base/SHA256SUMS" || die "could not download $base/SHA256SUMS"
  want="$(awk -v file="$name" '$2 == file || $2 == "*" file { print $1 }' "$WORK_DIR/SHA256SUMS")"
  if [ -z "$want" ]; then
    die "SHA256SUMS for $KOMODO_VERSION lists no $name"
  fi
  got="$(sha256_of "$WORK_DIR/$name")"
  if [ "$got" != "$want" ]; then
    die "checksum mismatch for $name: got $got, want $want; nothing was installed"
  fi
  mkdir -p "$dir"
  chmod 0755 "$WORK_DIR/$name"
  mv -f "$WORK_DIR/$name" "$dir/$name"
  BINARY="$dir/$name"
}

link_binary() {
  mkdir -p "$KOMODO_BIN_DIR"
  ln -sfn "$BINARY" "$KOMODO_BIN_DIR/komodo"
  say "linked $KOMODO_BIN_DIR/komodo -> $BINARY"
  case ":$PATH:" in
    *":$KOMODO_BIN_DIR:"*) ;;
    *) say "add $KOMODO_BIN_DIR to PATH, such as in ~/.profile: export PATH=\"$KOMODO_BIN_DIR:\$PATH\"" ;;
  esac
}

# Parses flags, resolves the binary, and installs it, or shows usage on -h/--help.
main() {
  if [ "$#" -gt 0 ]; then
    case "$1" in
      -h | --help)
        usage
        exit 0
        ;;
      *)
        usage
        exit 2
        ;;
    esac
  fi
  detect_platform
  check_prerequisites
  local repo komodo="$KOMODO_BIN_DIR/komodo"
  repo="$(git rev-parse --show-toplevel 2>/dev/null || true)"
  if [ -n "$repo" ]; then
    check_wsl_filesystem "$repo"
  fi
  if command -v go >/dev/null 2>&1 && [ -f "$SCRIPT_DIR/cmd/komodo/main.go" ]; then
    build_binary
  else
    download_binary
  fi
  link_binary
  if [ -z "$repo" ]; then
    # komodo runs inside a git repository, so the global install runs in the checkout, else an empty one.
    local toolkit
    toolkit="$(cd "$SCRIPT_DIR" && git rev-parse --show-toplevel 2>/dev/null || true)"
    if [ -z "$toolkit" ]; then
      toolkit="$WORK_DIR/empty"
      git init -q --template= "$toolkit"
    fi
    (cd "$toolkit" && "$komodo" install --global)
    say "done; run komodo init inside a repo to add the line to it"
    return
  fi
  (cd "$repo" && "$komodo" install --global)
  (cd "$repo" && "$komodo" init)
  if ! (cd "$repo" && "$komodo" doctor); then
    say "komodo doctor found the problems above; fix them and run komodo doctor again"
  fi
  say "done"
}

main "$@"
