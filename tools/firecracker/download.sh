#!/usr/bin/env bash
#
# Download Firecracker binary and compatible kernel.
# Idempotent — skips download if correct version is already present.
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN_DIR="${SCRIPT_DIR}/bin"

# Version pins
FIRECRACKER_VERSION="${FIRECRACKER_VERSION:-1.12.0}"
KERNEL_VERSION="${KERNEL_VERSION:-5.10}"
# The kernel CI bucket path matches the Firecracker major.minor version.
# This must be updated when FIRECRACKER_VERSION changes to a new major.minor.
FC_KERNEL_COMPAT="${FC_KERNEL_COMPAT:-${FIRECRACKER_VERSION%.*}}"
ARCH="x86_64"

# URLs
FC_BASE_URL="https://github.com/firecracker-microvm/firecracker/releases/download/v${FIRECRACKER_VERSION}"
FC_ARCHIVE="firecracker-v${FIRECRACKER_VERSION}-${ARCH}.tgz"
FC_URL="${FC_BASE_URL}/${FC_ARCHIVE}"
FC_SHA_URL="${FC_BASE_URL}/SHA256SUMS"

# Kernel URL uses FC_KERNEL_COMPAT for the bucket path
# List available kernels matching the major version and pick the latest patch
KERNEL_LISTING_URL="https://s3.amazonaws.com/spec.ccfc.min?prefix=firecracker-ci/v${FC_KERNEL_COMPAT}/${ARCH}/vmlinux-${KERNEL_VERSION}.&delimiter=/"
KERNEL_KEY=$(curl -fsSL "${KERNEL_LISTING_URL}" | grep -oP "<Key>firecracker-ci/v${FC_KERNEL_COMPAT}/${ARCH}/vmlinux-${KERNEL_VERSION}\.[0-9]+</Key>" | tail -1 | sed 's/<[^>]*>//g')

if [[ -z "${KERNEL_KEY}" ]]; then
    echo "[ERROR] Could not find kernel matching vmlinux-${KERNEL_VERSION}.* in S3 bucket"
    exit 1
fi

KERNEL_FULL_VERSION=$(basename "${KERNEL_KEY}" | sed 's/vmlinux-//')
KERNEL_URL="https://s3.amazonaws.com/spec.ccfc.min/${KERNEL_KEY}"
echo "[INFO] Resolved kernel: vmlinux-${KERNEL_FULL_VERSION}"

# Target paths
FC_BIN="${BIN_DIR}/firecracker"
KERNEL_BIN="${BIN_DIR}/vmlinux"
KERNEL_VERSION_FILE="${BIN_DIR}/vmlinux.version"

mkdir -p "${BIN_DIR}"

# --- Firecracker binary ---
if [[ -x "${FC_BIN}" ]]; then
    CURRENT_VERSION=$("${FC_BIN}" --version 2>/dev/null | head -1 | grep -oP '\d+\.\d+\.\d+' || echo "unknown")
    if [[ "${CURRENT_VERSION}" == "${FIRECRACKER_VERSION}" ]]; then
        echo "[OK] Firecracker v${FIRECRACKER_VERSION} already present"
    else
        echo "[UPDATE] Firecracker version mismatch (${CURRENT_VERSION}), re-downloading..."
        rm -f "${FC_BIN}"
    fi
fi

if [[ ! -x "${FC_BIN}" ]]; then
    echo "[DOWNLOAD] Firecracker v${FIRECRACKER_VERSION}..."
    FC_TMPDIR=$(mktemp -d)
    trap 'rm -rf "${FC_TMPDIR}"' EXIT

    curl -fsSL -o "${FC_TMPDIR}/${FC_ARCHIVE}" "${FC_URL}"

    # Verify checksum if available
    if curl -fsSL -o "${FC_TMPDIR}/SHA256SUMS" "${FC_SHA_URL}" 2>/dev/null; then
        EXPECTED_SHA=$(grep "${FC_ARCHIVE}" "${FC_TMPDIR}/SHA256SUMS" | awk '{print $1}')
        ACTUAL_SHA=$(sha256sum "${FC_TMPDIR}/${FC_ARCHIVE}" | awk '{print $1}')
        if [[ "${EXPECTED_SHA}" != "${ACTUAL_SHA}" ]]; then
            echo "[ERROR] Checksum mismatch for ${FC_ARCHIVE}"
            echo "  Expected: ${EXPECTED_SHA}"
            echo "  Actual:   ${ACTUAL_SHA}"
            exit 1
        fi
        echo "[OK] Checksum verified"
    else
        echo "[WARN] SHA256SUMS not available, skipping checksum verification"
    fi

    # Extract the Firecracker binary from the archive
    tar xzf "${FC_TMPDIR}/${FC_ARCHIVE}" -C "${FC_TMPDIR}"
    FC_EXTRACTED=$(find "${FC_TMPDIR}" -name "firecracker-v${FIRECRACKER_VERSION}-${ARCH}" -type f | head -1)
    if [[ -z "${FC_EXTRACTED}" ]]; then
        echo "[ERROR] Could not find firecracker binary in archive"
        exit 1
    fi
    cp "${FC_EXTRACTED}" "${FC_BIN}"
    chmod +x "${FC_BIN}"
    echo "[OK] Firecracker v${FIRECRACKER_VERSION} installed to ${FC_BIN}"

    rm -rf "${FC_TMPDIR}"
    trap - EXIT
fi

# --- Kernel ---
NEED_KERNEL=false
if [[ ! -f "${KERNEL_BIN}" ]]; then
    NEED_KERNEL=true
elif [[ -f "${KERNEL_VERSION_FILE}" ]]; then
    CURRENT_KERNEL_VERSION=$(cat "${KERNEL_VERSION_FILE}")
    if [[ "${CURRENT_KERNEL_VERSION}" != "${KERNEL_VERSION}" ]]; then
        echo "[UPDATE] Kernel version mismatch (${CURRENT_KERNEL_VERSION}), re-downloading..."
        NEED_KERNEL=true
    fi
else
    # No version file — assume stale
    NEED_KERNEL=true
fi

if [[ "${NEED_KERNEL}" == "true" ]]; then
    echo "[DOWNLOAD] Kernel (${KERNEL_VERSION})..."
    curl -fsSL -o "${KERNEL_BIN}" "${KERNEL_URL}"
    chmod +r "${KERNEL_BIN}"
    echo "${KERNEL_VERSION}" > "${KERNEL_VERSION_FILE}"
    echo "[OK] Kernel installed to ${KERNEL_BIN}"
else
    echo "[OK] Kernel ${KERNEL_VERSION} already present at ${KERNEL_BIN}"
fi

echo ""
echo "=== Firecracker Toolchain ==="
echo "  Firecracker: ${FC_BIN} (v${FIRECRACKER_VERSION})"
echo "  Kernel:      ${KERNEL_BIN} (${KERNEL_VERSION})"
