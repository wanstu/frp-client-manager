package frpconfig

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

type ClientConfig struct {
	ClientID          string            `json:"clientID" toml:"clientID,omitempty"`
	User              string            `json:"user" toml:"user,omitempty"`
	ServerAddr        string            `json:"serverAddr" toml:"serverAddr,omitempty"`
	ServerPort        int               `json:"serverPort" toml:"serverPort,omitempty"`
	NatHoleStunServer string            `json:"natHoleStunServer" toml:"natHoleStunServer,omitempty"`
	LoginFailExit     *bool             `json:"loginFailExit,omitempty" toml:"loginFailExit,omitempty"`
	Auth              AuthConfig        `json:"auth" toml:"auth,omitempty"`
	Log               LogConfig         `json:"log" toml:"log,omitempty"`
	WebServer         WebServerConfig   `json:"webServer" toml:"webServer,omitempty"`
	Transport         TransportConfig   `json:"transport" toml:"transport,omitempty"`
	DNSServer         string            `json:"dnsServer" toml:"dnsServer,omitempty"`
	Start             []string          `json:"start,omitempty" toml:"start,omitempty"`
	UDPPacketSize     int               `json:"udpPacketSize" toml:"udpPacketSize,omitempty"`
	FeatureGates      map[string]bool   `json:"featureGates,omitempty" toml:"featureGates,omitempty"`
	VirtualNet        VirtualNetConfig  `json:"virtualNet" toml:"virtualNet,omitempty"`
	Metadatas         map[string]string `json:"metadatas,omitempty" toml:"metadatas,omitempty"`
	Includes          []string          `json:"includes,omitempty" toml:"includes,omitempty"`
	Proxies           []ProxyConfig     `json:"proxies" toml:"proxies,omitempty"`
	Visitors          []VisitorConfig   `json:"visitors" toml:"visitors,omitempty"`
}

type AuthConfig struct {
	Method           string            `json:"method" toml:"method,omitempty"`
	AdditionalScopes []string          `json:"additionalScopes,omitempty" toml:"additionalScopes,omitempty"`
	Token            string            `json:"token" toml:"token,omitempty"`
	TokenSource      TokenSourceConfig `json:"tokenSource" toml:"tokenSource,omitempty"`
	OIDC             OIDCConfig        `json:"oidc" toml:"oidc,omitempty"`
}

type TokenSourceConfig struct {
	Type string          `json:"type" toml:"type,omitempty"`
	File TokenSourceFile `json:"file" toml:"file,omitempty"`
}

type TokenSourceFile struct {
	Path string `json:"path" toml:"path,omitempty"`
}

type OIDCConfig struct {
	ClientID                 string            `json:"clientID" toml:"clientID,omitempty"`
	ClientSecret             string            `json:"clientSecret" toml:"clientSecret,omitempty"`
	Audience                 string            `json:"audience" toml:"audience,omitempty"`
	Scope                    string            `json:"scope" toml:"scope,omitempty"`
	TokenEndpointURL         string            `json:"tokenEndpointURL" toml:"tokenEndpointURL,omitempty"`
	AdditionalEndpointParams map[string]string `json:"additionalEndpointParams,omitempty" toml:"additionalEndpointParams,omitempty"`
	TrustedCAFile            string            `json:"trustedCaFile" toml:"trustedCaFile,omitempty"`
	InsecureSkipVerify       bool              `json:"insecureSkipVerify" toml:"insecureSkipVerify,omitempty"`
	ProxyURL                 string            `json:"proxyURL" toml:"proxyURL,omitempty"`
}

type LogConfig struct {
	To                string `json:"to" toml:"to,omitempty"`
	Level             string `json:"level" toml:"level,omitempty"`
	MaxDays           int    `json:"maxDays" toml:"maxDays,omitempty"`
	DisablePrintColor bool   `json:"disablePrintColor" toml:"disablePrintColor,omitempty"`
}

type WebServerConfig struct {
	Addr        string `json:"addr" toml:"addr,omitempty"`
	Port        int    `json:"port" toml:"port,omitempty"`
	User        string `json:"user" toml:"user,omitempty"`
	Password    string `json:"password" toml:"password,omitempty"`
	AssetsDir   string `json:"assetsDir" toml:"assetsDir,omitempty"`
	PprofEnable bool   `json:"pprofEnable" toml:"pprofEnable,omitempty"`
}

