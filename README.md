# FRP Client Manager

一个用于 Windows 的轻量 frpc 桌面管理器，技术栈和桌面生命周期参考 `D:\projects\ai-dev-manager`。

## 当前能力

- Wails 2 + Go 桌面应用
- Windows 系统托盘
- 关闭主窗口后隐藏到托盘
- Windows 登录自启（HKCU Run）
- `--autostart` 隐藏启动
- 单实例运行，重复启动时唤醒主窗口
- 配置 frpc 可执行文件并管理多个独立配置 Profile
- 每个 Profile 可绑定不同 frpc 配置文件、独立启停/重启、独立 PID 与日志
- 多个 Profile 可以同时运行多个 frpc 进程
- 每个 Profile 可单独设置“管理器启动后自动启动”
- 支持全部启动、全部停止、全部重启
- 管理器退出后可选择保留或停止全部 frpc
- PID 记录与已托管进程恢复；旧版单配置自动迁移为“默认连接”
- Windows 下校验 PID 对应的真实可执行文件，避免误杀无关进程
- frpc 配置读取、编辑、备份和保存
- 可视化管理 frps 连接、认证、TLS/传输参数
- 可视化管理 TCP/UDP/HTTP/HTTPS/TCPMux/STCP/SUDP/XTCP 代理
- 可视化管理 STCP/SUDP/XTCP Visitor、健康检查、带宽、负载均衡、Headers、Metadata 和 Client Plugin
- 可视化配置支持 TOML 预览、保存、保存并重启；未知字段会阻止可视化重写，避免静默丢配置
- 保留“原始配置”页面处理 YAML/JSON、Go Template 和未来新增字段
- 调用 `frpc verify -c <config>` 验证配置
- 持久化 frpc 日志并在 UI 中显示最近输出
- 托盘、窗口、任务栏与 EXE 共用 `cmd/frp-client-desktop/assets/appicon.png` 图标源

详细设计见 `docs/frp-visual-config.md`。

## 数据目录

默认使用 Go 的 `os.UserConfigDir()`，Windows 下一般位于：

```text
%APPDATA%\frp-client-manager
```

其中保存：

- `settings.json`
- `frpc.pid.json`
- `frpc.log`

配置文件默认路径也是该目录下的 `frpc.toml`；如果程序同目录已经存在 `frpc.toml`，会优先使用同目录配置。

## 构建

PowerShell：

```powershell
.\scripts\build.ps1
```

或直接：

```powershell
cd .\cmd\frp-client-desktop
wails build -clean
```

输出：

```text
cmd\frp-client-desktop\build\bin\frp-client-manager.exe
```

## CI/CD

- 推送到 `master` 或向 `master` 提交 Pull Request：运行 Windows CI，执行前端检查、`go test ./...`、`go vet ./...` 和 Wails 生产构建，并上传 Windows amd64 构建产物。
- 推送 `v*` 标签：运行 Release 工作流，重新完成全部检查和生产构建，自动创建 GitHub Release，并上传 `frp-client-manager-windows-amd64.exe` 及 SHA-256 文件。
- 首个稳定版本标签为 `v1.0`。

## 运行规则

- 点击窗口关闭按钮：仅隐藏到托盘。
- 托盘“退出（保留 frpc）”：管理器退出，当前所有 frpc 继续运行。
- 托盘“退出并停止 frpc”：先停止全部受管理的 frpc，再退出管理器。
- 下次启动管理器时，如果此前保留的 frpc 仍存在，会通过各 Profile 的 PID 文件恢复管理状态。
