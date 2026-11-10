#!/bin/sh
# Download only. No node, login service, configuration, or PATH changes.
# Keep execution at the end: a truncated curl | sh must not begin an install.
main() {
    set -eu
    version=${DIALTONE_VERSION:-v0.5.3}
    install_dir=${DIALTONE_INSTALL_DIR:-"$HOME/.local/bin"}
    release_base=${DIALTONE_RELEASE_BASE_URL:-https://github.com/timcash/dialtone/releases}
    download_dir=
    stage_dir=

    die() { printf 'dialtone installer: %s\n' "$*" >&2; exit 1; }
    cleanup() {
        [ -z "$download_dir" ] || rm -rf -- "$download_dir"
        [ -z "$stage_dir" ] || rm -rf -- "$stage_dir"
    }
    trap cleanup 0
    trap 'exit 1' 1 2 15

    while [ "$#" -gt 0 ]; do
        case "$1" in
            --version|--install-dir)
                [ "$#" -ge 2 ] || die "$1 needs a value"
                case "$1" in
                    --version) version=$2 ;;
                    --install-dir) install_dir=$2 ;;
                esac
                shift 2 ;;
            -h|--help)
                printf '%s\n' 'Usage: sh install.sh [--version v0.5.3] [--install-dir DIR]'
                return ;;
            *) die "unknown option: $1" ;;
        esac
    done
    # A single explicit tag supplies BOTH requests; never use latest/download.
    case "$version" in *[!A-Za-z0-9.-]*) die 'version must be a tag such as v0.5.3' ;; esac
    printf '%s\n' "$version" | LC_ALL=C grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+(-[A-Za-z0-9][A-Za-z0-9.-]*)?$' || die 'version must be a tag such as v0.5.3'
    [ -n "$install_dir" ] || die 'install directory is empty'
    case "$install_dir" in /*) ;; *) install_dir="$PWD/$install_dir" ;; esac
    release_base=${release_base%/}
    # HTTPS for GitHub/mirrors; plain HTTP is limited to local fixture servers.
    case "$release_base" in
        https://*) protocols='=https'; redirect_limit=5 ;;
        http://127.0.0.1:*)
            printf '%s\n' "$release_base" | LC_ALL=C grep -Eq '^http://127\.0\.0\.1:[0-9]+(/[^?#]*)?$' || die 'invalid loopback release base'
            protocols='=http'; redirect_limit=0 ;;
        *) die 'release base must use HTTPS (or loopback HTTP for tests)' ;;
    esac

    os=$(uname -s)
    arch=$(uname -m)
    # A Rosetta shell still runs on Apple Silicon; select its native binary.
    if [ "$os" = Darwin ] && [ "$arch" = x86_64 ] && [ "$(sysctl -n hw.optional.arm64 2>/dev/null || :)" = 1 ]; then
        arch=arm64
    fi
    case "$os/$arch" in
        Linux/x86_64|Linux/amd64) asset=dialtone-linux-x86_64 ;;
        Linux/aarch64|Linux/arm64) asset=dialtone-linux-arm64 ;;
        Darwin/arm64|Darwin/aarch64) asset=dialtone-macos-arm64 ;;
        *) die "unsupported platform: $os/$arch; see docs/install.md for source builds" ;;
    esac
    command -v curl >/dev/null 2>&1 || die 'curl is required'
    if command -v sha256sum >/dev/null 2>&1; then
        hash_tool=sha256sum
    elif command -v shasum >/dev/null 2>&1; then
        hash_tool=shasum
    else
        die 'sha256sum or shasum is required'
    fi

    download_dir=$(mktemp -d "${TMPDIR:-/tmp}/dialtone-install.XXXXXXXX")
    url="$release_base/download/$version/$asset"
    curl --fail --silent --show-error --location --max-redirs "$redirect_limit" --proto "$protocols" --proto-redir "$protocols" --connect-timeout 15 --max-time 300 "$url" -o "$download_dir/binary"
    curl --fail --silent --show-error --location --max-redirs "$redirect_limit" --proto "$protocols" --proto-redir "$protocols" --connect-timeout 15 --max-time 60 "$url.sha256" -o "$download_dir/checksum"
    # Accept exactly the release pipeline's one-line sha256sum record, with the
    # expected filename. Never let a checksum file choose a local path to read.
    [ "$(wc -c < "$download_dir/checksum" | tr -d ' ')" = "$((67 + ${#asset}))" ] || die 'invalid checksum record'
    [ "$(wc -l < "$download_dir/checksum" | tr -d ' ')" = 1 ] || die 'invalid checksum record'
    LC_ALL=C grep -Eq "^[0-9a-fA-F]{64}  $asset$" "$download_dir/checksum" || die 'invalid checksum record'
    expected=$(cut -c 1-64 "$download_dir/checksum" | tr 'A-F' 'a-f')
    if [ "$hash_tool" = sha256sum ]; then
        actual=$(sha256sum < "$download_dir/binary")
    else
        actual=$(shasum -a 256 < "$download_dir/binary")
    fi
    actual=${actual%% *}
    [ "$actual" = "$expected" ] || die 'SHA-256 mismatch; previous install preserved'
    [ -s "$download_dir/binary" ] || die 'empty binary'

    # Stage on the destination filesystem so the final rename is atomic. Nothing
    # touches the previous executable until all downloads and checks succeed.
    mkdir -p -- "$install_dir"
    [ ! -d "$install_dir/dialtone" ] || die 'destination is a directory'
    stage_dir=$(mktemp -d "$install_dir/.dialtone-install.XXXXXXXX")
    cp -- "$download_dir/binary" "$stage_dir/dialtone"
    chmod 755 "$stage_dir/dialtone"
    mv -f -- "$stage_dir/dialtone" "$install_dir/dialtone"
    printf 'Installed %s to %s\n' "$version" "$install_dir/dialtone"
    printf '%s\n' 'No node started. Add the install directory to your PATH, or use the full path.' 'See https://github.com/timcash/dialtone/blob/main/docs/install.md'
}

main "$@"
