#!/bin/sh

set -eu

REPO="pablodz/delete-all-deployments-cloudflare"
BINARY_NAME="delete-all-deployments-cloudflare"

require_command() {
	command -v "$1" >/dev/null 2>&1 || {
		echo "error: required command not found: $1" >&2
		exit 1
	}
}

detect_os() {
	case "$(uname -s)" in
		Linux)
			echo "linux"
			;;
		Darwin)
			echo "darwin"
			;;
		MINGW*|MSYS*|CYGWIN*)
			echo "windows"
			;;
		*)
			echo "unsupported operating system: $(uname -s)" >&2
			exit 1
			;;
	esac
}

detect_arch() {
	case "$(uname -m)" in
		x86_64|amd64)
			echo "amd64"
			;;
		aarch64|arm64)
			echo "arm64"
			;;
		i386|i686)
			echo "386"
			;;
		*)
			echo "unsupported architecture: $(uname -m)" >&2
			exit 1
			;;
	esac
}

json_string_values() {
	key="$1"
	printf '%s\n' "$RELEASE_JSON" \
		| tr ',' '\n' \
		| sed -n "s/.*\"$key\"[[:space:]]*:[[:space:]]*\"\([^\"]*\)\".*/\1/p"
}

select_asset_url() {
	os_name="$1"
	arch_name="$2"
	asset_pattern="/${BINARY_NAME}_[^/]*_${os_name}_${arch_name}\\.tar\\.gz$"
	json_string_values "browser_download_url" \
		| grep "$asset_pattern" \
		| head -n 1
}

download_file() {
	url="$1"
	output_path="$2"
	curl -fsSL "$url" -o "$output_path"
}

find_binary() {
	search_dir="$1"
	find "$search_dir" -type f -name "$BINARY_NAME" | head -n 1
}

require_command curl
require_command tar
require_command find
require_command sed
require_command grep

OS_NAME="$(detect_os)"
ARCH_NAME="$(detect_arch)"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT INT TERM

echo "detecting platform: ${OS_NAME}/${ARCH_NAME}"
RELEASE_JSON="$(curl -fsSL -H 'Accept: application/vnd.github+json' "https://api.github.com/repos/$REPO/releases/latest")"
ASSET_URL="$(select_asset_url "$OS_NAME" "$ARCH_NAME" || true)"

if [ -z "$ASSET_URL" ]; then
	echo "error: no release asset found for ${OS_NAME}/${ARCH_NAME}" >&2
	exit 1
fi

ARCHIVE_NAME=$(basename "$ASSET_URL")
ARCHIVE_PATH="$TMP_DIR/$ARCHIVE_NAME"

echo "downloading release asset: $ARCHIVE_NAME"
download_file "$ASSET_URL" "$ARCHIVE_PATH"
tar -xzf "$ARCHIVE_PATH" -C "$TMP_DIR"

BINARY_PATH="$(find_binary "$TMP_DIR")"
if [ -z "$BINARY_PATH" ]; then
	echo "error: downloaded archive does not contain $BINARY_NAME" >&2
	exit 1
fi

cp "$BINARY_PATH" "$PWD/$BINARY_NAME"
chmod +x "$PWD/$BINARY_NAME"
echo "installed $PWD/$BINARY_NAME"