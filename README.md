# FRP Client Manager

一个用于 Windows、Linux 和 macOS 的轻量 frpc 桌面管理器，技术栈和桌面生命周期参考 `D:\projects\ai-dev-manager`。

## 当前能力

- Wails 2 + Go 桌面应用
- Windows / Linux / macOS 系统托盘
- Windows/macOS 在托盘就绪后关闭到托盘；Linux 默认保持可见
- 跨平台登录自启：Windows HKCU Run、Linux XDG Autostart、macOS LaunchAgent
- `--autostart` 在 Windows/macOS 托盘就绪后隐藏；Linux 自启动保持可见
- 单实例运行，重复启动时唤醒主窗口
- 配置 frpc 可执行文件并管理多个独立配置 Profile
- 每个 Profile 可绑定不同 frpc 配置文件、独立启停/重启、独立 PID 与日志
- 多个 Profile 可以同时运行多个 frpc 进程
- 每个 Profile 可单独设置“管理器启动后自动启动”
- 支持全部启动、全部停止、全部重启
- 管理器退出后可选择保留或停止全部 frpc
- PID 记录与已托管进程恢复；旧版单配置自动迁移为“默认连接”
- Windows / Linux 下校验 PID 对应的真实可执行文件；macOS 校验 PID 与进程命令，降低误杀无关进程风险
- frpc 配置读取、编辑、备份和保存
- 可视化管理 frps 连接、认证、TLS/传输参数
- 可视化管理 TCP/UDP/HTTP/HTTPS/TCPMux/STCP/SUDP/XTCP 代理
- 可视化管理 STCP/SUDP/XTCP Visitor、健康检查、带宽、负载均衡、Headers、Metadata 和 Client Plugin
- 可视化配置支持 TOML 预览、保存、保存并重启；未知字段会阻止可视化重写，避免静默丢配置
- 保留“原始配置”页面处理 YAML/JSON、Go Template 和未来新增字段
- 调用 `frpc verify -c <config>` 验证配置
- 持久化 frpc 日志并在 UI 中显示最近输出
- 托盘和桌面应用构建共用 `cmd/frp-client-desktop/assets/appicon.png` 图标源

详细设计见 `docs/frp-visual-config.md`。

## 数据目录

默认使用 Go 的 `os.UserConfigDir()`：

- Windows：`%APPDATA%\frp-client-manager`
- Linux：`$XDG_CONFIG_HOME/frp-client-manager`，通常为 `~/.config/frp-client-manager`
- macOS：`~/Library/Application Support/frp-client-manager`

其中保存：

- `settings.json`
- 默认 Profile 的 `frpc.pid.json` / `frpc.log`
- 其他 Profile 的独立 PID / 日志目录

配置文件默认路径也是该目录下的 `frpc.toml`；如果程序同目录已经存在 `frpc.toml`，会优先使用同目录配置。Windows 默认查找 `frpc.exe`，Linux/macOS 默认查找 `frpc`。

## 构建

Windows：

```powershell
.\scripts\build.ps1
```

Linux / macOS：

```bash
bash ./scripts/build.sh
```

Linux 构建脚本使用 Wails 的 `webkit2_41` tag，构建机需要 GTK3 与 WebKitGTK 4.1 开发依赖。

输出：

```text
Windows: cmd/frp-client-desktop/build/bin/frp-client-manager.exe
Linux:   cmd/frp-client-desktop/build/bin/frp-client-manager
macOS:   cmd/frp-client-desktop/build/bin/frp-client-manager.app
```

## CI/CD

桌面基础库和 reusable workflow 均固定为 Wails Desktop Kit **v0.2.0**。常规业务 API 用法保持兼容；Linux 的 HideSafe 行为详见下方运行规则。

- 推送到 `master` 或向 `master` 提交 Pull Request：Windows、Ubuntu 和 macOS 三个平台分别执行前端检查、`go test ./...`、`go vet ./...` 与 Wails 生产构建。
- CI 上传 Windows amd64 EXE、Linux amd64 可执行文件、macOS Universal（Intel + Apple Silicon）`.app.zip`，并为各产物生成 SHA-256。
- 推送 `v*` 标签：三平台重新完整构建，全部成功后才创建 GitHub Release 并合并上传三平台产物。
- 首个 Windows-only 稳定版本标签为 `v1.0`；跨平台发布从后续版本开始。

## 运行规则

- 点击窗口关闭按钮：Windows/macOS 托盘可用时隐藏到托盘；Linux 或托盘失败时退出管理器并保留 frpc。
- 托盘初始化或运行失败会恢复主窗口，避免应用隐藏后无法找回。
- 托盘“退出（保留 frpc）”：管理器退出，当前所有 frpc 继续运行。
- 托盘“退出并停止 frpc”：先停止全部受管理的 frpc，再退出管理器。
- 下次启动管理器时，如果此前保留的 frpc 仍存在，会通过各 Profile 的 PID 文件恢复管理状态。
