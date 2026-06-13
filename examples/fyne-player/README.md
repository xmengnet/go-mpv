# Fyne 视频播放器示例

基于 [Fyne](https://fyne.io/) GUI 框架的嵌入式视频播放器，使用 go-mpv 的软件渲染功能。本案例展示了原生 Wayland 支持以及现代化沉浸式深色模式 UI。

## 功能

- 本地视频文件播放
- URL 视频流播放（HTTP/HTTPS）
- 播放/暂停/停止控制
- 进度条拖动跳转
- 音量控制
- 支持 Emby/Jellyfin 等流媒体服务器
- 沉浸式半透明悬浮控制面板
- 原生 Wayland 窗口与渲染支持

## 前置要求

### 1. 安装 libmpv

```bash
# Debian/Ubuntu
sudo apt install libmpv-dev

# Arch Linux
sudo pacman -S mpv

# macOS
brew install mpv

# Fedora
sudo dnf install mpv-devel
```

### 2. 安装 Fyne 依赖（Linux）

```bash
# X11 及通用底层依赖
sudo apt install libgl-dev libx11-dev libxcursor-dev libxrandr-dev libxinerama-dev libxi-dev

# Wayland 原生协议依赖
sudo apt install libwayland-dev wayland-protocols
```

## 运行

**注意**：Go-GLFW 在 Linux/Wayland 环境下由于宏定义缺失可能导致编译报错。本项目内置了 `Makefile` 处理 CGO 环境变量和 Wayland 构建标签。

```bash
cd examples/fyne-player

# 直接运行（推荐，已包含 Wayland 标签和编译宏修复）
make run

# 或手动运行（效果同上）
CGO_CFLAGS="-D_GNU_SOURCE" go run -tags wayland main.go

# 运行并打开指定的本地文件
CGO_CFLAGS="-D_GNU_SOURCE" go run -tags wayland main.go /path/to/video.mp4
```

## 使用说明

### 界面布局

```text
┌─────────────────────────────────────────────────────┐
│ [Open File] [Open URL]                              │
│                      Video Title                    │
│                                                     │
│                                                     │
│                                                     │
│                   视频渲染全屏层                        │
│                                                     │
│                                                     │
│  ┌───────────────────────────────────────────────┐  │
│  │ 00:05 ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━ 05:30   │  │
│  │           [⏸] [⏹]    🔊 ━━━━━━                  │  │
│  └───────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────┘
```

### 快捷操作

| 操作 | 说明 |
|------|------|
| Open File | 支持常见视频格式（mp4, mkv, avi 等） |
| Open URL | 支持 HTTP/HTTPS 视频流 |
| 暂停按钮 | 暂停/继续播放 |
| 停止按钮 | 停止当前播放 |
| 进度条 | 点击或拖动跳转 |
| 音量条 | 调节音量大小 |

### 播放 Emby/Jellyfin

1. 点击「Open URL」
2. 输入视频地址，例如：
   ```text
   http://192.168.1.100:8096/emby/videos/12345/stream?api_key=xxx
   ```
3. 如果需要认证，在「Headers」中输入：
   ```text
   X-Emby-Token: your_api_key
   ```

## 技术细节

### 渲染原理

```text
┌──────────────────────────────────────────────────────┐
│                    播放流程                            │
│                                                      │
│  mpv 解码视频                                         │
│      ↓                                               │
│  vo=libmpv 输出到 RenderContext                       │
│      ↓                                               │
│  RenderSW() 渲染到 image.RGBA.Pix                    │
│      ↓                                               │
│  Fyne canvas.Image 显示到界面                         │
│                                                      │
└──────────────────────────────────────────────────────┘
```

### 关键配置

| 配置项 | 值 | 说明 |
|--------|-----|------|
| `vo` | `libmpv` | 使用 libmpv 软渲染输出 |
| `hwdec` | `auto` | 自动检测硬件解码 |
| `ao` | `pulse` | 使用 PulseAudio 音频输出 |

### 硬件解码说明

- `hwdec=auto`：mpv 会自动检测并使用可用的硬件解码器（VAAPI, VDPAU, NVDEC 等）
- 硬件解码能减轻 CPU 负担，但视频帧仍需通过 `RenderSW()` 拉取内存像素后由 CPU 渲染
- 如果遇到花屏或严重性能问题，可以改为 `hwdec=no` 强制使用软件解码

## 常见问题

### Q: 启动报 "implicit declaration of function xxx" 编译错误？

A: 这是由于第三方库 Go-GLFW 在您的 Linux 编译环境中缺失相关的宏。我们在 `Makefile` 中已经通过 `CGO_CFLAGS="-D_GNU_SOURCE"` 进行修复，请直接使用 `make run`。

### Q: 控制台出现 "PlatformError: Wayland: Focusing a window requires user interaction"？

A: 这是 Wayland 的原生协议安全限制，禁止应用在没有用户交互的情况下强行抢夺窗口焦点或获取全局坐标。这是正常的日志提示，不会影响任何视频播放功能。

### Q: 播放卡顿或掉帧？

A: `fyne-player` 属于**软件像素渲染（RenderSW）**的展示案例，依赖于 CPU 拉取像素写入贴图，受限于单线程拷贝带宽。可以尝试：
- 使用更低的渲染分辨率（修改 `main.go` 中的 `renderWidth` 和 `renderHeight` 常量）
- 设置 `hwdec=no` 使用软件解码，有时软解配合软渲染效果更佳

### Q: 没有声音？

A: 检查音频输出配置：
- Linux: 确保安装 PulseAudio 或 PipeWire
- 尝试将初始化代码中的 `ao` 选项改为 `alsa` 或 `pipewire`

## 依赖

- [go-mpv](https://github.com/xmengnet/go-mpv) - mpv 绑定
- [Fyne](https://fyne.io/) - 现代 GUI 框架
- libmpv - 视频播放核心库
