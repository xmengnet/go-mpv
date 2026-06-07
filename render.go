package mpv

/*
#include <mpv/render.h>
#include <mpv/render_gl.h>
#include <stdlib.h>
#include <stdint.h>

// Forward declaration for Go callback bridge.
extern void goRenderUpdateCallback(void *ctx);

// Bridge function: calls the Go-exported callback.
static void render_update_callback_bridge(void *ctx) {
    goRenderUpdateCallback(ctx);
}

// set_update_callback_bridge wraps mpv_render_context_set_update_callback
// with our C bridge function.
static void set_update_callback_bridge(mpv_render_context *ctx, uintptr_t callback_ctx) {
    mpv_render_context_set_update_callback(ctx, render_update_callback_bridge, (void*)callback_ctx);
}

// Forward declaration for GL get_proc_address bridge.
extern void* goGLGetProcAddress(void *ctx, char *name);

// Bridge function for OpenGL get_proc_address.
// Cast away const to match the Go export signature.
static void* gl_get_proc_address_bridge(void *ctx, const char *name) {
    return goGLGetProcAddress(ctx, (char*)name);
}

// create_render_context_gl creates a render context for the OpenGL backend.
// This helper avoids complex struct construction in Go.
static int create_render_context_gl(mpv_render_context **res, mpv_handle *mpv,
                                     uintptr_t get_proc_ctx, int advanced_control) {
    mpv_opengl_init_params gl_params = {
        .get_proc_address = gl_get_proc_address_bridge,
        .get_proc_address_ctx = (void*)get_proc_ctx,
    };
    int adv = advanced_control;
    mpv_render_param params[] = {
        {MPV_RENDER_PARAM_API_TYPE, MPV_RENDER_API_TYPE_OPENGL},
        {MPV_RENDER_PARAM_OPENGL_INIT_PARAMS, &gl_params},
        {MPV_RENDER_PARAM_ADVANCED_CONTROL, &adv},
        {0}
    };
    return mpv_render_context_create(res, mpv, params);
}

// create_render_context_sw creates a render context for the software backend.
static int create_render_context_sw(mpv_render_context **res, mpv_handle *mpv) {
    mpv_render_param params[] = {
        {MPV_RENDER_PARAM_API_TYPE, MPV_RENDER_API_TYPE_SW},
        {0}
    };
    return mpv_render_context_create(res, mpv, params);
}

// render_gl renders to an OpenGL FBO.
static int render_gl(mpv_render_context *ctx, int fbo, int w, int h,
                     int internal_format, int flip_y) {
    mpv_opengl_fbo fbo_param = {
        .fbo = fbo,
        .w = w,
        .h = h,
        .internal_format = internal_format,
    };
    int flip = flip_y;
    mpv_render_param params[] = {
        {MPV_RENDER_PARAM_OPENGL_FBO, &fbo_param},
        {MPV_RENDER_PARAM_FLIP_Y, &flip},
        {0}
    };
    return mpv_render_context_render(ctx, params);
}

// render_sw renders to a software buffer.
static int render_sw(mpv_render_context *ctx, int w, int h,
                     const char *format, size_t stride, void *ptr) {
    int size[2] = {w, h};
    mpv_render_param params[] = {
        {MPV_RENDER_PARAM_SW_SIZE, &size[0]},
        {MPV_RENDER_PARAM_SW_FORMAT, (void*)format},
        {MPV_RENDER_PARAM_SW_STRIDE, &stride},
        {MPV_RENDER_PARAM_SW_POINTER, ptr},
        {0}
    };
    return mpv_render_context_render(ctx, params);
}
*/
import "C"

import (
	"sync"
	"unsafe"
)

// RenderParamType corresponds to mpv_render_param_type.
type RenderParamType int

// Render parameter types.
const (
	RENDER_PARAM_API_TYPE           RenderParamType = C.MPV_RENDER_PARAM_API_TYPE
	RENDER_PARAM_OPENGL_INIT_PARAMS RenderParamType = C.MPV_RENDER_PARAM_OPENGL_INIT_PARAMS
	RENDER_PARAM_OPENGL_FBO         RenderParamType = C.MPV_RENDER_PARAM_OPENGL_FBO
	RENDER_PARAM_FLIP_Y             RenderParamType = C.MPV_RENDER_PARAM_FLIP_Y
	RENDER_PARAM_DEPTH              RenderParamType = C.MPV_RENDER_PARAM_DEPTH
	RENDER_PARAM_ADVANCED_CONTROL   RenderParamType = C.MPV_RENDER_PARAM_ADVANCED_CONTROL
	RENDER_PARAM_BLOCK_FOR_TARGET   RenderParamType = C.MPV_RENDER_PARAM_BLOCK_FOR_TARGET_TIME
	RENDER_PARAM_SKIP_RENDERING     RenderParamType = C.MPV_RENDER_PARAM_SKIP_RENDERING
	RENDER_PARAM_SW_SIZE            RenderParamType = C.MPV_RENDER_PARAM_SW_SIZE
	RENDER_PARAM_SW_FORMAT          RenderParamType = C.MPV_RENDER_PARAM_SW_FORMAT
	RENDER_PARAM_SW_STRIDE          RenderParamType = C.MPV_RENDER_PARAM_SW_STRIDE
	RENDER_PARAM_SW_POINTER         RenderParamType = C.MPV_RENDER_PARAM_SW_POINTER
)

