package frpconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadAndMarshalCommonConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "frpc.toml")
	input := `serverAddr = "frps.example.com"
serverPort = 7000
user = "demo"
auth.method = "token"
auth.token = "secret"
transport.protocol = "tcp"
transport.wireProtocol = "v2"
transport.tls.enable = true

[[proxies]]
name = "ssh"
type = "tcp"
enabled = true
localIP = "127.0.0.1"
localPort = 22
remotePort = 6001
transport.useEncryption = true

[[proxies]]
name = "web"
type = "http"
localIP = "127.0.0.1"
localPort = 8080
customDomains = ["app.example.com"]
locations = ["/"]
requestHeaders.set.x-from-frp = "yes"

[[visitors]]
name = "private"
type = "xtcp"
serverName = "private-service"
secretKey = "abcdef"
bindAddr = "127.0.0.1"
bindPort = 9000
keepTunnelOpen = true
`
	if err := os.WriteFile(path, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.ServerAddr != "frps.example.com" || len(cfg.Proxies) != 2 || len(cfg.Visitors) != 1 {
		t.Fatalf("unexpected config: %#v", cfg)
	}
	if got := cfg.Proxies[1].RequestHeaders.Set["x-from-frp"]; got != "yes" {
		t.Fatalf("request header = %q", got)
	}

	output, err := Marshal(cfg)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	text := string(output)
	for _, expected := range []string{"frps.example.com", "ssh", "app.example.com", "private-service"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("generated TOML does not contain %q:\n%s", expected, text)
		}
	}
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "frpc.toml")
	input := "serverAddr = \"127.0.0.1\"\nserverPort = 7000\nfutureField = \"keep-me\"\n"
	if err := os.WriteFile(path, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("Load() expected error for unknown field")
	}
}

func TestValidateTypeSpecificRequirements(t *testing.T) {
	cfg := Default()
	cfg.Proxies = []ProxyConfig{{
		Name:      "web",
		Type:      "http",
		LocalIP:   "127.0.0.1",
		LocalPort: 8080,
	}}
	if err := Validate(cfg); err == nil {
		t.Fatal("Validate() expected HTTP domain error")
	}

	cfg.Proxies[0].CustomDomains = []string{"example.com"}
	if err := Validate(cfg); err != nil {
		t.Fatalf("Validate() unexpected error = %v", err)
	}
}
