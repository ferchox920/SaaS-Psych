#!/usr/bin/env bash
set -euo pipefail

MIGRATE_VERSION="${MIGRATE_VERSION:-v4.18.3}"
INSTALL_DIR="${MIGRATE_INSTALL_DIR:-/usr/local/bin}"

if command -v migrate >/dev/null 2>&1; then
  echo "migrate already installed: $(command -v migrate)"
  migrate -version
  exit 0
fi

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch_raw="$(uname -m)"
case "$arch_raw" in
  x86_64) arch="amd64" ;;
  aarch64|arm64) arch="arm64" ;;
  *)
    echo "unsupported architecture: $arch_raw"
    exit 1
    ;;
esac

case "$os" in
  linux|darwin) ;;
  *)
    echo "unsupported OS: $os (this helper supports Linux/macOS)"
    exit 1
    ;;
esac

artifact="migrate.${os}-${arch}.tar.gz"
url="https://github.com/golang-migrate/migrate/releases/download/${MIGRATE_VERSION}/${artifact}"

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

echo "downloading ${url}"
curl -fsSL "$url" -o "${tmp_dir}/migrate.tar.gz"
tar -xzf "${tmp_dir}/migrate.tar.gz" -C "$tmp_dir"

if [[ ! -x "${tmp_dir}/migrate" ]]; then
  echo "migrate binary not found in archive: ${artifact}"
  exit 1
fi

if [[ -w "$INSTALL_DIR" ]]; then
  install -m 0755 "${tmp_dir}/migrate" "${INSTALL_DIR}/migrate"
else
  sudo install -m 0755 "${tmp_dir}/migrate" "${INSTALL_DIR}/migrate"
fi

if command -v migrate >/dev/null 2>&1; then
  echo "migrate installed successfully: $(command -v migrate)"
  migrate -version
else
  echo "migrate installed to ${INSTALL_DIR}/migrate but not found in PATH"
  "${INSTALL_DIR}/migrate" -version
fi