// RenderUpdateFlag corresponds to mpv_render_update_flag.
type RenderUpdateFlag uint64

// Render update flags.
const (
	RENDER_UPDATE_FRAME RenderUpdateFlag = C.MPV_RENDER_UPDATE_FRAME
)

// RenderFrameInfoFlag corresponds to mpv_render_frame_info_flag.
type RenderFrameInfoFlag uint64

// Render frame info flags.
const (
	RENDER_FRAME_INFO_PRESENT    RenderFrameInfoFlag = C.MPV_RENDER_FRAME_INFO_PRESENT
	RENDER_FRAME_INFO_REDRAW     RenderFrameInfoFlag = C.MPV_RENDER_FRAME_INFO_REDRAW
	RENDER_FRAME_INFO_REPEAT     RenderFrameInfoFlag = C.MPV_RENDER_FRAME_INFO_REPEAT
	RENDER_FRAME_INFO_BLOCK_VSYNC RenderFrameInfoFlag = C.MPV_RENDER_FRAME_INFO_BLOCK_VSYNC
)

// OpenGLFBO describes an OpenGL framebuffer object for rendering.
type OpenGLFBO struct {
	// FBO is the framebuffer object name. 0 means the default framebuffer.
	FBO int
	// W and H are the dimensions of the framebuffer.
	W, H int
	// InternalFormat is the underlying texture internal format (e.g. GL_RGBA8),
	// or 0 if unknown.
	InternalFormat int
}

// RenderContext wraps mpv_render_context for embedded video rendering.
// It must be created before mpv_initialize and freed before mpv_terminate_destroy.
type RenderContext struct {
	ctx      *C.mpv_render_context
	updateCh chan struct{}
	cbID     uintptr
}

var renderCtxRegistry sync.Map
var renderCtxMu sync.Mutex
var nextRenderCtxID uint64

func registerRenderContext(rc *RenderContext) uintptr {
	renderCtxMu.Lock()
	nextRenderCtxID++
	id := nextRenderCtxID
	renderCtxMu.Unlock()
	renderCtxRegistry.Store(id, rc)
	return uintptr(id)
}

func unregisterRenderContext(id uintptr) {
	renderCtxRegistry.Delete(uint64(id))
}

// glProcAddrRegistry maps cgo handle values to Go get_proc_address functions.
// This is needed because cgo cannot pass Go function pointers to C directly.
var glProcAddrRegistry sync.Map

// nextGLProcAddrID generates unique IDs for the registry.
var nextGLProcAddrID uint64
var glProcAddrMu sync.Mutex

func registerGLProcAddr(fn func(string) unsafe.Pointer) uintptr {
	glProcAddrMu.Lock()
	nextGLProcAddrID++
	id := nextGLProcAddrID
	glProcAddrMu.Unlock()
	glProcAddrRegistry.Store(id, fn)
	return uintptr(id)
}

func unregisterGLProcAddr(id uintptr) {
	glProcAddrRegistry.Delete(uint64(id))
}

//export goGLGetProcAddress
func goGLGetProcAddress(ctx unsafe.Pointer, name *C.char) unsafe.Pointer {
	id := uint64(uintptr(ctx))
	if fn, ok := glProcAddrRegistry.Load(id); ok {
		return fn.(func(string) unsafe.Pointer)(C.GoString(name))
	}
	return nil
}

//export goRenderUpdateCallback
func goRenderUpdateCallback(ctx unsafe.Pointer) {
	id := uint64(uintptr(ctx))
	if v, ok := renderCtxRegistry.Load(id); ok {
		rc := v.(*RenderContext)
		select {
		case rc.updateCh <- struct{}{}:
		default:
		}
	}
}

