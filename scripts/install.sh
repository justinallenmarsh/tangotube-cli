#!/usr/bin/env bash
# Install tt, the TangoTube CLI.
#
#   curl -fsSL https://tangotube.tv/install-cli | bash
#
# TANGOTUBE_BIN_DIR    where tt goes (default ~/.local/bin)
# TANGOTUBE_VERSION    a release tag like v0.1.0 (default: the latest)
# TANGOTUBE_SKIP_SETUP set to skip installing the agent skill afterwards
set -euo pipefail

REPO="justinallenmarsh/tangotube-cli"
BIN_DIR="${TANGOTUBE_BIN_DIR:-$HOME/.local/bin}"
VERSION="${TANGOTUBE_VERSION:-latest}"

say() { printf '  %s\n' "$*" >&2; }
die() { printf '  tt: %s\n' "$*" >&2; exit 1; }

case "$(uname -s)" in
  Darwin) os=darwin ;;
  Linux) os=linux ;;
  *) die "tt ships for macOS and Linux; on anything else, build it from https://github.com/$REPO" ;;
esac
case "$(uname -m)" in
  x86_64 | amd64) arch=amd64 ;;
  arm64 | aarch64) arch=arm64 ;;
  *) die "tt ships for amd64 and arm64, not $(uname -m)" ;;
esac

if [ "$VERSION" = latest ]; then
  base="https://github.com/$REPO/releases/latest/download"
else
  base="https://github.com/$REPO/releases/download/$VERSION"
fi
asset="tt_${os}_${arch}.tar.gz"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

sha256() {
  if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | cut -d' ' -f1
  else shasum -a 256 "$1" | cut -d' ' -f1; fi
}

from_release() {
  command -v curl >/dev/null 2>&1 || return 1
  curl -fsSL "$base/$asset" -o "$tmp/$asset" 2>/dev/null || return 1
  curl -fsSL "$base/checksums.txt" -o "$tmp/checksums.txt" 2>/dev/null || die "the release has no checksums.txt; not installing an unchecked binary"
  want="$(grep " $asset\$" "$tmp/checksums.txt" | cut -d' ' -f1)"
  [ -n "$want" ] || die "checksums.txt does not list $asset"
  [ "$(sha256 "$tmp/$asset")" = "$want" ] || die "$asset does not match its checksum; not installing it"
  tar -xzf "$tmp/$asset" -C "$tmp" tt
  say "Downloaded tt ($VERSION, $os/$arch), checksum verified."
}

from_source() {
  command -v go >/dev/null 2>&1 || return 1
  here="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" 2>/dev/null && pwd || true)"
  if [ -n "$here" ] && [ -f "$here/../go.mod" ] && grep -q "module github.com/$REPO" "$here/../go.mod"; then
    say "No release to download; building tt from $(cd "$here/.." && pwd)."
    # The same version stamp as make build, so tt version says what it is.
    (
      cd "$here/.."
      pkg="github.com/$REPO/internal/commands"
      ver="$(git describe --tags --always --dirty 2>/dev/null || echo dev)"
      commit="$(git rev-parse --short HEAD 2>/dev/null || echo none)"
      date="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
      go build -trimpath -ldflags "-s -w -X $pkg.Version=$ver -X $pkg.Commit=$commit -X $pkg.Date=$date" -o "$tmp/tt" ./cmd/tt
    )
  else
    say "No release to download; building tt with go install."
    GOBIN="$tmp" go install "github.com/$REPO/cmd/tt@$VERSION"
  fi
}

from_release || from_source ||
  die "no release for $os/$arch could be downloaded, and Go is not installed to build one"

mkdir -p "$BIN_DIR"
install -m 0755 "$tmp/tt" "$BIN_DIR/tt"
say "Installed $BIN_DIR/tt"

case ":$PATH:" in
  *":$BIN_DIR:"*) ;;
  *) say "Add $BIN_DIR to your PATH:  export PATH=\"$BIN_DIR:\$PATH\"" ;;
esac

if [ -z "${TANGOTUBE_SKIP_SETUP:-}" ] && [ -t 1 ] && [ -t 2 ]; then
  "$BIN_DIR/tt" setup || true
fi

say ""
say "Next:  tt search \"di sarli noelia\""
