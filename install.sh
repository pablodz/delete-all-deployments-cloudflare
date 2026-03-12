#!/bin/sh

set -eu

REPO="pablodz/delete-all-deployments-cloudflare"
BINARY_NAME="delete-all-deployments-cloudflare"
INSTALL_DIR="${INSTALL_DIR:-$PWD}"
VERSION="${VERSION:-latest}"

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
		armv7l)
			echo "armv7"
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

release_api_url() {
	if [ "$VERSION" = "latest" ]; then
		echo "https://api.github.com/repos/$REPO/releases/latest"
	else
		echo "https://api.github.com/repos/$REPO/releases/tags/$VERSION"
	fi
}

json_string_values() {
	key="$1"
	printf '%s\n' "$RELEASE_JSON" \
		| tr ',' '\n' \
		| sed -n "s/.*\"$key\"[[:space:]]*:[[:space:]]*\"\([^\"]*\)\".*/\1/p"
}

extract_json_value() {
	key="$1"
	json_string_values "$key" | head -n 1
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

extract_archive() {
	archive_path="$1"
	destination_dir="$2"

	case "$archive_path" in
		*.tar.gz)
			tar -xzf "$archive_path" -C "$destination_dir"
			;;
		*.zip)
			require_command unzip
			unzip -q "$archive_path" -d "$destination_dir"
			;;
		*)
			echo "error: unsupported archive format: $archive_path" >&2
			exit 1
			;;
	esac
}

find_binary() {
	search_dir="$1"
	find "$search_dir" -type f \( -name "$BINARY_NAME" -o -name "$BINARY_NAME.exe" \) | head -n 1
}

install_binary() {
	binary_path="$1"
	mkdir -p "$INSTALL_DIR"
	target_path="$INSTALL_DIR/$BINARY_NAME"
	cp "$binary_path" "$target_path"
	chmod +x "$target_path"
	echo "installed $target_path"
}

build_from_source() {
	source_url="$1"
	work_dir="$2"
	require_command go

	source_archive="$work_dir/source.tar.gz"
	source_dir="$work_dir/source"
	mkdir -p "$source_dir"
	download_file "$source_url" "$source_archive"
	tar -xzf "$source_archive" -C "$source_dir" --strip-components=1
	(
		cd "$source_dir"
		GO111MODULE=on go build -o "$work_dir/$BINARY_NAME" .
	)
	install_binary "$work_dir/$BINARY_NAME"
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
RELEASE_JSON="$(curl -fsSL -H 'Accept: application/vnd.github+json' "$(release_api_url)")"
ASSET_URL="$(select_asset_url "$OS_NAME" "$ARCH_NAME" || true)"

if [ -n "$ASSET_URL" ]; then
	ARCHIVE_NAME=$(basename "$ASSET_URL")
	ARCHIVE_PATH="$TMP_DIR/$ARCHIVE_NAME"
	EXTRACT_DIR="$TMP_DIR/extracted"
	mkdir -p "$EXTRACT_DIR"

	echo "downloading release asset: $ARCHIVE_NAME"
	download_file "$ASSET_URL" "$ARCHIVE_PATH"
	extract_archive "$ARCHIVE_PATH" "$EXTRACT_DIR"

	BINARY_PATH="$(find_binary "$EXTRACT_DIR")"
	if [ -z "$BINARY_PATH" ]; then
		echo "error: downloaded archive does not contain $BINARY_NAME" >&2
		exit 1
	fi

	install_binary "$BINARY_PATH"
	exit 0
fi

SOURCE_URL="$(extract_json_value tarball_url)"
if [ -z "$SOURCE_URL" ]; then
	echo "error: no matching release asset found for ${OS_NAME}/${ARCH_NAME} and source fallback is unavailable" >&2
	exit 1
fi

echo "no prebuilt asset found for ${OS_NAME}/${ARCH_NAME}; falling back to source build"
build_from_source "$SOURCE_URL" "$TMP_DIR"