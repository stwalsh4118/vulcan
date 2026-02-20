package firecracker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/containernetworking/cni/libcni"
	"github.com/containernetworking/cni/pkg/types"
	types100 "github.com/containernetworking/cni/pkg/types/100"
)

// Networking defaults for the Firecracker CNI bridge.
const (
	// DefaultBridgeName is the Linux bridge device for microVM networking.
	DefaultBridgeName = "fcbr0"

	// DefaultSubnet is the CIDR subnet for microVM IP allocation.
	DefaultSubnet = "10.168.0.0/24"

	// DefaultGateway is the gateway IP address on the bridge.
	DefaultGateway = "10.168.0.1"

	// CNINetworkName is the CNI network name used in conflist.
	CNINetworkName = "vulcan-fcnet"

	// CNIVersion is the CNI spec version used in conflist.
	CNIVersion = "1.0.0"

	// CNIIfName is the interface name inside the network namespace.
	CNIIfName = "eth0"

	// CNICacheDir is the directory for CNI result caching.
	CNICacheDir = "/var/lib/cni/cache"

	// NetNSRunDir is the directory for network namespaces.
	NetNSRunDir = "/var/run/netns"

	// NetNSPrefix is the prefix for per-VM namespace names.
	NetNSPrefix = "vulcan-"
)

// Required CNI plugins for Firecracker networking.
var requiredCNIPlugins = []string{"bridge", "host-local", "tc-redirect-tap"}

// NetworkConfig holds the network configuration returned after CNI setup.
type NetworkConfig struct {
	// TAPDevice is the name of the TAP device created by tc-redirect-tap.
	TAPDevice string

	// GuestIP is the IP address assigned to the guest (CIDR notation).
	GuestIP string

	// GatewayIP is the gateway address for the guest.
	GatewayIP string

	// MACAddress is the MAC address of the guest interface.
	MACAddress string

	// NamespacePath is the full path to the network namespace.
	NamespacePath string
}

// ipForwardPath is the sysctl path to enable IPv4 forwarding.
const ipForwardPath = "/proc/sys/net/ipv4/ip_forward"

// NetworkManager handles CNI-based networking for Firecracker microVMs.
type NetworkManager struct {
	cniBinDir     string
	cniConfigDir  string
	cniConfig     *libcni.CNIConfig
	confList      *libcni.NetworkConfigList
	confListBytes []byte // cached conflist JSON for WriteConfList
	logger        *slog.Logger

	mu         sync.Mutex
	namespaces map[string]string // vmID → namespace path
}

// NewNetworkManager creates a NetworkManager with the given CNI configuration.
func NewNetworkManager(cfg Config, logger *slog.Logger) (*NetworkManager, error) {
	cniConfig := libcni.NewCNIConfigWithCacheDir(
		[]string{cfg.CNIBinDir},
		CNICacheDir,
		nil,
	)

	confBytes, err := generateConfList()
	if err != nil {
		return nil, fmt.Errorf("generate CNI conflist: %w", err)
	}

	confList, err := libcni.ConfListFromBytes(confBytes)
	if err != nil {
		return nil, fmt.Errorf("parse CNI conflist: %w", err)
	}

	return &NetworkManager{
		cniBinDir:     cfg.CNIBinDir,
		cniConfigDir:  cfg.CNIConfigDir,
		cniConfig:     cniConfig,
		confList:      confList,
		confListBytes: confBytes,
		logger:        logger,
		namespaces:    make(map[string]string),
	}, nil
}

// Setup creates a network namespace and configures networking for a microVM.
// Returns the network configuration including the TAP device name and guest IP.
func (nm *NetworkManager) Setup(ctx context.Context, vmID string) (*NetworkConfig, error) {
	nsName := NetNSPrefix + vmID
	nsPath := filepath.Join(NetNSRunDir, nsName)

	// Create the network namespace.
	if err := createNetNS(nsName); err != nil {
		return nil, fmt.Errorf("create netns %s: %w", nsName, err)
	}

	nm.mu.Lock()
	nm.namespaces[vmID] = nsPath
	nm.mu.Unlock()

	// Invoke CNI ADD.
	rtConf := &libcni.RuntimeConf{
		ContainerID: vmID,
		NetNS:       nsPath,
		IfName:      CNIIfName,
	}

	result, err := nm.cniConfig.AddNetworkList(ctx, nm.confList, rtConf)
	if err != nil {
		// Attempt cleanup on failure.
		cleanupErr := deleteNetNS(nsName)
		if cleanupErr != nil {
			nm.logger.Warn("failed to clean up netns after CNI ADD failure",
				"vmID", vmID, "cleanup_error", cleanupErr)
		}
		nm.mu.Lock()
		delete(nm.namespaces, vmID)
		nm.mu.Unlock()
		return nil, fmt.Errorf("CNI ADD for %s: %w", vmID, err)
	}

	// Parse the CNI result.
	netCfg, err := parseResult(result, nsPath)
	if err != nil {
		// Attempt full teardown on parse failure.
		if delErr := nm.cniConfig.DelNetworkList(ctx, nm.confList, rtConf); delErr != nil {
			nm.logger.Debug("cleanup CNI DEL after parse failure", "vmID", vmID, "error", delErr)
		}
		if nsErr := deleteNetNS(nsName); nsErr != nil {
			nm.logger.Debug("cleanup netns after parse failure", "vmID", vmID, "error", nsErr)
		}
		nm.mu.Lock()
		delete(nm.namespaces, vmID)
		nm.mu.Unlock()
		return nil, fmt.Errorf("parse CNI result for %s: %w", vmID, err)
	}

	nm.logger.Info("network setup complete",
		"vmID", vmID,
		"tap", netCfg.TAPDevice,
		"guest_ip", netCfg.GuestIP,
		"namespace", nsPath,
	)

	return netCfg, nil
}

