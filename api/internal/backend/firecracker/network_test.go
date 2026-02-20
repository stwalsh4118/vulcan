package firecracker

import (
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	types100 "github.com/containernetworking/cni/pkg/types/100"
)

func TestGenerateConfList(t *testing.T) {
	data, err := generateConfList()
	if err != nil {
		t.Fatalf("generateConfList: %v", err)
	}

	// Verify it's valid JSON.
	var parsed confListJSON
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal conflist: %v", err)
	}

	if parsed.CNIVersion != CNIVersion {
		t.Errorf("cniVersion = %q, want %q", parsed.CNIVersion, CNIVersion)
	}
	if parsed.Name != CNINetworkName {
		t.Errorf("name = %q, want %q", parsed.Name, CNINetworkName)
	}
	if len(parsed.Plugins) != 2 {
		t.Fatalf("plugins count = %d, want 2", len(parsed.Plugins))
	}

	// First plugin: bridge.
	bridge := parsed.Plugins[0]
	if bridge["type"] != "bridge" {
		t.Errorf("plugin[0].type = %q, want %q", bridge["type"], "bridge")
	}
	if bridge["bridge"] != DefaultBridgeName {
		t.Errorf("plugin[0].bridge = %q, want %q", bridge["bridge"], DefaultBridgeName)
	}
	if bridge["isGateway"] != true {
		t.Error("plugin[0].isGateway should be true")
	}
	if bridge["ipMasq"] != true {
		t.Error("plugin[0].ipMasq should be true")
	}

	ipam, ok := bridge["ipam"].(map[string]any)
	if !ok {
		t.Fatal("plugin[0].ipam is not a map")
	}
	if ipam["type"] != "host-local" {
		t.Errorf("ipam.type = %q, want %q", ipam["type"], "host-local")
	}
	if ipam["subnet"] != DefaultSubnet {
		t.Errorf("ipam.subnet = %q, want %q", ipam["subnet"], DefaultSubnet)
	}
	if ipam["gateway"] != DefaultGateway {
		t.Errorf("ipam.gateway = %q, want %q", ipam["gateway"], DefaultGateway)
	}

	// Second plugin: tc-redirect-tap.
	tap := parsed.Plugins[1]
	if tap["type"] != "tc-redirect-tap" {
		t.Errorf("plugin[1].type = %q, want %q", tap["type"], "tc-redirect-tap")
	}
}

func TestGenerateConfListIsValidCNI(t *testing.T) {
	data, err := generateConfList()
	if err != nil {
		t.Fatalf("generateConfList: %v", err)
	}

	// Verify the JSON has the required top-level fields for a CNI conflist.
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	for _, key := range []string{"cniVersion", "name", "plugins"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("missing required field %q in conflist", key)
		}
	}

	// plugins must be an array.
	plugins, ok := raw["plugins"].([]any)
	if !ok {
		t.Fatal("plugins is not an array")
	}
	if len(plugins) == 0 {
		t.Fatal("plugins array is empty")
	}

	// Each plugin must have a "type" field.
	for i, p := range plugins {
		plugin, ok := p.(map[string]any)
		if !ok {
			t.Fatalf("plugin[%d] is not a map", i)
		}
		if _, ok := plugin["type"]; !ok {
			t.Errorf("plugin[%d] missing 'type' field", i)
		}
	}
}

func TestVerifyPluginsPresent(t *testing.T) {
	tmpDir := t.TempDir()

	// Create fake plugin binaries.
	for _, name := range requiredCNIPlugins {
		path := filepath.Join(tmpDir, name)
		if err := os.WriteFile(path, []byte("fake"), 0o755); err != nil {
			t.Fatalf("create fake plugin %s: %v", name, err)
		}
	}

	nm := &NetworkManager{cniBinDir: tmpDir}
	if err := nm.Verify(); err != nil {
		t.Errorf("Verify with all plugins present: %v", err)
	}
}

func TestVerifyPluginsMissing(t *testing.T) {
	tmpDir := t.TempDir()

	// Create only bridge plugin, omit the others.
	if err := os.WriteFile(filepath.Join(tmpDir, "bridge"), []byte("fake"), 0o755); err != nil {
		t.Fatalf("create bridge: %v", err)
	}

	nm := &NetworkManager{cniBinDir: tmpDir}
	err := nm.Verify()
	if err == nil {
		t.Fatal("expected error when plugins are missing")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "host-local") {
		t.Errorf("error should mention 'host-local': %s", errStr)
	}
	if !strings.Contains(errStr, "tc-redirect-tap") {
		t.Errorf("error should mention 'tc-redirect-tap': %s", errStr)
	}
}

