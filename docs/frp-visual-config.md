# FRP 可视化配置设计

基线：FRP v0.71.x 的现代客户端配置（TOML / YAML / JSON）。本项目的可视化编辑器优先支持 TOML；YAML、JSON、Go Template 和未来未知字段保留“原始配置”入口。

## 多配置 / 多实例模型

管理器支持多个 Profile。每个 Profile 包含：

- 独立名称
- 独立 frpc 配置文件路径
- 独立“管理器启动后自动启动”开关
- 独立 PID 记录
- 独立运行日志
- 独立 frpc 进程生命周期

所有 Profile 共用同一个 frpc 可执行文件路径，但可以同时启动多个 frpc 进程。

运行页提供：

- 每个 Profile 的运行状态、PID、配置文件路径
- 单独启动 / 停止 / 重启
- 全部启动 / 全部停止 / 全部重启
- 设置当前编辑 Profile

“连接配置”和“原始配置”页面通过顶部 Profile 下拉框切换当前配置。

旧版单配置的 `config_path` 与 `start_frpc_on_launch` 会自动迁移为名为“默认连接”的 Profile；默认连接继续使用旧版 PID/日志目录，以便恢复升级前仍在运行的 frpc。

## 页面结构

### 1. 服务器连接

默认只显示最常用参数：

- `serverAddr`
- `serverPort`
- `user`
- `clientID`
- `auth.method`
- `auth.token`
- `transport.protocol`
- `transport.wireProtocol`
- `transport.tls.enable`
- `transport.tcpMux`
- `loginFailExit`

高级折叠区包括：

- OIDC
- 预连接池、拨号超时、Keepalive、Heartbeat
- 上游 HTTP / SOCKS5 / NTLM Proxy
- QUIC
- TLS 证书、CA、Server Name
- frpc Web Server / API
- 日志、DNS、STUN、UDP Packet Size
- Includes、Metadata、Feature Gates、VirtualNet

### 2. 代理映射

左侧为代理列表，右侧为单条代理编辑器。支持：

- `tcp`
- `udp`
- `http`
- `https`
- `tcpmux`
- `stcp`
- `sudp`
- `xtcp`

每条代理使用 `enabled` 独立启停。新配置不主动使用全局 `start` 白名单；读取已有 `start` 时会保留。

公共能力：

- Local IP / Local Port
- 加密 / 压缩
- 带宽限制与 client/server 限制位置
- Proxy Protocol v1/v2
- Load Balancer Group / Group Key
- TCP / HTTP Health Check
- Metadata / Annotations
- Client Plugin

按类型动态显示：

- TCP / UDP：Remote Port
- HTTP：Domains、Subdomain、Locations、Basic Auth、Route By HTTP User、Host Header Rewrite、Request/Response Headers
- HTTPS：Domains、Subdomain
- TCPMux：Domains、Subdomain、HTTP Auth、Route By HTTP User、Multiplexer
- STCP / SUDP / XTCP：Secret Key、Allow Users
- XTCP：NAT Traversal

Client Plugin：

- `unix_domain_socket`
- `http_proxy`
- `socks5`
- `static_file`
- `https2http`
- `https2https`
- `http2https`
- `http2http`
- `tls2raw`
- `virtual_net`

### 3. Visitor

支持：

- STCP Visitor
- SUDP Visitor
- XTCP Visitor
- VirtualNet Visitor Plugin

公共字段：

- name / enabled
- serverUser / serverName
- secretKey
- bindAddr / bindPort
- encryption / compression

XTCP 额外支持：

- keepTunnelOpen
- maxRetriesAnHour
- minRetryInterval
- fallbackTo
- fallbackTimeoutMs
- NAT Traversal

## 安全保存策略

可视化编辑器不会盲目重写无法识别的配置：

1. 只对 TOML 启用可视化读写。
2. TOML 使用严格字段解析。
3. 检测到未知字段或 Go Template 时，拒绝进入可视化保存并提示使用“原始配置”。
4. 保存前进行应用侧结构校验。
5. 保存时保留 `.bak`。
6. “预览 TOML”可以在写盘前查看最终结果。
7. “保存并重启”仅在配置成功写盘后才重启 frpc。

## 原始配置保留的场景

以下场景优先使用原始编辑器：

- YAML / JSON
- Go Template
- FRP 新版本新增但本 GUI 尚未同步的字段
- 需要精确保留注释和排版的配置
- 特殊插件或实验性配置

## 图标

`cmd/frp-client-desktop/assets/appicon.png` 是唯一图标源：

- 系统托盘直接嵌入此 PNG。
- 构建脚本复制到 `build/appicon.png`。
- 构建前删除旧 `build/windows/icon.ico`，由 Wails 重新生成 Windows 多尺寸 ICO。
- Windows 窗口标题栏、任务栏和 EXE 资源因此使用同一图标。
