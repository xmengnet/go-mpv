package main

import (
	"fmt"
	"image"
	"image/color"
	"log"
	"os"
	"sync"
	"time"
	"unsafe"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	mpv "github.com/xmengnet/go-mpv"
)

const (
	renderWidth  = 854
	renderHeight = 480
)

// VideoWidget 视频渲染组件
type VideoWidget struct {
	widget.BaseWidget
	image     *canvas.Image
	rgba      *image.RGBA
	m         *mpv.Mpv
	rc        *mpv.RenderContext
	mu        sync.Mutex
	playing   bool
	paused    bool
	duration  float64
	position  float64
	title     string
	onRefresh func()
}

// NewVideoWidget 创建视频组件
func NewVideoWidget() *VideoWidget {
	v := &VideoWidget{
		rgba: image.NewRGBA(image.Rect(0, 0, renderWidth, renderHeight)),
	}
	v.ExtendBaseWidget(v)
	return v
}

// InitPlayer 初始化播放器
func (v *VideoWidget) InitPlayer() error {
	v.m = mpv.Create()

	// 配置
	v.m.SetOptionString("vo", "libmpv")
	v.m.SetOptionString("hwdec", "auto")
	v.m.SetOptionString("ao", "pulse")
	v.m.SetOption("cache", mpv.FORMAT_FLAG, true)
	v.m.SetOption("cache-secs", mpv.FORMAT_DOUBLE, 30.0)

	if err := v.m.Initialize(); err != nil {
		return fmt.Errorf("initialization failed: %w", err)
	}

	// 初始化软渲染上下文
	rc, err := v.m.NewSoftwareRenderContext()
	if err != nil {
		return fmt.Errorf("failed to create software render context: %w", err)
	}
	v.rc = rc

	go v.eventLoop()

	return nil
}

// eventLoop 事件循环
func (v *VideoWidget) eventLoop() {
	for {
		e := v.m.WaitEvent(10000)

		switch e.Event_Id {
		case mpv.EVENT_FILE_LOADED:
			v.playing = true
			v.title = v.m.GetPropertyString("media-title")
			duration, _ := v.m.GetProperty("duration", mpv.FORMAT_DOUBLE)
			if d, ok := duration.(float64); ok {
				v.duration = d
			}
			go v.renderLoop()
			if v.onRefresh != nil {
				v.onRefresh()
			}

		case mpv.EVENT_END_FILE:
			v.playing = false
			v.paused = false
			if v.onRefresh != nil {
				v.onRefresh()
			}

		case mpv.EVENT_SHUTDOWN:
			return
		}
	}
}

// renderLoop 渲染循环
func (v *VideoWidget) renderLoop() {
	time.Sleep(100 * time.Millisecond)

	for v.playing {
		select {
		case <-v.rc.WaitUpdate():
			flags := v.rc.Update()
			if flags&mpv.RENDER_UPDATE_FRAME != 0 {
				v.mu.Lock()
				err := v.rc.RenderSW(
					renderWidth, renderHeight,
					"rgba",
					renderWidth*4,
					unsafe.Pointer(&v.rgba.Pix[0]),
				)
				v.mu.Unlock()

				if err == nil && v.image != nil {
					v.image.Refresh()
				}
				v.rc.ReportSwap()
			}
		case <-time.After(100 * time.Millisecond):
		}

		if !v.playing {
			break
		}

		pos, _ := v.m.GetProperty("time-pos", mpv.FORMAT_DOUBLE)
		if p, ok := pos.(float64); ok {
			v.position = p
		}
	}
}

// Play 播放文件或URL
func (v *VideoWidget) Play(source string) error {
	if v.m == nil {
		return fmt.Errorf("player not initialized")
	}
	return v.m.Command([]string{"loadfile", source})
}

// PlayURL 播放URL流
func (v *VideoWidget) PlayURL(url string, headers map[string]string) error {
	if v.m == nil {
		return fmt.Errorf("player not initialized")
	}

	if len(headers) > 0 {
		headerStr := ""
		for k, val := range headers {
			if headerStr != "" {
				headerStr += ","
			}
			headerStr += k + ": " + val
		}
		v.m.SetOptionString("http-header-fields", headerStr)
	}

	return v.m.Command([]string{"loadfile", url})
}

// Pause 暂停/继续
func (v *VideoWidget) Pause() {
	if v.m == nil {
		return
	}
	v.paused = !v.paused
	v.m.SetProperty("pause", mpv.FORMAT_FLAG, v.paused)
}

// Seek 跳转
func (v *VideoWidget) Seek(seconds float64) {
	if v.m == nil {
		return
	}
	v.m.SetProperty("time-pos", mpv.FORMAT_DOUBLE, seconds)
}

