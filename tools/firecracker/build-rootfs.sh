#!/usr/bin/env bash
#
# Build an Alpine-based ext4 rootfs image for a given runtime.
# Usage: ./build-rootfs.sh <runtime> [image_size_mb]
#
# Requires: sudo, mkfs.ext4, mount, curl
#
set -euo pipefail

RUNTIME="${1:?Usage: $0 <go|node|python> [image_size_mb]}"
IMAGE_SIZE_MB="${2:-512}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
IMAGES_DIR="${SCRIPT_DIR}/images"
BIN_DIR="${SCRIPT_DIR}/bin"
GUEST_BIN="${BIN_DIR}/vulcan-guest"

# Alpine minirootfs version
ALPINE_VERSION="${ALPINE_VERSION:-3.21}"
ALPINE_MINOR="${ALPINE_MINOR:-3.21.3}"
ARCH="x86_64"
ALPINE_URL="https://dl-cdn.alpinelinux.org/alpine/v${ALPINE_VERSION}/releases/${ARCH}/alpine-minirootfs-${ALPINE_MINOR}-${ARCH}.tar.gz"

# Validate runtime
case "${RUNTIME}" in
    go|node|python) ;;
    *)
        echo "[ERROR] Unsupported runtime: ${RUNTIME}"
        echo "Supported: go, node, python"
        exit 1
        ;;
esac

# Check prerequisites
if [[ ! -f "${GUEST_BIN}" ]]; then
    echo "[ERROR] vulcan-guest binary not found at ${GUEST_BIN}"
    echo "Run 'make guest' first."
    exit 1
fi

IMAGE_PATH="${IMAGES_DIR}/${RUNTIME}.ext4"
MOUNT_POINT=$(mktemp -d)

cleanup() {
    sudo umount "${MOUNT_POINT}/proc" 2>/dev/null || true
    sudo umount "${MOUNT_POINT}/sys" 2>/dev/null || true
    sudo umount "${MOUNT_POINT}/dev" 2>/dev/null || true
    sudo umount "${MOUNT_POINT}" 2>/dev/null || true
    rmdir "${MOUNT_POINT}" 2>/dev/null || true
    # Remove partial image on failure
    if [[ "${BUILD_SUCCESS:-false}" != "true" ]]; then
        rm -f "${IMAGE_PATH}"
    fi
}
trap cleanup EXIT

mkdir -p "${IMAGES_DIR}"

echo "[BUILD] Creating ${RUNTIME}.ext4 (${IMAGE_SIZE_MB}MB)..."

# Create empty ext4 image
dd if=/dev/zero of="${IMAGE_PATH}" bs=1M count="${IMAGE_SIZE_MB}" status=none
mkfs.ext4 -F -q "${IMAGE_PATH}"

# Mount and populate
sudo mount -o loop "${IMAGE_PATH}" "${MOUNT_POINT}"

echo "[BOOTSTRAP] Installing Alpine Linux minirootfs..."
ALPINE_TMPDIR=$(mktemp -d)
curl -fsSL -o "${ALPINE_TMPDIR}/alpine-minirootfs.tar.gz" "${ALPINE_URL}"
sudo tar xzf "${ALPINE_TMPDIR}/alpine-minirootfs.tar.gz" -C "${MOUNT_POINT}"
rm -rf "${ALPINE_TMPDIR}"

# Configure Alpine repositories
sudo tee "${MOUNT_POINT}/etc/apk/repositories" > /dev/null <<EOF
https://dl-cdn.alpinelinux.org/alpine/v${ALPINE_VERSION}/main
https://dl-cdn.alpinelinux.org/alpine/v${ALPINE_VERSION}/community
EOF

# Copy resolv.conf for DNS resolution during chroot package installation
sudo cp /etc/resolv.conf "${MOUNT_POINT}/etc/resolv.conf"

# Bind-mount /proc, /sys, /dev for chroot package installation
sudo mount --bind /proc "${MOUNT_POINT}/proc"
sudo mount --bind /sys "${MOUNT_POINT}/sys"
sudo mount --bind /dev "${MOUNT_POINT}/dev"

# Install runtime-specific packages
echo "[INSTALL] Installing ${RUNTIME} runtime..."
case "${RUNTIME}" in
    go)
        sudo chroot "${MOUNT_POINT}" /sbin/apk add --no-cache go musl-dev
        ;;
    node)
        sudo chroot "${MOUNT_POINT}" /sbin/apk add --no-cache nodejs
        ;;
    python)
        sudo chroot "${MOUNT_POINT}" /sbin/apk add --no-cache python3
        ;;
esac

# Unmount chroot bind mounts
sudo umount "${MOUNT_POINT}/proc"
sudo umount "${MOUNT_POINT}/sys"
sudo umount "${MOUNT_POINT}/dev"

# Copy guest agent binary
echo "[INSTALL] Installing vulcan-guest agent..."
sudo cp "${GUEST_BIN}" "${MOUNT_POINT}/usr/local/bin/vulcan-guest"
sudo chmod +x "${MOUNT_POINT}/usr/local/bin/vulcan-guest"

# Create work directory
sudo mkdir -p "${MOUNT_POINT}/work"

# Create init script that starts vulcan-guest as the init process
sudo tee "${MOUNT_POINT}/init" > /dev/null <<'INITEOF'
#!/bin/sh
exec /usr/local/bin/vulcan-guest
INITEOF
sudo chmod +x "${MOUNT_POINT}/init"

# Symlink /sbin/init to our init
sudo ln -sf /init "${MOUNT_POINT}/sbin/init"

# Unmount
sudo umount "${MOUNT_POINT}"
rmdir "${MOUNT_POINT}"
BUILD_SUCCESS=true
trap - EXIT

echo "[OK] ${RUNTIME}.ext4 created at ${IMAGE_PATH}"
echo "     Size: $(du -h "${IMAGE_PATH}" | awk '{print $1}')"
