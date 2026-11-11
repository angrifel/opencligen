#!/bin/sh
# Installs the release tools into .tools/bin of this repository, so nothing
# is installed system-wide. Versions and SHA-256 checksums are pinned below;
# a download whose checksum does not match is rejected.
#
# Usage: scripts/install-tools.sh [cog|goreleaser|syft]...   (default: all)
#
# Needs only curl, tar and sha256sum (Linux) or shasum (macOS). Supported
# platforms: Linux and macOS on amd64 and arm64.
#
# To upgrade a tool, change its version and the checksums of every platform.
# GoReleaser and Syft checksums come from the checksums files of their
# GitHub releases; Cocogitto publishes none, so its checksums were computed
# from the release downloads when they were pinned. Syft generates the SBOM
# of a release (.goreleaser.yaml).
set -eu

COG_VERSION=7.0.0
GORELEASER_VERSION=2.20.0
SYFT_VERSION=1.57.1

root=$(cd "$(dirname "$0")/.." && pwd)
bin="$root/.tools/bin"

os=$(uname -s)
arch=$(uname -m)
case "$os/$arch" in
Linux/x86_64)
	cog_target=x86_64-unknown-linux-musl
	cog_sha=e03938ff2c4c86d71c00c0f3284dbbe95c5ca76fe34a51f33e945c23010d59bb
	gr_target=Linux_x86_64
	gr_sha=d2483159e6007f814132cb56635ad30a70013dbeb813d6033a7e01adf6ae108c
	syft_target=linux_amd64
	syft_sha=bf52519eb57f675d47c07d43213cb6dff6449f3d6d1147ff6701ed91f44eef0c
	;;
Linux/aarch64 | Linux/arm64)
	cog_target=aarch64-unknown-linux-gnu
	cog_sha=379949f50c4f8245ef9532913e83f351c048b55e40ebd52ed8a850642c07a753
	gr_target=Linux_arm64
	gr_sha=2dada0a6bbcd10f3a05228d98e77d25fb362efb9e5bab6362cf37e64c8a26cb9
	syft_target=linux_arm64
	syft_sha=aa3d3e07344f590bcce8d31043b752daa76c8a56c2aa34ceeee63c6168152582
	;;
Darwin/x86_64)
	cog_target=x86_64-apple-darwin
	cog_sha=591dcd462e2e35ecc21caf41a15a65d464b7ce1ce8a5c299fa0d46a7798dddb0
	gr_target=Darwin_x86_64
	gr_sha=a1dfe9492c67b860129bc08fa5952955f0666abe4741e1846f93a6c6b47360e6
	syft_target=darwin_amd64
	syft_sha=2355758c1142982a67b309604c5d75effdbe638a57c771979ea8fd1a16656abc
	;;
Darwin/arm64)
	cog_target=aarch64-apple-darwin
	cog_sha=429303b6d68c6461aa221ab4de82b26d0bbcdee7127947ea9c7c411ab9bc574c
	gr_target=Darwin_arm64
	gr_sha=50bd83a6fcbf1dac272cfc4c46f9116052a9b0b463404a044af71a832fd92ad6
	syft_target=darwin_arm64
	syft_sha=e0a74a5d2688d1254b4e17b71f068a83215e3625f846265bf6614c7ba06f3a96
	;;
*)
	echo "install-tools: unsupported platform $os/$arch" >&2
	exit 1
	;;
esac

sha256() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | cut -d' ' -f1
	else
		shasum -a 256 "$1" | cut -d' ' -f1
	fi
}

# install NAME VERSION URL SHA256 PATH_IN_ARCHIVE
install() {
	name=$1 version=$2 url=$3 want=$4 member=$5
	stamp="$bin/.$name.version"
	if [ -x "$bin/$name" ] && [ "$(cat "$stamp" 2>/dev/null)" = "$version" ]; then
		return
	fi
	tmp=$(mktemp -d)
	trap 'rm -rf "$tmp"' EXIT
	echo "install-tools: downloading $name $version"
	curl -fsSL -o "$tmp/archive.tar.gz" "$url"
	got=$(sha256 "$tmp/archive.tar.gz")
	if [ "$got" != "$want" ]; then
		echo "install-tools: checksum mismatch for $url" >&2
		echo "  want $want" >&2
		echo "  got  $got" >&2
		exit 1
	fi
	tar -xzf "$tmp/archive.tar.gz" -C "$tmp" "$member"
	mkdir -p "$bin"
	mv "$tmp/$member" "$bin/$name"
	chmod 0755 "$bin/$name"
	echo "$version" >"$stamp"
	rm -rf "$tmp"
	trap - EXIT
}

tools=${*:-cog goreleaser syft}
for tool in $tools; do
	case $tool in
	cog)
		install cog "$COG_VERSION" \
			"https://github.com/cocogitto/cocogitto/releases/download/$COG_VERSION/cocogitto-$COG_VERSION-$cog_target.tar.gz" \
			"$cog_sha" "$cog_target/cog"
		;;
	goreleaser)
		install goreleaser "$GORELEASER_VERSION" \
			"https://github.com/goreleaser/goreleaser/releases/download/v$GORELEASER_VERSION/goreleaser_$gr_target.tar.gz" \
			"$gr_sha" goreleaser
		;;
	syft)
		install syft "$SYFT_VERSION" \
			"https://github.com/anchore/syft/releases/download/v$SYFT_VERSION/syft_${SYFT_VERSION}_$syft_target.tar.gz" \
			"$syft_sha" syft
		;;
	*)
		echo "install-tools: unknown tool $tool" >&2
		exit 1
		;;
	esac
done