func TestVerifyNoPlugins(t *testing.T) {
	tmpDir := t.TempDir()

	nm := &NetworkManager{cniBinDir: tmpDir}
	err := nm.Verify()
	if err == nil {
		t.Fatal("expected error when no plugins exist")
	}

	for _, plugin := range requiredCNIPlugins {
		if !strings.Contains(err.Error(), plugin) {
			t.Errorf("error should mention %q: %s", plugin, err.Error())
		}
	}
}

func TestWriteConfList(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "cni-conf")

	confBytes, _ := generateConfList()
	nm := &NetworkManager{
		cniConfigDir:  configDir,
		confListBytes: confBytes,
		logger:        testLogger(),
	}

	if err := nm.WriteConfList(); err != nil {
		t.Fatalf("WriteConfList: %v", err)
	}

	// Verify file was written.
	confPath := filepath.Join(configDir, CNINetworkName+".conflist")
	data, err := os.ReadFile(confPath)
	if err != nil {
		t.Fatalf("read conflist: %v", err)
	}

	// Verify it parses as valid JSON.
	var parsed confListJSON
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal written conflist: %v", err)
	}
	if parsed.Name != CNINetworkName {
		t.Errorf("name = %q, want %q", parsed.Name, CNINetworkName)
	}
}

func TestWriteConfListIdempotent(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "cni-conf")

	confBytes, _ := generateConfList()
	nm := &NetworkManager{
		cniConfigDir:  configDir,
		confListBytes: confBytes,
		logger:        testLogger(),
	}

	// Write twice — second call should overwrite without error.
	if err := nm.WriteConfList(); err != nil {
		t.Fatalf("first WriteConfList: %v", err)
	}
	if err := nm.WriteConfList(); err != nil {
		t.Fatalf("second WriteConfList: %v", err)
	}
}

func TestTeardownIdempotent(t *testing.T) {
	nm := &NetworkManager{
		namespaces: make(map[string]string),
		logger:     testLogger(),
	}

	// Teardown for a VM that was never set up should be a no-op.
	err := nm.Teardown(t.Context(), "nonexistent-vm")
	if err != nil {
		t.Errorf("Teardown of nonexistent VM should return nil, got: %v", err)
	}
}

func TestGenerateMAC(t *testing.T) {
	mac := GenerateMAC("test-vm-1")

	// Must be 6 bytes.
	if len(mac) != 6 {
		t.Fatalf("MAC length = %d, want 6", len(mac))
	}

	// First byte must be locally administered, unicast (0x02).
	if mac[0] != 0x02 {
		t.Errorf("first byte = 0x%02x, want 0x02", mac[0])
	}

	// Must be a valid hardware address.
	parsed, err := net.ParseMAC(mac.String())
	if err != nil {
		t.Fatalf("invalid MAC %s: %v", mac.String(), err)
	}
	if len(parsed) != 6 {
		t.Errorf("parsed MAC length = %d, want 6", len(parsed))
	}
}

func TestGenerateMACDeterministic(t *testing.T) {
	mac1 := GenerateMAC("vm-abc")
	mac2 := GenerateMAC("vm-abc")

	if mac1.String() != mac2.String() {
		t.Errorf("same input produced different MACs: %s vs %s", mac1, mac2)
	}
}

func TestGenerateMACUnique(t *testing.T) {
	mac1 := GenerateMAC("vm-1")
	mac2 := GenerateMAC("vm-2")

	if mac1.String() == mac2.String() {
		t.Errorf("different inputs produced same MAC: %s", mac1)
	}
}

func TestDeleteNetNSIdempotent(t *testing.T) {
	// Deleting a namespace that doesn't exist should be a no-op.
	err := deleteNetNS("vulcan-nonexistent-test-ns")
	if err != nil {
		t.Errorf("deleteNetNS of nonexistent namespace should return nil, got: %v", err)
	}
}

func TestRequiredCNIPlugins(t *testing.T) {
	// Verify the list contains the expected plugins.
	expected := map[string]bool{
		"bridge":          false,
		"host-local":      false,
		"tc-redirect-tap": false,
	}

	for _, p := range requiredCNIPlugins {
		if _, ok := expected[p]; ok {
			expected[p] = true
		} else {
			t.Errorf("unexpected plugin in requiredCNIPlugins: %q", p)
		}
	}

	for name, found := range expected {
		if !found {
			t.Errorf("missing expected plugin: %q", name)
		}
	}
}