type TransportConfig struct {
	DialServerTimeout       int        `json:"dialServerTimeout" toml:"dialServerTimeout,omitempty"`
	DialServerKeepalive     int        `json:"dialServerKeepalive" toml:"dialServerKeepalive,omitempty"`
	PoolCount               int        `json:"poolCount" toml:"poolCount,omitempty"`
	TCPMux                  *bool      `json:"tcpMux,omitempty" toml:"tcpMux,omitempty"`
	TCPMuxKeepaliveInterval int        `json:"tcpMuxKeepaliveInterval" toml:"tcpMuxKeepaliveInterval,omitempty"`
	Protocol                string     `json:"protocol" toml:"protocol,omitempty"`
	WireProtocol            string     `json:"wireProtocol" toml:"wireProtocol,omitempty"`
	ConnectServerLocalIP    string     `json:"connectServerLocalIP" toml:"connectServerLocalIP,omitempty"`
	ProxyURL                string     `json:"proxyURL" toml:"proxyURL,omitempty"`
	HeartbeatInterval       int        `json:"heartbeatInterval" toml:"heartbeatInterval,omitempty"`
	HeartbeatTimeout        int        `json:"heartbeatTimeout" toml:"heartbeatTimeout,omitempty"`
	QUIC                    QUICConfig `json:"quic" toml:"quic,omitempty"`
	TLS                     TLSConfig  `json:"tls" toml:"tls,omitempty"`
}

type QUICConfig struct {
	KeepalivePeriod    int `json:"keepalivePeriod" toml:"keepalivePeriod,omitempty"`
	MaxIdleTimeout     int `json:"maxIdleTimeout" toml:"maxIdleTimeout,omitempty"`
	MaxIncomingStreams int `json:"maxIncomingStreams" toml:"maxIncomingStreams,omitempty"`
}

type TLSConfig struct {
	Enable                    *bool  `json:"enable,omitempty" toml:"enable,omitempty"`
	CertFile                  string `json:"certFile" toml:"certFile,omitempty"`
	KeyFile                   string `json:"keyFile" toml:"keyFile,omitempty"`
	TrustedCAFile             string `json:"trustedCaFile" toml:"trustedCaFile,omitempty"`
	ServerName                string `json:"serverName" toml:"serverName,omitempty"`
	DisableCustomTLSFirstByte *bool  `json:"disableCustomTLSFirstByte,omitempty" toml:"disableCustomTLSFirstByte,omitempty"`
}

type VirtualNetConfig struct {
	Address string `json:"address" toml:"address,omitempty"`
}

type ProxyConfig struct {
	Name              string            `json:"name" toml:"name"`
	Type              string            `json:"type" toml:"type"`
	Enabled           *bool             `json:"enabled,omitempty" toml:"enabled,omitempty"`
	LocalIP           string            `json:"localIP" toml:"localIP,omitempty"`
	LocalPort         int               `json:"localPort" toml:"localPort,omitempty"`
	RemotePort        int               `json:"remotePort" toml:"remotePort,omitempty"`
	CustomDomains     []string          `json:"customDomains,omitempty" toml:"customDomains,omitempty"`
	Subdomain         string            `json:"subdomain" toml:"subdomain,omitempty"`
	Locations         []string          `json:"locations,omitempty" toml:"locations,omitempty"`
	HTTPUser          string            `json:"httpUser" toml:"httpUser,omitempty"`
	HTTPPassword      string            `json:"httpPassword" toml:"httpPassword,omitempty"`
	RouteByHTTPUser   string            `json:"routeByHTTPUser" toml:"routeByHTTPUser,omitempty"`
	HostHeaderRewrite string            `json:"hostHeaderRewrite" toml:"hostHeaderRewrite,omitempty"`
	Multiplexer       string            `json:"multiplexer" toml:"multiplexer,omitempty"`
	SecretKey         string            `json:"secretKey" toml:"secretKey,omitempty"`
	AllowUsers        []string          `json:"allowUsers,omitempty" toml:"allowUsers,omitempty"`
	Transport         ProxyTransport    `json:"transport" toml:"transport,omitempty"`
	LoadBalancer      LoadBalancer      `json:"loadBalancer" toml:"loadBalancer,omitempty"`
	HealthCheck       HealthCheck       `json:"healthCheck" toml:"healthCheck,omitempty"`
	RequestHeaders    HeaderOperations  `json:"requestHeaders" toml:"requestHeaders,omitempty"`
	ResponseHeaders   HeaderOperations  `json:"responseHeaders" toml:"responseHeaders,omitempty"`
	Metadatas         map[string]string `json:"metadatas,omitempty" toml:"metadatas,omitempty"`
	Annotations       map[string]string `json:"annotations,omitempty" toml:"annotations,omitempty"`
	NatTraversal      NatTraversal      `json:"natTraversal" toml:"natTraversal,omitempty"`
	Plugin            PluginConfig      `json:"plugin" toml:"plugin,omitempty"`
}