// Teardown removes networking and the network namespace for a microVM.
// Safe to call multiple times — subsequent calls are no-ops.
func (nm *NetworkManager) Teardown(ctx context.Context, vmID string) error {
	nm.mu.Lock()
	nsPath, exists := nm.namespaces[vmID]
	if !exists {
		nm.mu.Unlock()
		return nil // Already torn down or never set up.
	}
	delete(nm.namespaces, vmID)
	nm.mu.Unlock()

	nsName := NetNSPrefix + vmID

	// Invoke CNI DEL.
	rtConf := &libcni.RuntimeConf{
		ContainerID: vmID,
		NetNS:       nsPath,
		IfName:      CNIIfName,
	}

	var firstErr error
	if err := nm.cniConfig.DelNetworkList(ctx, nm.confList, rtConf); err != nil {
		firstErr = fmt.Errorf("CNI DEL for %s: %w", vmID, err)
		nm.logger.Warn("CNI DEL failed", "vmID", vmID, "error", err)
	}

	// Remove the network namespace.
	if err := deleteNetNS(nsName); err != nil {
		nm.logger.Warn("netns cleanup failed", "vmID", vmID, "error", err)
		if firstErr == nil {
			firstErr = fmt.Errorf("delete netns for %s: %w", vmID, err)
		}
	}

	if firstErr == nil {
		nm.logger.Info("network teardown complete", "vmID", vmID)
	}

	return firstErr
}

// TeardownAll cleans up all tracked namespaces. Used during server shutdown.
func (nm *NetworkManager) TeardownAll(ctx context.Context) {
	nm.mu.Lock()
	vmIDs := make([]string, 0, len(nm.namespaces))
	for vmID := range nm.namespaces {
		vmIDs = append(vmIDs, vmID)
	}
	nm.mu.Unlock()

	for _, vmID := range vmIDs {
		if err := nm.Teardown(ctx, vmID); err != nil {
			nm.logger.Error("teardown failed during shutdown", "vmID", vmID, "error", err)
		}
	}
}