func TestParseResultValid(t *testing.T) {
	result := &types100.Result{
		CNIVersion: "1.0.0",
		Interfaces: []*types100.Interface{
			{Name: "eth0", Mac: "02:ab:cd:ef:01:23", Sandbox: "/var/run/netns/vulcan-test"},
		},
		IPs: []*types100.IPConfig{
			{
				Address: mustParseCIDR("10.168.0.2/24"),
				Gateway: net.ParseIP("10.168.0.1"),
			},
		},
	}

	cfg, err := parseResult(result, "/var/run/netns/vulcan-test")
	if err != nil {
		t.Fatalf("parseResult: %v", err)
	}

	if cfg.TAPDevice != "eth0" {
		t.Errorf("TAPDevice = %q, want %q", cfg.TAPDevice, "eth0")
	}
	if cfg.GuestIP != "10.168.0.2/24" {
		t.Errorf("GuestIP = %q, want %q", cfg.GuestIP, "10.168.0.2/24")
	}
	if cfg.GatewayIP != "10.168.0.1" {
		t.Errorf("GatewayIP = %q, want %q", cfg.GatewayIP, "10.168.0.1")
	}
	if cfg.MACAddress != "02:ab:cd:ef:01:23" {
		t.Errorf("MACAddress = %q, want %q", cfg.MACAddress, "02:ab:cd:ef:01:23")
	}
	if cfg.NamespacePath != "/var/run/netns/vulcan-test" {
		t.Errorf("NamespacePath = %q, want %q", cfg.NamespacePath, "/var/run/netns/vulcan-test")
	}
}

func TestParseResultNoIPs(t *testing.T) {
	result := &types100.Result{
		CNIVersion: "1.0.0",
		Interfaces: []*types100.Interface{
			{Name: "eth0", Mac: "02:ab:cd:ef:01:23", Sandbox: "/var/run/netns/vulcan-test"},
		},
		IPs: []*types100.IPConfig{}, // No IPs.
	}

	_, err := parseResult(result, "/var/run/netns/vulcan-test")
	if err == nil {
		t.Fatal("expected error for result with no IPs")
	}
	if !strings.Contains(err.Error(), "no IP address") {
		t.Errorf("error = %q, want to contain 'no IP address'", err.Error())
	}
}

func TestParseResultNoSandboxInterface(t *testing.T) {
	result := &types100.Result{
		CNIVersion: "1.0.0",
		Interfaces: []*types100.Interface{
			{Name: "veth123", Mac: "02:ab:cd:ef:01:23", Sandbox: ""}, // Host-side veth, no sandbox.
		},
		IPs: []*types100.IPConfig{
			{
				Address: mustParseCIDR("10.168.0.2/24"),
				Gateway: net.ParseIP("10.168.0.1"),
			},
		},
	}

	_, err := parseResult(result, "/var/run/netns/vulcan-test")
	if err == nil {
		t.Fatal("expected error for result with no TAP device")
	}
	if !strings.Contains(err.Error(), "no TAP device") {
		t.Errorf("error = %q, want to contain 'no TAP device'", err.Error())
	}
}

func TestParseResultNoGateway(t *testing.T) {
	result := &types100.Result{
		CNIVersion: "1.0.0",
		Interfaces: []*types100.Interface{
			{Name: "tap0", Mac: "02:11:22:33:44:55", Sandbox: "/var/run/netns/vulcan-test"},
		},
		IPs: []*types100.IPConfig{
			{
				Address: mustParseCIDR("10.168.0.5/24"),
				Gateway: nil, // No gateway.
			},
		},
	}

	cfg, err := parseResult(result, "/var/run/netns/vulcan-test")
	if err != nil {
		t.Fatalf("parseResult: %v", err)
	}

	if cfg.GatewayIP != "" {
		t.Errorf("GatewayIP = %q, want empty string", cfg.GatewayIP)
	}
	if cfg.GuestIP != "10.168.0.5/24" {
		t.Errorf("GuestIP = %q, want %q", cfg.GuestIP, "10.168.0.5/24")
	}
}

func TestEnsureIPForwardingPath(t *testing.T) {
	// Verify the ip_forward path constant is set correctly.
	if ipForwardPath != "/proc/sys/net/ipv4/ip_forward" {
		t.Errorf("ipForwardPath = %q, want /proc/sys/net/ipv4/ip_forward", ipForwardPath)
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func mustParseCIDR(s string) net.IPNet {
	ip, ipNet, err := net.ParseCIDR(s)
	if err != nil {
		panic(err)
	}
	// Preserve the host IP (ParseCIDR normalizes to network address).
	ipNet.IP = ip
	return *ipNet
}
