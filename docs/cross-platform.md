# 跨平台实现说明

当前目标平台：

- Windows 10/11 amd64
- Linux amd64（桌面环境）
- macOS Universal（Intel + Apple Silicon）

## 系统托盘

使用 `github.com/gogpu/systray` 的统一实现：

- Windows：Shell Notification Area
- Linux：D-Bus StatusNotifierItem
- macOS：NSStatusItem

三个目标平台统一支持：

- 显示 / 隐藏主窗口
- 启动全部连接
- 停止全部连接
- 重启全部连接
- 登录系统时启动管理器
- 退出并保留 frpc
- 退出并停止全部 frpc

关闭主窗口时继续隐藏到托盘。

## 登录自启

### Windows

使用：

`HKCU\Software\Microsoft\Windows\CurrentVersion\Run`

启动参数：

`--autostart`

### Linux

使用 XDG Autostart：

`~/.config/autostart/frp-client-manager.desktop`

实际配置目录遵循 `os.UserConfigDir()` / `XDG_CONFIG_HOME`。

### macOS

使用当前用户 LaunchAgent：

`~/Library/LaunchAgents/com.wanstu.frp-client-manager.plist`

LaunchAgent 使用当前应用可执行文件路径，并传入 `--autostart`。

## frpc 进程管理

所有平台均为每个 Profile 维护独立：

- PID 文件
- 日志文件
- 启动 / 停止 / 重启状态

Windows 使用 Win32 Process API 检查 PID 存活并核对真实可执行文件。

Linux 使用 signal 0 检查 PID，并通过 `/proc/<pid>/exe` 核对真实可执行文件。

macOS 使用 signal 0 检查 PID，并通过 `ps` 的进程命令信息核对预期程序。

Linux/macOS 启动 frpc 时创建独立 session，使管理器退出后 frpc 可以继续运行。

## 默认 frpc 名称

- Windows：`frpc.exe`
- Linux / macOS：`frpc`

如果桌面应用所在目录存在对应 frpc，会优先使用同目录文件。

## 构建

Windows：

```powershell
.\scripts\build.ps1
```

Linux/macOS：

```bash
bash ./scripts/build.sh
```

Linux 使用 WebKitGTK 4.1：

```text
-tags webkit2_41
```

Ubuntu CI 安装：

```text
libgtk-3-dev
libwebkit2gtk-4.1-dev
```

## CI/CD

CI 使用三个原生 GitHub runner：

- `windows-latest`
- `ubuntu-24.04`
- `macos-latest`

这样 Wails 最终链接和打包由目标系统自身完成，而不是依赖 Windows 交叉链接。

未来 Release 产物：

- `frp-client-manager-windows-amd64.exe`
- `frp-client-manager-linux-amd64`
- `frp-client-manager-macos-universal.app.zip`
- 每个产物对应的 SHA-256 文件

## 当前验证状态

已验证：

- Windows 单元测试 / Vet
- Windows Wails production build
- GitHub Actions workflow 通过 actionlint
- Linux amd64 核心层交叉编译
- Linux amd64 Desktop 入口交叉编译
- macOS arm64 核心层交叉编译
- macOS arm64 Desktop 入口交叉编译

仍需目标系统真机或原生 CI 验证：

- Linux 桌面托盘实际显示 / 点击
- Linux XDG 登录自启
- Linux 保留 frpc 后重启管理器恢复 PID
- macOS 菜单栏托盘实际显示 / 点击
- macOS LaunchAgent 登录自启
- macOS `.app` 双击启动与隐藏到托盘
- macOS 保留 frpc 后重启管理器恢复 PID