// NewRenderContext creates a new OpenGL render context for embedded video rendering.
//
// The getProcAddress function should return OpenGL function pointers for the
// given function name (e.g., wrapping SDL_GL_GetProcAddress or glXGetProcAddress).
//
// The OpenGL context must be current on the calling thread when this function
// is called. advancedControl enables MPV_RENDER_PARAM_ADVANCED_CONTROL for
// better performance (direct rendering, GPU screenshots), but requires strict
// threading discipline - see render.h documentation.
//
// This should be called after Create() but before Initialize().
// The returned RenderContext must be freed with Free() before TerminateDestroy().
func (m *Mpv) NewRenderContext(getProcAddress func(name string) unsafe.Pointer, advancedControl bool) (*RenderContext, error) {
	rc := &RenderContext{
		updateCh: make(chan struct{}, 1),
	}

	adv := 0
	if advancedControl {
		adv = 1
	}

	id := registerGLProcAddr(getProcAddress)
	rc.cbID = registerRenderContext(rc)

	var ctx *C.mpv_render_context
	err := C.create_render_context_gl(
		&ctx,
		m.handle,
		C.uintptr_t(id),
		C.int(adv),
	)
	if err != C.MPV_ERROR_SUCCESS {
		unregisterGLProcAddr(id)
		unregisterRenderContext(rc.cbID)
		return nil, newError(err)
	}
	rc.ctx = ctx

	// Set update callback: fires when a new frame is available.
	C.set_update_callback_bridge(rc.ctx, C.uintptr_t(rc.cbID))

	return rc, nil
}

// NewSoftwareRenderContext creates a software render context.
// This renders to CPU memory buffers instead of GPU. It's slow but doesn't
// require OpenGL. Useful for thumbnails, screenshots, or headless rendering.
//
// This should be called after Create() but before Initialize().
func (m *Mpv) NewSoftwareRenderContext() (*RenderContext, error) {
	rc := &RenderContext{
		updateCh: make(chan struct{}, 1),
	}
	rc.cbID = registerRenderContext(rc)

	var ctx *C.mpv_render_context
	err := C.create_render_context_sw(&ctx, m.handle)
	if err != C.MPV_ERROR_SUCCESS {
		unregisterRenderContext(rc.cbID)
		return nil, newError(err)
	}
	rc.ctx = ctx

	C.set_update_callback_bridge(rc.ctx, C.uintptr_t(rc.cbID))

	return rc, nil
}

// Update checks whether a new frame is available for rendering.
// Returns a bitset of RenderUpdateFlag values (check with RENDER_UPDATE_FRAME).
//
// Must be called on the render thread (where the OpenGL context is current).
// Should be called after receiving a signal from WaitUpdate().
func (rc *RenderContext) Update() RenderUpdateFlag {
	return RenderUpdateFlag(C.mpv_render_context_update(rc.ctx))
}

// Render renders the current video frame to the specified OpenGL FBO.
//
// fbo: the target framebuffer (use FBO=0 for the default framebuffer).
// flipY: set to true when rendering to the default framebuffer (which has
// a flipped Y coordinate system in OpenGL).
//
// Must be called on the render thread (where the OpenGL context is current).
func (rc *RenderContext) Render(fbo OpenGLFBO, flipY bool) error {
	flip := C.int(0)
	if flipY {
		flip = 1
	}
	return newError(C.render_gl(
		rc.ctx,
		C.int(fbo.FBO),
		C.int(fbo.W),
		C.int(fbo.H),
		C.int(fbo.InternalFormat),
		flip,
	))
}

// RenderSW renders the current video frame to a CPU memory buffer.
//
// w, h: target surface dimensions.
// format: pixel format string, e.g. "rgb0", "bgr0", "0bgr", "0rgb", "rgb24".
// stride: bytes per line (must be >= w * bytes_per_pixel, ideally 64-byte aligned).
// ptr: pointer to the first pixel at top-left (0,0).
//
// Must be called on the render thread.
func (rc *RenderContext) RenderSW(w, h int, format string, stride int, ptr unsafe.Pointer) error {
	cFormat := C.CString(format)
	defer C.free(unsafe.Pointer(cFormat))
	return newError(C.render_sw(
		rc.ctx,
		C.int(w),
		C.int(h),
		cFormat,
		C.size_t(stride),
		ptr,
	))
}

// ReportSwap tells mpv that a frame was flipped/swapped. This is optional
// but helps the player achieve better A/V timing.
//
// Once called, you must call it consistently for every frame, or expect
// bad video playback.
func (rc *RenderContext) ReportSwap() {
	C.mpv_render_context_report_swap(rc.ctx)
}

// WaitUpdate returns a channel that receives a signal when a new frame is
// available or the display configuration changed. Use this in your render
// loop to avoid busy-waiting.
//
// Example:
//
//	for {
//	    <-rc.WaitUpdate()
//	    if rc.Update()&mpv.RENDER_UPDATE_FRAME != 0 {
//	        rc.Render(fbo, true)
//	        window.GLSwap()
//	        rc.ReportSwap()
//	    }
//	}
func (rc *RenderContext) WaitUpdate() <-chan struct{} {
	return rc.updateCh
}

// Free destroys the render context. Must be called before Mpv.TerminateDestroy().
// If video is still playing, it will be forcefully disabled.
// After this call, the RenderContext must not be used anymore.
func (rc *RenderContext) Free() {
	if rc.cbID != 0 {
		unregisterRenderContext(rc.cbID)
		rc.cbID = 0
	}
	if rc.ctx != nil {
		C.mpv_render_context_free(rc.ctx)
		rc.ctx = nil
	}
}
