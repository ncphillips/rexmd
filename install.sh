#!/bin/sh
set -eu

case "$(uname -s)" in
  Darwin) os=darwin ;;
  Linux) os=linux ;;
  *) echo "Unsupported operating system" >&2; exit 1 ;;
esac
case "$(uname -m)" in
  arm64|aarch64) arch=arm64 ;;
  x86_64|amd64) arch=amd64 ;;
  *) echo "Unsupported architecture" >&2; exit 1 ;;
esac

archive="rexmd_${os}_${arch}"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' 0
trap 'exit 1' HUP INT TERM

curl -fL "https://github.com/ncphillips/rex-md/releases/latest/download/${archive}.tar.gz" -o "$tmp/rexmd.tar.gz"
tar -xzf "$tmp/rexmd.tar.gz" -C "$tmp"
mkdir -p "$HOME/.local/bin"
install -m 755 "$tmp/$archive/rexmd" "$HOME/.local/bin/rexmd"
echo "Installed rexmd to $HOME/.local/bin/rexmd"