// Verify checks that all required CNI plugins exist in the bin directory.
func (nm *NetworkManager) Verify() error {
	var missing []string
	for _, plugin := range requiredCNIPlugins {
		pluginPath := filepath.Join(nm.cniBinDir, plugin)
		_, err := os.Stat(pluginPath)
		if err == nil {
			continue
		}
		if errors.Is(err, os.ErrNotExist) {
			missing = append(missing, plugin)
		} else {
			return fmt.Errorf("stat CNI plugin %s: %w", plugin, err)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing CNI plugins in %s: %s", nm.cniBinDir, strings.Join(missing, ", "))
	}
	return nil
}

// WriteConfList writes the CNI conflist to the config directory.
func (nm *NetworkManager) WriteConfList() error {
	if err := os.MkdirAll(nm.cniConfigDir, 0o755); err != nil {
		return fmt.Errorf("create CNI config dir: %w", err)
	}

	confPath := filepath.Join(nm.cniConfigDir, CNINetworkName+".conflist")
	if err := os.WriteFile(confPath, nm.confListBytes, 0o644); err != nil {
		return fmt.Errorf("write conflist: %w", err)
	}

	nm.logger.Info("wrote CNI conflist", "path", confPath)
	return nil
}

// confList is the structure for generating the CNI conflist JSON.
type confListJSON struct {
	CNIVersion string           `json:"cniVersion"`
	Name       string           `json:"name"`
	Plugins    []map[string]any `json:"plugins"`
}

// generateConfList returns the CNI conflist JSON for bridge + tc-redirect-tap.
func generateConfList() ([]byte, error) {
	confList := confListJSON{
		CNIVersion: CNIVersion,
		Name:       CNINetworkName,
		Plugins: []map[string]any{
			{
				"type":      "bridge",
				"bridge":    DefaultBridgeName,
				"isGateway": true,
				"ipMasq":    true,
				"ipam": map[string]any{
					"type":    "host-local",
					"subnet":  DefaultSubnet,
					"gateway": DefaultGateway,
				},
			},
			{
				"type": "tc-redirect-tap",
			},
		},
	}

	data, err := json.MarshalIndent(confList, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal conflist: %w", err)
	}
	return data, nil
}

// parseResult extracts NetworkConfig from a CNI ADD result.
func parseResult(result types.Result, nsPath string) (*NetworkConfig, error) {
	res, err := types100.NewResultFromResult(result)
	if err != nil {
		return nil, fmt.Errorf("convert CNI result: %w", err)
	}

	netCfg := &NetworkConfig{
		NamespacePath: nsPath,
	}

	// Extract the TAP device name — tc-redirect-tap creates a TAP interface
	// in the sandbox (namespace) alongside the veth (CNIIfName / "eth0").
	// We need the TAP, not the veth, so skip the interface matching CNIIfName.
	for _, iface := range res.Interfaces {
		if iface.Sandbox != "" && iface.Name != CNIIfName {
			netCfg.TAPDevice = iface.Name
			netCfg.MACAddress = iface.Mac
			break
		}
	}

	// Fallback: if tc-redirect-tap didn't add a separate interface,
	// use the first sandboxed interface and its MAC.
	if netCfg.TAPDevice == "" {
		for _, iface := range res.Interfaces {
			if iface.Sandbox != "" {
				netCfg.TAPDevice = iface.Name
				netCfg.MACAddress = iface.Mac
				break
			}
		}
	}

	if netCfg.TAPDevice == "" {
		return nil, fmt.Errorf("no TAP device in CNI result (no interface with sandbox set)")
	}

	// Extract the first assigned IP.
	if len(res.IPs) > 0 {
		netCfg.GuestIP = res.IPs[0].Address.String()
		if res.IPs[0].Gateway != nil {
			netCfg.GatewayIP = res.IPs[0].Gateway.String()
		}
	}

	if netCfg.GuestIP == "" {
		return nil, fmt.Errorf("no IP address in CNI result")
	}

	return netCfg, nil
}

// createNetNS creates a named network namespace using ip netns add.
func createNetNS(name string) error {
	if err := os.MkdirAll(NetNSRunDir, 0o755); err != nil {
		return fmt.Errorf("create netns dir: %w", err)
	}
	cmd := exec.Command("ip", "netns", "add", name)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ip netns add %s: %s: %w", name, strings.TrimSpace(string(output)), err)
	}
	return nil
}

// deleteNetNS removes a named network namespace.
// Returns nil if the namespace does not exist (idempotent).
func deleteNetNS(name string) error {
	nsPath := filepath.Join(NetNSRunDir, name)
	_, err := os.Stat(nsPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil // Already removed.
		}
		return fmt.Errorf("stat netns %s: %w", name, err)
	}
	cmd := exec.Command("ip", "netns", "delete", name)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ip netns delete %s: %s: %w", name, strings.TrimSpace(string(output)), err)
	}
	return nil
}

// EnsureIPForwarding enables IPv4 forwarding on the host.
// This is required for outbound NAT from the bridge subnet.
// Idempotent: reads the current value and only writes if not already enabled.
func EnsureIPForwarding() error {
	data, err := os.ReadFile(ipForwardPath)
	if err != nil {
		return fmt.Errorf("read ip_forward: %w", err)
	}
	if strings.TrimSpace(string(data)) == "1" {
		return nil // Already enabled.
	}
	if err := os.WriteFile(ipForwardPath, []byte("1"), 0o644); err != nil {
		return fmt.Errorf("enable ip_forward: %w", err)
	}
	return nil
}

// GenerateMAC creates a locally-administered MAC address from the VM ID.
// Uses the VM ID hash to generate a deterministic but unique MAC.
func GenerateMAC(vmID string) net.HardwareAddr {
	// Start with locally administered, unicast prefix.
	mac := make(net.HardwareAddr, 6)
	mac[0] = 0x02 // Locally administered, unicast.

	// Use simple hash of vmID for remaining bytes.
	hash := uint32(0)
	for _, b := range []byte(vmID) {
		hash = hash*31 + uint32(b)
	}
	mac[1] = byte(hash >> 24)
	mac[2] = byte(hash >> 16)
	mac[3] = byte(hash >> 8)
	mac[4] = byte(hash)
	mac[5] = byte(hash >> 12)

	return mac
}
