package main

/*
#include <locale.h>
#include <stdlib.h>
*/
import "C"


import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"
	"unsafe"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	mpv "github.com/xmengnet/go-mpv"
)

const (
	renderWidth  = 854
	renderHeight = 480
)

// VideoWidget encapsulates the mpv instance and the GTK picture used to display frames
type VideoWidget struct {
	picture   *gtk.Picture
	rgba      []byte
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

// NewVideoWidget creates a new VideoWidget
func NewVideoWidget() *VideoWidget {
	v := &VideoWidget{
		picture: gtk.NewPicture(),
		rgba:    make([]byte, renderWidth*renderHeight*4),
	}
	v.picture.SetSizeRequest(renderWidth, renderHeight)
	// Make it expand to fill available space
	v.picture.SetHExpand(true)
	v.picture.SetVExpand(true)
	return v
}

// InitPlayer initializes mpv and its software renderer
func (v *VideoWidget) InitPlayer() error {
	// libmpv strict requirement: LC_NUMERIC must be "C"
	// GTK usually overrides this during its own initialization.
	cStr := C.CString("C")
	C.setlocale(C.LC_NUMERIC, cStr)
	C.free(unsafe.Pointer(cStr))

	v.m = mpv.Create()
	if v.m == nil || v.m.MPVHandle() == nil {
		return fmt.Errorf("mpv_create failed (possibly due to locale issues)")
	}

	v.m.SetOptionString("vo", "libmpv")
	v.m.SetOptionString("hwdec", "auto")
	v.m.SetOption("cache", mpv.FORMAT_FLAG, true)
	v.m.SetOption("cache-secs", mpv.FORMAT_DOUBLE, 30.0)

	if err := v.m.Initialize(); err != nil {
		return fmt.Errorf("initialization failed: %w", err)
	}

	rc, err := v.m.NewSoftwareRenderContext()
	if err != nil {
		return fmt.Errorf("failed to create software render context: %w", err)
	}
	v.rc = rc

	go v.eventLoop()

	return nil
}

func (v *VideoWidget) eventLoop() {
	for {
		e := v.m.WaitEvent(10000)
		if e == nil {
			continue
		}

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
				glib.IdleAdd(v.onRefresh)
			}

		case mpv.EVENT_END_FILE:
			v.playing = false
			v.paused = false
			if v.onRefresh != nil {
				glib.IdleAdd(v.onRefresh)
			}

		case mpv.EVENT_SHUTDOWN:
			return
		}
	}
}

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
					unsafe.Pointer(&v.rgba[0]),
				)
				v.mu.Unlock()

				if err == nil {
					// Copy pixel data to avoid data races when main thread creates texture
					frameData := make([]byte, len(v.rgba))
					copy(frameData, v.rgba)

					glib.IdleAdd(func() {
						bytes := glib.NewBytes(frameData)
						texture := gdk.NewMemoryTexture(
							renderWidth, renderHeight,
							gdk.MemoryR8G8B8A8,
							bytes,
							renderWidth*4,
						)
						v.picture.SetPaintable(texture)
					})
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

func (v *VideoWidget) Play(source string) error {
	if v.m == nil {
		return fmt.Errorf("player not initialized")
	}
	return v.m.Command([]string{"loadfile", source})
}

func (v *VideoWidget) PlayURL(url string, headerStr string) error {
	if v.m == nil {
		return fmt.Errorf("player not initialized")
	}
	if headerStr != "" {
		v.m.SetOptionString("http-header-fields", headerStr)
	}
	return v.m.Command([]string{"loadfile", url})
}

func (v *VideoWidget) Pause() {
	if v.m == nil {
		return
	}
	v.paused = !v.paused
	v.m.SetProperty("pause", mpv.FORMAT_FLAG, v.paused)
}

func (v *VideoWidget) Seek(seconds float64) {
	if v.m == nil {
		return
	}
	v.m.SetProperty("time-pos", mpv.FORMAT_DOUBLE, seconds)
}

func (v *VideoWidget) SetVolume(vol int) {
	if v.m == nil {
		return
	}
	v.m.SetProperty("volume", mpv.FORMAT_INT64, int64(vol))
}

func (v *VideoWidget) Stop() {
	if v.m != nil {
		v.m.Command([]string{"stop"})
	}
	v.playing = false
	v.paused = false
	v.position = 0
	v.duration = 0
	v.title = ""
	
	// clear screen
	glib.IdleAdd(func() {
		v.picture.SetPaintable(nil)
	})
}

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

// PlayerUI represents the GTK window and controls
type PlayerUI struct {
	window    *gtk.ApplicationWindow
	video     *VideoWidget
	titleLbl  *gtk.Label
	posLbl    *gtk.Label
	durLbl    *gtk.Label
	progress  *gtk.Scale
	playBtn   *gtk.Button
	stopBtn   *gtk.Button
	volSlider *gtk.Scale
	
	seekUpdateBlocked bool
}

func NewPlayerUI(app *gtk.Application) *PlayerUI {
	window := gtk.NewApplicationWindow(app)
	window.SetTitle("Go-MPV Player (gotk4)")
	window.SetDefaultSize(960, 600)

	ui := &PlayerUI{
		window: window,
		video:  NewVideoWidget(),
	}

	if err := ui.video.InitPlayer(); err != nil {
		log.Fatal("failed to initialize player:", err)
	}

	ui.setupUI()
	return ui
}

func (ui *PlayerUI) setupUI() {
	// Top bar
	topBar := gtk.NewBox(gtk.OrientationHorizontal, 5)
	topBar.SetMarginTop(5)
	topBar.SetMarginBottom(5)
	topBar.SetMarginStart(5)
	topBar.SetMarginEnd(5)

	openFileBtn := gtk.NewButtonWithLabel("Open File")
	openFileBtn.ConnectClicked(func() {
		dialog := gtk.NewFileChooserNative("Open Video", &ui.window.Window, gtk.FileChooserActionOpen, "Open", "Cancel")
		dialog.ConnectResponse(func(response int) {
			if response == int(gtk.ResponseAccept) {
				file := dialog.File()
				if file != nil {
					path := file.Path()
					if err := ui.video.Play(path); err != nil {
						log.Printf("Failed to play: %v\n", err)
					} else {
						ui.playBtn.SetSensitive(true)
					}
				}
			}
			dialog.Destroy()
		})
		dialog.Show()
	})

	openUrlBtn := gtk.NewButtonWithLabel("Open URL")
	// TODO: Replace with a proper dialog to input URL
	openUrlBtn.ConnectClicked(func() {
		log.Println("Open URL clicked (Input dialog to be implemented in GTK4)")
	})

	ui.titleLbl = gtk.NewLabel("Waiting...")
	ui.titleLbl.SetHExpand(true)
	
	topBar.Append(openFileBtn)
	topBar.Append(openUrlBtn)
	topBar.Append(ui.titleLbl)

	// Bottom Control bar
	bottomBar := gtk.NewBox(gtk.OrientationVertical, 5)
	bottomBar.SetMarginTop(5)
	bottomBar.SetMarginBottom(5)
	bottomBar.SetMarginStart(5)
	bottomBar.SetMarginEnd(5)

	// Progress Row
	progressRow := gtk.NewBox(gtk.OrientationHorizontal, 5)
	ui.posLbl = gtk.NewLabel("00:00")
	ui.durLbl = gtk.NewLabel("00:00")
	ui.progress = gtk.NewScaleWithRange(gtk.OrientationHorizontal, 0, 100, 1)
	ui.progress.SetDrawValue(false)
	ui.progress.SetHExpand(true)

	// Update seek position when user changes value
	ui.progress.ConnectValueChanged(func() {
		if !ui.seekUpdateBlocked && ui.video.playing {
			val := ui.progress.Value()
			dur := ui.video.duration
			if dur > 0 {
				ui.video.Seek(val / 100 * dur)
			}
		}
	})

	progressRow.Append(ui.posLbl)
	progressRow.Append(ui.progress)
	progressRow.Append(ui.durLbl)

	// Buttons Row
	controlRow := gtk.NewBox(gtk.OrientationHorizontal, 5)
	controlRow.SetHAlign(gtk.AlignCenter)

	ui.playBtn = gtk.NewButtonWithLabel("Pause")
	ui.playBtn.SetSensitive(false)
	ui.playBtn.ConnectClicked(func() {
		ui.video.Pause()
		if ui.video.paused {
			ui.playBtn.SetLabel("Play")
		} else {
			ui.playBtn.SetLabel("Pause")
		}
	})

	ui.stopBtn = gtk.NewButtonWithLabel("Stop")
	ui.stopBtn.ConnectClicked(func() {
		ui.video.Stop()
		ui.playBtn.SetSensitive(false)
		ui.playBtn.SetLabel("Pause")
		ui.titleLbl.SetText("Waiting...")
		ui.posLbl.SetText("00:00")
		ui.durLbl.SetText("00:00")
		
		ui.seekUpdateBlocked = true
		ui.progress.SetValue(0)
		ui.seekUpdateBlocked = false
	})

	ui.volSlider = gtk.NewScaleWithRange(gtk.OrientationHorizontal, 0, 100, 5)
	ui.volSlider.SetDrawValue(false)
	ui.volSlider.SetValue(100)
	ui.volSlider.SetSizeRequest(100, -1)
	ui.volSlider.ConnectValueChanged(func() {
		ui.video.SetVolume(int(ui.volSlider.Value()))
	})

	controlRow.Append(ui.playBtn)
	controlRow.Append(ui.stopBtn)
	volLabel := gtk.NewLabel("Vol:")
	volLabel.SetMarginStart(20)
	controlRow.Append(volLabel)
	controlRow.Append(ui.volSlider)

	bottomBar.Append(progressRow)
	bottomBar.Append(controlRow)

	// Main Layout
	mainBox := gtk.NewBox(gtk.OrientationVertical, 0)
	mainBox.Append(topBar)
	mainBox.Append(ui.video.picture)
	mainBox.Append(bottomBar)

	ui.window.SetChild(mainBox)

	ui.video.onRefresh = func() {
		ui.titleLbl.SetText(ui.video.title)
		if !ui.video.playing {
			ui.playBtn.SetSensitive(false)
		} else {
			ui.playBtn.SetSensitive(true)
		}
	}

	// Update timer
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			if ui.video.playing {
				pos := ui.video.position
				dur := ui.video.duration
				glib.IdleAdd(func() {
					ui.posLbl.SetText(formatTime(pos))
					ui.durLbl.SetText(formatTime(dur))
					if dur > 0 {
						ui.seekUpdateBlocked = true
						ui.progress.SetValue(pos / dur * 100)
						ui.seekUpdateBlocked = false
					}
				})
			}
		}
	}()
}

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

func main() {
	app := gtk.NewApplication("com.github.xmengnet.gompv.gotk4player", gio.ApplicationFlagsNone)
	
	app.ConnectActivate(func() {
		ui := NewPlayerUI(app)

		if len(os.Args) > 1 {
			source := os.Args[1]
			if err := ui.video.Play(source); err != nil {
				log.Printf("Playback failed: %v", err)
			} else {
				ui.playBtn.SetSensitive(true)
			}
		}
		
		ui.window.Show()
	})

	os.Exit(app.Run(os.Args))
}