// SetVolume 设置音量
func (v *VideoWidget) SetVolume(vol int) {
	if v.m == nil {
		return
	}
	v.m.SetProperty("volume", mpv.FORMAT_INT64, int64(vol))
}

// GetPosition 获取当前播放位置
func (v *VideoWidget) GetPosition() float64 {
	return v.position
}

// GetDuration 获取总时长
func (v *VideoWidget) GetDuration() float64 {
	return v.duration
}

// GetTitle 获取标题
func (v *VideoWidget) GetTitle() string {
	return v.title
}

// IsPlaying 是否正在播放
func (v *VideoWidget) IsPlaying() bool {
	return v.playing
}

// IsPaused 是否暂停
func (v *VideoWidget) IsPaused() bool {
	return v.paused
}

// Stop 停止播放
func (v *VideoWidget) Stop() {
	if v.m != nil {
		v.m.Command([]string{"stop"})
	}
	v.playing = false
	v.paused = false
	v.position = 0
	v.duration = 0
	v.title = ""
}

// Destroy 销毁播放器
func (v *VideoWidget) Destroy() {
	if v.rc != nil {
		v.rc.Free()
		v.rc = nil
	}
	if v.m != nil {
		v.m.TerminateDestroy()
		v.m = nil
	}
}

// CreateRenderer 创建渲染器
func (v *VideoWidget) CreateRenderer() fyne.WidgetRenderer {
	v.image = canvas.NewImageFromImage(v.rgba)
	v.image.FillMode = canvas.ImageFillContain
	v.image.SetMinSize(fyne.NewSize(400, 225))
	return &videoRenderer{widget: v}
}

// videoRenderer Fyne渲染器
type videoRenderer struct {
	widget *VideoWidget
}

func (r *videoRenderer) Layout(size fyne.Size) {
	r.widget.image.Resize(size)
}

func (r *videoRenderer) MinSize() fyne.Size {
	return fyne.NewSize(400, 225)
}

func (r *videoRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.widget.image}
}

func (r *videoRenderer) Destroy() {}

func (r *videoRenderer) Refresh() {
	canvas.Refresh(r.widget.image)
}

// PlayerUI 播放器UI
type PlayerUI struct {
	window    fyne.Window
	video     *VideoWidget
	titleLbl  *widget.Label
	posLbl    *widget.Label
	durLbl    *widget.Label
	progress  *widget.Slider
	playBtn   *widget.Button
	stopBtn   *widget.Button
	volSlider *widget.Slider
}

// NewPlayerUI 创建播放器UI
func NewPlayerUI(w fyne.Window) *PlayerUI {
	ui := &PlayerUI{
		window: w,
		video:  NewVideoWidget(),
	}

	if err := ui.video.InitPlayer(); err != nil {
		log.Fatal("failed to initialize player:", err)
	}

	ui.setupUI()
	return ui
}