type ProxyTransport struct {
	UseEncryption        bool   `json:"useEncryption" toml:"useEncryption,omitempty"`
	UseCompression       bool   `json:"useCompression" toml:"useCompression,omitempty"`
	BandwidthLimit       string `json:"bandwidthLimit" toml:"bandwidthLimit,omitempty"`
	BandwidthLimitMode   string `json:"bandwidthLimitMode" toml:"bandwidthLimitMode,omitempty"`
	ProxyProtocolVersion string `json:"proxyProtocolVersion" toml:"proxyProtocolVersion,omitempty"`
}

type LoadBalancer struct {
	Group    string `json:"group" toml:"group,omitempty"`
	GroupKey string `json:"groupKey" toml:"groupKey,omitempty"`
}

type HealthCheck struct {
	Type            string       `json:"type" toml:"type,omitempty"`
	TimeoutSeconds  int          `json:"timeoutSeconds" toml:"timeoutSeconds,omitempty"`
	MaxFailed       int          `json:"maxFailed" toml:"maxFailed,omitempty"`
	IntervalSeconds int          `json:"intervalSeconds" toml:"intervalSeconds,omitempty"`
	Path            string       `json:"path" toml:"path,omitempty"`
	HTTPHeaders     []HTTPHeader `json:"httpHeaders,omitempty" toml:"httpHeaders,omitempty"`
}

type HTTPHeader struct {
	Name  string `json:"name" toml:"name"`
	Value string `json:"value" toml:"value"`
}

type HeaderOperations struct {
	Set map[string]string `json:"set,omitempty" toml:"set,omitempty"`
}

type NatTraversal struct {
	DisableAssistedAddrs bool `json:"disableAssistedAddrs" toml:"disableAssistedAddrs,omitempty"`
}

type PluginConfig struct {
	Type              string           `json:"type" toml:"type,omitempty"`
	UnixPath          string           `json:"unixPath" toml:"unixPath,omitempty"`
	HTTPUser          string           `json:"httpUser" toml:"httpUser,omitempty"`
	HTTPPassword      string           `json:"httpPassword" toml:"httpPassword,omitempty"`
	Username          string           `json:"username" toml:"username,omitempty"`
	Password          string           `json:"password" toml:"password,omitempty"`
	LocalPath         string           `json:"localPath" toml:"localPath,omitempty"`
	StripPrefix       string           `json:"stripPrefix" toml:"stripPrefix,omitempty"`
	LocalAddr         string           `json:"localAddr" toml:"localAddr,omitempty"`
	CrtPath           string           `json:"crtPath" toml:"crtPath,omitempty"`
	KeyPath           string           `json:"keyPath" toml:"keyPath,omitempty"`
	HostHeaderRewrite string           `json:"hostHeaderRewrite" toml:"hostHeaderRewrite,omitempty"`
	RequestHeaders    HeaderOperations `json:"requestHeaders" toml:"requestHeaders,omitempty"`
}

type VisitorConfig struct {
	Name              string           `json:"name" toml:"name"`
	Type              string           `json:"type" toml:"type"`
	Enabled           *bool            `json:"enabled,omitempty" toml:"enabled,omitempty"`
	ServerUser        string           `json:"serverUser" toml:"serverUser,omitempty"`
	ServerName        string           `json:"serverName" toml:"serverName,omitempty"`
	SecretKey         string           `json:"secretKey" toml:"secretKey,omitempty"`
	BindAddr          string           `json:"bindAddr" toml:"bindAddr,omitempty"`
	BindPort          int              `json:"bindPort" toml:"bindPort,omitempty"`
	KeepTunnelOpen    bool             `json:"keepTunnelOpen" toml:"keepTunnelOpen,omitempty"`
	MaxRetriesAnHour  int              `json:"maxRetriesAnHour" toml:"maxRetriesAnHour,omitempty"`
	MinRetryInterval  int              `json:"minRetryInterval" toml:"minRetryInterval,omitempty"`
	FallbackTo        string           `json:"fallbackTo" toml:"fallbackTo,omitempty"`
	FallbackTimeoutMs int              `json:"fallbackTimeoutMs" toml:"fallbackTimeoutMs,omitempty"`
	Transport         VisitorTransport `json:"transport" toml:"transport,omitempty"`
	NatTraversal      NatTraversal     `json:"natTraversal" toml:"natTraversal,omitempty"`
	Plugin            VisitorPlugin    `json:"plugin" toml:"plugin,omitempty"`
}

