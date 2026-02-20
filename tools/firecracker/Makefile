# Firecracker microVM Build Tooling
#
# Targets:
#   make download      - Download Firecracker binary and kernel
#   make guest         - Build vulcan-guest static binary
#   make rootfs-go     - Build Go runtime rootfs image
#   make rootfs-node   - Build Node runtime rootfs image
#   make rootfs-python - Build Python runtime rootfs image
#   make rootfs-all    - Build all rootfs images
#   make all           - Download + build guest + build all rootfs
#   make clean         - Remove built artifacts

# Version pins
FIRECRACKER_VERSION ?= 1.12.0
KERNEL_VERSION      ?= 5.10
ALPINE_VERSION      ?= 3.21
ALPINE_MINOR        ?= 3.21.3
ROOTFS_SIZE_MB      ?= 512

# Paths
SCRIPT_DIR   := $(shell pwd)
BIN_DIR      := $(SCRIPT_DIR)/bin
IMAGES_DIR   := $(SCRIPT_DIR)/images
API_DIR      := $(SCRIPT_DIR)/../../api
GUEST_BIN    := $(BIN_DIR)/vulcan-guest
FC_BIN       := $(BIN_DIR)/firecracker
KERNEL_BIN   := $(BIN_DIR)/vmlinux
DOWNLOAD_STAMP := $(BIN_DIR)/.downloaded

.PHONY: all download guest rootfs-all rootfs-go rootfs-node rootfs-python clean verify

all: download guest rootfs-all

# --- Download Firecracker binary and kernel ---
# Uses a stamp file to avoid re-running download.sh on every make invocation.
# The stamp file is touched after a successful download.
download: $(DOWNLOAD_STAMP)

$(DOWNLOAD_STAMP):
	@mkdir -p $(BIN_DIR)
	FIRECRACKER_VERSION=$(FIRECRACKER_VERSION) \
	KERNEL_VERSION=$(KERNEL_VERSION) \
	bash $(SCRIPT_DIR)/download.sh
	@touch $(DOWNLOAD_STAMP)

$(FC_BIN): $(DOWNLOAD_STAMP)
$(KERNEL_BIN): $(DOWNLOAD_STAMP)

# --- Build vulcan-guest static binary ---
guest: $(GUEST_BIN)

$(GUEST_BIN): $(shell find $(API_DIR)/cmd/vulcan-guest $(API_DIR)/internal/guest $(API_DIR)/internal/backend/firecracker -name '*.go' 2>/dev/null)
	@mkdir -p $(BIN_DIR)
	cd $(API_DIR) && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $(GUEST_BIN) ./cmd/vulcan-guest
	@echo "[OK] vulcan-guest built at $(GUEST_BIN)"

# --- Build rootfs images ---
rootfs-all: rootfs-go rootfs-node rootfs-python

rootfs-go: $(IMAGES_DIR)/go.ext4
rootfs-node: $(IMAGES_DIR)/node.ext4
rootfs-python: $(IMAGES_DIR)/python.ext4

$(IMAGES_DIR)/%.ext4: $(GUEST_BIN)
	ALPINE_VERSION=$(ALPINE_VERSION) ALPINE_MINOR=$(ALPINE_MINOR) \
	bash $(SCRIPT_DIR)/build-rootfs.sh $* $(ROOTFS_SIZE_MB)

# --- Verify built artifacts ---
verify:
	@echo "=== Verifying build artifacts ==="
	@echo ""
	@echo "--- Firecracker binary ---"
	@test -x $(FC_BIN) && echo "[OK] $(FC_BIN)" || echo "[MISSING] $(FC_BIN)"
	@echo ""
	@echo "--- Kernel ---"
	@test -f $(KERNEL_BIN) && echo "[OK] $(KERNEL_BIN)" || echo "[MISSING] $(KERNEL_BIN)"
	@echo ""
	@echo "--- Guest agent ---"
	@test -x $(GUEST_BIN) && echo "[OK] $(GUEST_BIN)" || echo "[MISSING] $(GUEST_BIN)"
	@test -x $(GUEST_BIN) && file $(GUEST_BIN) | grep -q "statically linked" && echo "    Static: yes" || echo "    Static: check manually"
	@echo ""
	@echo "--- Rootfs images ---"
	@for rt in go node python; do \
		if test -f $(IMAGES_DIR)/$$rt.ext4; then \
			echo "[OK] $$rt.ext4 ($$(du -h $(IMAGES_DIR)/$$rt.ext4 | awk '{print $$1}'))"; \
		else \
			echo "[MISSING] $$rt.ext4"; \
		fi; \
	done

# --- Clean ---
clean:
	rm -rf $(BIN_DIR) $(IMAGES_DIR)
	@echo "[OK] Cleaned build artifacts"