// setupUI 初始化UI
func (ui *PlayerUI) setupUI() {
	// 标题
	ui.titleLbl = widget.NewLabel("Waiting...")
	ui.titleLbl.Wrapping = fyne.TextTruncate
	ui.titleLbl.TextStyle = fyne.TextStyle{Bold: true}

	// 时间标签
	ui.posLbl = widget.NewLabel("00:00")
	ui.durLbl = widget.NewLabel("00:00")

	// 进度条
	ui.progress = widget.NewSlider(0, 100)
	ui.progress.OnChanged = func(value float64) {
		if ui.video.IsPlaying() {
			ui.video.Seek(value / 100 * ui.video.GetDuration())
		}
	}

	// 播放/暂停按钮
	ui.playBtn = widget.NewButtonWithIcon("", theme.MediaPauseIcon(), func() {
		ui.video.Pause()
		if ui.video.IsPaused() {
			ui.playBtn.SetIcon(theme.MediaPlayIcon())
		} else {
			ui.playBtn.SetIcon(theme.MediaPauseIcon())
		}
	})
	ui.playBtn.Disable()

	// 停止按钮
	ui.stopBtn = widget.NewButtonWithIcon("", theme.MediaStopIcon(), func() {
		ui.video.Stop()
		ui.playBtn.Disable()
		ui.playBtn.SetIcon(theme.MediaPauseIcon())
		ui.titleLbl.SetText("Waiting...")
		ui.posLbl.SetText("00:00")
		ui.durLbl.SetText("00:00")
		ui.progress.SetValue(0)
	})

	// 音量图标和滑块
	volIcon := widget.NewIcon(theme.VolumeUpIcon())
	ui.volSlider = widget.NewSlider(0, 100)
	ui.volSlider.SetValue(100)
	ui.volSlider.OnChanged = func(value float64) {
		ui.video.SetVolume(int(value))
	}

	// 文件选择按钮
	fileBtn := widget.NewButtonWithIcon("Open File", theme.FolderOpenIcon(), func() {
		fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			defer reader.Close()

			path := reader.URI().Path()
			if err := ui.video.Play(path); err != nil {
				dialog.ShowError(err, ui.window)
				return
			}
			ui.playBtn.Enable()
		}, ui.window)
		fd.Show()
	})

	// URL输入按钮
	urlBtn := widget.NewButtonWithIcon("Open URL", theme.ComputerIcon(), func() {
		urlEntry := widget.NewEntry()
		urlEntry.SetPlaceHolder("http://example.com/video.mp4")

		headerEntry := widget.NewEntry()
		headerEntry.SetPlaceHolder("Optional: X-Emby-Token: your_token")

		items := []*widget.FormItem{
			widget.NewFormItem("Video URL", urlEntry),
			widget.NewFormItem("Headers", headerEntry),
		}

		dialog.ShowForm("Open Stream", "Play", "Cancel", items, func(ok bool) {
			if !ok || urlEntry.Text == "" {
				return
			}

			headers := make(map[string]string)
			if headerEntry.Text != "" {
				parts := splitHeader(headerEntry.Text)
				if len(parts) == 2 {
					headers[parts[0]] = parts[1]
				}
			}

			if err := ui.video.PlayURL(urlEntry.Text, headers); err != nil {
				dialog.ShowError(err, ui.window)
				return
			}
			ui.playBtn.Enable()
		}, ui.window)
	})

	// 刷新回调
	ui.video.onRefresh = func() {
		ui.titleLbl.SetText(ui.video.GetTitle())
	}

	// 定时更新进度
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			if ui.video.IsPlaying() {
				pos := ui.video.GetPosition()
				dur := ui.video.GetDuration()
				ui.posLbl.SetText(formatTime(pos))
				ui.durLbl.SetText(formatTime(dur))
				if dur > 0 {
					ui.progress.SetValue(pos / dur * 100)
				}
			}
		}
	}()

	// ===== 布局 =====

	// 顶部：标题栏
	topBar := container.NewHBox(
		fileBtn,
		urlBtn,
		layout.NewSpacer(),
	)
	
	// 对标题进行居中和样式增强
	titleContainer := container.NewCenter(ui.titleLbl)
	topPanel := container.NewVBox(topBar, titleContainer)

	// 视频区域
	videoArea := ui.video

	// 底部控制栏
	// 进度条行
	progressRow := container.NewBorder(
		nil, nil,
		container.NewPadded(ui.posLbl),
		container.NewPadded(ui.durLbl),
		ui.progress,
	)

	// 按钮行
	controlRow := container.NewHBox(
		layout.NewSpacer(),
		ui.playBtn,
		ui.stopBtn,
		layout.NewSpacer(),
		container.NewCenter(volIcon),
		ui.volSlider,
	)

	bottomBar := container.NewVBox(
		progressRow,
		controlRow,
	)
	
	// 给控制栏增加带半透明背景的垫层
	paddedBottom := container.NewPadded(bottomBar)
	bg := canvas.NewRectangle(color.NRGBA{R: 24, G: 24, B: 24, A: 220})
	bottomPanel := container.NewStack(bg, paddedBottom)

	// 主布局：使用 Stack 让视频沉浸式铺满底层，控制栏悬浮在上层
	content := container.NewStack(
		videoArea,
		container.NewBorder(
			container.NewPadded(topPanel),
			bottomPanel,
			nil, nil,
		),
	)

	ui.window.SetContent(content)
}

// Destroy 销毁
func (ui *PlayerUI) Destroy() {
	ui.video.Destroy()
}

// formatTime 格式化时间
func formatTime(seconds float64) string {
	if seconds < 0 {
		seconds = 0
	}
	h := int(seconds) / 3600
	m := (int(seconds) % 3600) / 60
	s := int(seconds) % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}

// splitHeader 分割HTTP头
func splitHeader(header string) []string {
	for i := 0; i < len(header); i++ {
		if header[i] == ':' {
			return []string{
				header[:i],
				header[i+2:],
			}
		}
	}
	return nil
}

func main() {
	a := app.New()
	a.Settings().SetTheme(theme.DarkTheme())
	w := a.NewWindow("Go-MPV Player")
	w.Resize(fyne.NewSize(960, 600))

	ui := NewPlayerUI(w)

	// 命令行参数播放
	if len(os.Args) > 1 {
		source := os.Args[1]
		if err := ui.video.Play(source); err != nil {
			log.Printf("Playback failed: %v", err)
		} else {
			ui.playBtn.Enable()
		}
	}

	w.SetOnClosed(func() {
		ui.Destroy()
	})

	w.ShowAndRun()
}
