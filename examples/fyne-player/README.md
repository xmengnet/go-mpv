# Fyne 视频播放器示例

基于 [Fyne](https://fyne.io/) GUI 框架的嵌入式视频播放器，使用 go-mpv 的软件渲染功能。

## 功能

- 本地视频文件播放
- URL 视频流播放（HTTP/HTTPS）
- 播放/暂停/停止控制
- 进度条拖动跳转
- 音量控制
- 支持 Emby/Jellyfin 等流媒体服务器
- 支持中文界面

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
sudo apt install libgl-dev libx11-dev libxcursor-dev libxrandr-dev libxinerama-dev libxi-dev
```

### 3. 中文字体配置

Fyne 默认不支持中文，需要配置字体。

**重要：** Fyne 只支持 `.ttf` 和 `.otf` 格式，不支持 `.ttc`（字体集合）格式。

#### 查找系统中文字体

```bash
# 列出系统中文字体
fc-list :lang=zh | grep -E "\.(ttf|otf)"
```

#### 设置环境变量

```bash
# 使用找到的 ttf/otf 字体，例如：
export FYNE_FONT="/path/to/your/chinese-font.ttf"

# 示例：
# 霞鹜文楷（推荐，开源字体）
export FYNE_FONT="/home/用户名/.local/share/fonts/LXGWWenKaiMono-Light.ttf"

# 仿宋
export FYNE_FONT="/home/用户名/.local/share/fonts/simfang.ttf"

# 思源黑体（如果有 otf 版本）
export FYNE_FONT="/usr/share/fonts/adobe-source-han-sans/SourceHanSansCN-Regular.otf"
```

#### 安装中文字体（如果没有）

```bash
# Debian/Ubuntu - 安装文泉驿字体
sudo apt install fonts-wqy-zenhei

# 安装后查找 ttf 文件
find /usr/share/fonts -name "*.ttf" | grep -i wqy

# Arch Linux
sudo pacman -S wqy-zenhei

# Fedora
sudo dnf install wqy-zenhei-fonts
```

## 运行

```bash
cd examples/fyne-player

# 设置中文字体（必须使用 ttf 或 otf 格式）
export FYNE_FONT="/home/用户名/.local/share/fonts/LXGWWenKaiMono-Light.ttf"

# 运行播放器
go run main.go

# 或播放指定文件
go run main.go /path/to/video.mp4

# 或播放网络视频
go run main.go http://example.com/video.mp4
```

### 一键运行脚本

```bash
#!/bin/bash
# 找到第一个可用的中文字体
FONT=$(fc-list :lang=zh -f "%{file}\n" | grep -E "\.(ttf|otf)$" | head -1)

if [ -z "$FONT" ]; then
    echo "未找到中文字体，请安装: sudo apt install fonts-wqy-zenhei"
    exit 1
fi

echo "使用字体: $FONT"
FYNE_FONT="$FONT" go run main.go "$@"
```

## 使用说明

### 界面布局

```
┌─────────────────────────────────────────────────────┐
│ [打开文件] [打开链接]                        │
│ 视频标题                                            │
├─────────────────────────────────────────────────────┤
│                                                     │
│                                                     │
│                   视频画面                           │
│                                                     │
│                                                     │
├─────────────────────────────────────────────────────┤
│ 00:05 ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━ 05:30   │
│           [⏸ 暂停] [⏹ 停止]    🔊 ━━━━━━           │
└─────────────────────────────────────────────────────┘
```

### 快捷操作

| 操作 | 说明 |
|------|------|
| 打开文件 | 支持常见视频格式（mp4, mkv, avi 等） |
| 打开链接 | 支持 HTTP/HTTPS 视频流 |
| 暂停按钮 | 暂停/继续播放 |
| 停止按钮 | 停止当前播放 |
| 进度条 | 点击或拖动跳转 |
| 音量条 | 调节音量大小 |

### 播放 Emby/Jellyfin

1. 点击「打开链接」
2. 输入视频地址，例如：
   ```
   http://192.168.1.100:8096/emby/videos/12345/stream?api_key=xxx
   ```
3. 如果需要认证，在「认证头」中输入：
   ```
   X-Emby-Token: your_api_key
   ```

## 技术细节

### 渲染原理

```
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
- 硬件解码减轻 CPU 负担，但视频帧仍需通过 `RenderSW()` 渲染到内存
- 如果遇到问题，可以改为 `hwdec=no` 强制使用软件解码

## 常见问题

### Q: 中文显示为方块？

A: 需要配置中文字体，参见「中文字体配置」部分。

### Q: 字体加载错误 "collections not allowed"？

A: Fyne 不支持 `.ttc` 格式（字体集合）。需要使用单独的 `.ttf` 或 `.otf` 文件。

```bash
# 错误的格式
export FYNE_FONT="font.ttc"  # ❌ 不支持

# 正确的格式
export FYNE_FONT="font.ttf"  # ✅ 支持
export FYNE_FONT="font.otf"  # ✅ 支持
```

查找系统可用的 ttf 字体：
```bash
fc-list :lang=zh -f "%{file}\n" | grep -E "\.(ttf|otf)$"
```

### Q: 播放卡顿？

A: 可以尝试：
- 使用更低的渲染分辨率（修改 `renderWidth` 和 `renderHeight`）
- 设置 `hwdec=no` 使用软件解码
- 增加缓存 `cache-secs`

### Q: 没有声音？

A: 检查音频输出配置：
- Linux: 确保安装 PulseAudio 或 PipeWire
- 修改 `ao` 为 `alsa` 或 `pipewire`

### Q: 退出时崩溃？

A: 确保正确调用 `Destroy()` 方法释放资源。

## 依赖

- [go-mpv](https://github.com/xmengnet/go-mpv) - mpv 绑定
- [Fyne](https://fyne.io/) - GUI 框架
- libmpv - 视频播放库
