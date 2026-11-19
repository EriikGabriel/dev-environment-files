#!/usr/bin/env bash
# Downloads the latest envsetup from the GitHub releases and installs it:
# the .deb on Ubuntu/Debian, the AppImage on Arch (and any other Linux).
#
#   curl -fsSL https://github.com/EriikGabriel/dev-environment-files/releases/latest/download/install.sh | bash
set -euo pipefail

repo="EriikGabriel/dev-environment-files"
api="https://api.github.com/repos/$repo/releases/latest"

case "$(uname -m)" in
  x86_64 | amd64) ;;
  *) echo "only x86_64 builds are published" >&2; exit 1 ;;
esac

# asset prints the download link of the first asset whose name ends with $1
asset() {
  curl -fsSL "$api" | grep -o '"browser_download_url": *"[^"]*'"$1"'"' | head -n1 | sed 's/.*"\(https[^"]*\)"$/\1/'
}

sudo=""
[ "$(id -u)" -eq 0 ] || sudo="sudo"

id="" like=""
if [ -r /etc/os-release ]; then
  . /etc/os-release
  id="${ID:-}" like="${ID_LIKE:-}"
fi
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

case " $id $like " in
  *" debian "* | *" ubuntu "*)
    url="$(asset '_amd64.deb')"
    [ -n "$url" ] || { echo "the latest release has no .deb" >&2; exit 1; }
    echo "downloading $url"
    curl -fL --progress-bar -o "$tmp/envsetup.deb" "$url"
    $sudo apt-get install -y "$tmp/envsetup.deb"
    echo "installed: open envsetup from the applications menu"
    ;;
  *)
    url="$(asset '_amd64.AppImage')"
    [ -n "$url" ] || { echo "the latest release has no AppImage" >&2; exit 1; }
    if [ "$id" = "arch" ] || [ "$id" = "cachyos" ] || [[ " $like " == *" arch "* ]]; then
      # AppImages need FUSE 2
      $sudo pacman -S --needed --noconfirm fuse2
    fi
    bin="$HOME/.local/bin"
    apps="$HOME/.local/share/applications"
    mkdir -p "$bin" "$apps"
    echo "downloading $url"
    curl -fL --progress-bar -o "$bin/envsetup.AppImage" "$url"
    chmod +x "$bin/envsetup.AppImage"
    cat >"$apps/envsetup.desktop" <<EOF
[Desktop Entry]
Name=envsetup
Comment=Configure and reinstall this machine
Exec=$bin/envsetup.AppImage
Icon=utilities-terminal
Terminal=false
Type=Application
Categories=Utility;System;
EOF
    echo "installed: $bin/envsetup.AppImage (also in the applications menu)"
    ;;
esac