type VisitorTransport struct {
	UseEncryption  bool `json:"useEncryption" toml:"useEncryption,omitempty"`
	UseCompression bool `json:"useCompression" toml:"useCompression,omitempty"`
}

type VisitorPlugin struct {
	Type          string `json:"type" toml:"type,omitempty"`
	DestinationIP string `json:"destinationIP" toml:"destinationIP,omitempty"`
}

func Default() ClientConfig {
	t := true
	return ClientConfig{
		ServerAddr:    "127.0.0.1",
		ServerPort:    7000,
		LoginFailExit: &t,
		Auth:          AuthConfig{Method: "token"},
		Log:           LogConfig{Level: "info", MaxDays: 3},
		Transport: TransportConfig{
			Protocol: "tcp",
			TLS:      TLSConfig{Enable: &t},
		},
		UDPPacketSize: 1500,
	}
}

func Load(path string) (ClientConfig, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return ClientConfig{}, errors.New("配置文件路径为空")
	}
	ext := strings.ToLower(filepath.Ext(path))
	if ext != "" && ext != ".toml" {
		return ClientConfig{}, fmt.Errorf("可视化编辑当前仅支持 TOML；%s 请使用原始配置编辑器", ext)
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Default(), nil
	}
	if err != nil {
		return ClientConfig{}, fmt.Errorf("读取配置失败: %w", err)
	}
	if bytes.Contains(data, []byte("{{")) {
		return ClientConfig{}, errors.New("检测到 Go Template 模板语法；为避免破坏模板，请使用原始配置编辑器")
	}
	cfg := Default()
	decoder := toml.NewDecoder(bytes.NewReader(data)).DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return ClientConfig{}, fmt.Errorf("可视化编辑暂不支持此 TOML 中的字段，或配置格式无效: %w", err)
	}
	Normalize(&cfg)
	return cfg, nil
}

func Marshal(cfg ClientConfig) ([]byte, error) {
	Normalize(&cfg)
	if err := Validate(cfg); err != nil {
		return nil, err
	}
	data, err := toml.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("生成 TOML 失败: %w", err)
	}
	return data, nil
}

func Save(path string, cfg ClientConfig) error {
	data, err := Marshal(cfg)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}
	if current, err := os.ReadFile(path); err == nil {
		if err := os.WriteFile(path+".bak", current, 0o600); err != nil {
			return fmt.Errorf("备份原配置失败: %w", err)
		}
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("保存 TOML 失败: %w", err)
	}
	return nil
}

func Normalize(cfg *ClientConfig) {
	if cfg.ServerAddr == "" {
		cfg.ServerAddr = "127.0.0.1"
	}
	if cfg.ServerPort == 0 {
		cfg.ServerPort = 7000
	}
	if cfg.Auth.Method == "" {
		cfg.Auth.Method = "token"
	}
	if cfg.Transport.Protocol == "" {
		cfg.Transport.Protocol = "tcp"
	}
	if cfg.Log.Level == "" {
		cfg.Log.Level = "info"
	}
	if cfg.UDPPacketSize == 0 {
		cfg.UDPPacketSize = 1500
	}
	for i := range cfg.Proxies {
		p := &cfg.Proxies[i]
		p.Name = strings.TrimSpace(p.Name)
		p.Type = strings.ToLower(strings.TrimSpace(p.Type))
		if p.LocalIP == "" && p.Plugin.Type == "" {
			p.LocalIP = "127.0.0.1"
		}
		if p.Transport.BandwidthLimitMode == "" {
			p.Transport.BandwidthLimitMode = "client"
		}
		if p.Type == "tcpmux" && p.Multiplexer == "" {
			p.Multiplexer = "httpconnect"
		}
	}
	for i := range cfg.Visitors {
		v := &cfg.Visitors[i]
		v.Name = strings.TrimSpace(v.Name)
		v.Type = strings.ToLower(strings.TrimSpace(v.Type))
		if v.BindAddr == "" {
			v.BindAddr = "127.0.0.1"
		}
	}
}

