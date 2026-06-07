// Example: Embedded video playback using SDL2 + OpenGL + go-mpv render API.
//
// Usage:
//
//	go run main.go <video_file_or_url>
//
// Requirements:
//
//	go get github.com/veandco/go-sdl2/sdl
//	apt install libsdl2-dev libmpv-dev
package main

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"unsafe"

	"github.com/veandco/go-sdl2/sdl"
	mpv "github.com/xmengnet/go-mpv"
)

func init() {
	// OpenGL and SDL2 require the main thread.
	runtime.LockOSThread()
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: render_sdl <file_or_url>")
		os.Exit(1)
	}

	// ---- SDL2 + OpenGL initialization ----
	if err := sdl.Init(sdl.INIT_VIDEO | sdl.INIT_EVENTS); err != nil {
		log.Fatal("SDL Init:", err)
	}
	defer sdl.Quit()

	// Request OpenGL 3.3 Core Profile
	sdl.GLSetAttribute(sdl.GL_CONTEXT_MAJOR_VERSION, 3)
	sdl.GLSetAttribute(sdl.GL_CONTEXT_MINOR_VERSION, 3)
	sdl.GLSetAttribute(sdl.GL_CONTEXT_PROFILE_MASK, sdl.GL_CONTEXT_PROFILE_CORE)

	window, err := sdl.CreateWindow(
		"go-mpv render example",
		sdl.WINDOWPOS_CENTERED, sdl.WINDOWPOS_CENTERED,
		1280, 720,
		sdl.WINDOW_OPENGL|sdl.WINDOW_RESIZABLE|sdl.WINDOW_SHOWN,
	)
	if err != nil {
		log.Fatal("CreateWindow:", err)
	}
	defer window.Destroy()

	glCtx, err := window.GLCreateContext()
	if err != nil {
		log.Fatal("GLCreateContext:", err)
	}
	defer sdl.GLDeleteContext(glCtx)

	// Enable vsync
	sdl.GLSetSwapInterval(1)

	// ---- mpv initialization ----
	m := mpv.Create()
	if m == nil {
		log.Fatal("mpv.Create() returned nil")
	}

	// Critical: tell mpv not to create its own video window.
	m.SetOptionString("vo", "libmpv")
	m.SetOptionString("hwdec", "auto")
	m.SetOptionString("terminal", "yes")

	if err := m.Initialize(); err != nil {
		log.Fatal("Initialize:", err)
	}

	// Create render context AFTER Initialize().
	rc, err := m.NewRenderContext(func(name string) unsafe.Pointer {
		return sdl.GLGetProcAddress(name)
	}, false) // advancedControl=false is safer for compatibility
	if err != nil {
		log.Fatal("NewRenderContext:", err)
	}
	defer rc.Free()

	// Load the file.
	if err := m.Command([]string{"loadfile", os.Args[1]}); err != nil {
		log.Fatal("loadfile:", err)
	}

	log.Println("Playing:", os.Args[1])

	// ---- Main loop ----
	running := true
	for running {
		// Process SDL events
		for event := sdl.PollEvent(); event != nil; event = sdl.PollEvent() {
			switch e := event.(type) {
			case *sdl.QuitEvent:
				running = false
			case *sdl.KeyboardEvent:
				if e.Type == sdl.KEYDOWN {
					switch e.Keysym.Sym {
					case sdl.K_ESCAPE, sdl.K_q:
						running = false
					case sdl.K_SPACE:
						// Toggle pause
						m.Command([]string{"cycle", "pause"})
					case sdl.K_LEFT:
						m.Command([]string{"seek", "-5"})
					case sdl.K_RIGHT:
						m.Command([]string{"seek", "5"})
					}
				}
			}
		}

		// Check for new frame (non-blocking via channel)
		select {
		case <-rc.WaitUpdate():
			flags := rc.Update()
			if flags&mpv.RENDER_UPDATE_FRAME != 0 {
				w, h := window.GetSize()
				fbo := mpv.OpenGLFBO{
					FBO: 0, // render to default framebuffer
					W:   int(w),
					H:   int(h),
				}
				if err := rc.Render(fbo, true); err != nil {
					log.Println("Render error:", err)
				}
				window.GLSwap()
				rc.ReportSwap()
			}
		default:
			// No frame ready, just idle briefly
			sdl.Delay(1)
		}

		// Process mpv events (non-blocking)
		for {
			e := m.WaitEvent(0)
			if e == nil || e.Event_Id == mpv.EVENT_NONE {
				break
			}
			switch e.Event_Id {
			case mpv.EVENT_SHUTDOWN:
				running = false
			case mpv.EVENT_FILE_LOADED:
				title := m.GetPropertyString("media-title")
				window.SetTitle(fmt.Sprintf("go-mpv: %s", title))
			case mpv.EVENT_END_FILE:
				log.Println("Playback ended")
			}
		}
	}

	log.Println("Exiting...")
	rc.Free()
	m.TerminateDestroy()
}