func Validate(cfg ClientConfig) error {
	if strings.TrimSpace(cfg.ServerAddr) == "" {
		return errors.New("frps 服务器地址不能为空")
	}
	if cfg.ServerPort < 1 || cfg.ServerPort > 65535 {
		return errors.New("frps 服务器端口必须在 1-65535 之间")
	}
	switch cfg.Auth.Method {
	case "", "token", "oidc":
	default:
		return fmt.Errorf("不支持的认证方式 %q", cfg.Auth.Method)
	}
	switch cfg.Transport.Protocol {
	case "", "tcp", "kcp", "quic", "websocket", "wss":
	default:
		return fmt.Errorf("不支持的传输协议 %q", cfg.Transport.Protocol)
	}
	if cfg.Transport.WireProtocol != "" && cfg.Transport.WireProtocol != "v1" && cfg.Transport.WireProtocol != "v2" {
		return fmt.Errorf("wireProtocol 只能是 v1 或 v2")
	}
	proxyNames := map[string]struct{}{}
	for i, p := range cfg.Proxies {
		if p.Name == "" {
			return fmt.Errorf("第 %d 条代理缺少名称", i+1)
		}
		if _, exists := proxyNames[p.Name]; exists {
			return fmt.Errorf("代理名称 %q 重复", p.Name)
		}
		proxyNames[p.Name] = struct{}{}
		switch p.Type {
		case "tcp", "udp", "http", "https", "tcpmux", "stcp", "sudp", "xtcp":
		default:
			return fmt.Errorf("代理 %q 使用了不支持的类型 %q", p.Name, p.Type)
		}
		if p.Plugin.Type == "" {
			if p.LocalPort < 1 || p.LocalPort > 65535 {
				return fmt.Errorf("代理 %q 的本地端口必须在 1-65535 之间", p.Name)
			}
		}
		if (p.Type == "tcp" || p.Type == "udp") && (p.RemotePort < 0 || p.RemotePort > 65535) {
			return fmt.Errorf("代理 %q 的远程端口必须在 0-65535 之间", p.Name)
		}
		if p.Type == "http" || p.Type == "https" || p.Type == "tcpmux" {
			if len(p.CustomDomains) == 0 && strings.TrimSpace(p.Subdomain) == "" {
				return fmt.Errorf("代理 %q 至少需要 customDomains 或 subdomain", p.Name)
			}
		}
		if p.Type == "stcp" || p.Type == "sudp" || p.Type == "xtcp" {
			if strings.TrimSpace(p.SecretKey) == "" {
				return fmt.Errorf("代理 %q 需要 secretKey", p.Name)
			}
		}
	}
	visitorNames := map[string]struct{}{}
	for i, v := range cfg.Visitors {
		if v.Name == "" {
			return fmt.Errorf("第 %d 条 Visitor 缺少名称", i+1)
		}
		if _, exists := visitorNames[v.Name]; exists {
			return fmt.Errorf("Visitor 名称 %q 重复", v.Name)
		}
		visitorNames[v.Name] = struct{}{}
		switch v.Type {
		case "stcp", "sudp", "xtcp":
		default:
			return fmt.Errorf("Visitor %q 使用了不支持的类型 %q", v.Name, v.Type)
		}
		if strings.TrimSpace(v.ServerName) == "" {
			return fmt.Errorf("Visitor %q 缺少 serverName", v.Name)
		}
		if strings.TrimSpace(v.SecretKey) == "" {
			return fmt.Errorf("Visitor %q 缺少 secretKey", v.Name)
		}
		if v.BindPort > 65535 {
			return fmt.Errorf("Visitor %q 的 bindPort 不能大于 65535", v.Name)
		}
	}
	return nil
}

func ProxyTypes() []string {
	return []string{"tcp", "udp", "http", "https", "tcpmux", "stcp", "sudp", "xtcp"}
}

func VisitorTypes() []string {
	return []string{"stcp", "sudp", "xtcp"}
}

func PluginTypes() []string {
	items := []string{"unix_domain_socket", "http_proxy", "socks5", "static_file", "https2http", "https2https", "http2https", "http2http", "tls2raw", "virtual_net"}
	sort.Strings(items)
	return items
}
