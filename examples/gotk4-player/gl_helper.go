package main

/*
#cgo LDFLAGS: -ldl
#include <locale.h>
#include <stdlib.h>
#include <dlfcn.h>
#include <stdbool.h>

// 获取 OpenGL 函数指针的跨平台加载器
void* get_gl_proc_address(const char* name) {
    static void* handle = NULL;
    static void* egl_handle = NULL;
    static bool initialized = false;
    
    if (!initialized) {
        initialized = true;
        handle = dlopen("libGL.so.1", RTLD_LAZY | RTLD_GLOBAL);
        if (!handle) {
            handle = dlopen("libOpenGL.so.0", RTLD_LAZY | RTLD_GLOBAL);
        }
        egl_handle = dlopen("libEGL.so.1", RTLD_LAZY | RTLD_GLOBAL);
    }
    
    void* symbol = NULL;
    if (handle) {
        symbol = dlsym(handle, name);
    }
    if (!symbol && handle) {
        void* (*glx_gpa)(const unsigned char*) = (void*(*)(const unsigned char*))dlsym(handle, "glXGetProcAddress");
        if (glx_gpa) {
            symbol = glx_gpa((const unsigned char*)name);
        }
    }
    if (!symbol && egl_handle) {
        void* (*egl_gpa)(const char*) = (void*(*)(const char*))dlsym(egl_handle, "eglGetProcAddress");
        if (egl_gpa) {
            symbol = egl_gpa(name);
        }
    }
    return symbol;
}

// 获取 GTK4 隐藏的 Framebuffer ID
int get_current_fbo() {
    // GL_DRAW_FRAMEBUFFER_BINDING = 0x8CA6
    void (*gl_get_integerv)(unsigned int, int*) = (void (*)(unsigned int, int*))get_gl_proc_address("glGetIntegerv");
    if (!gl_get_integerv) {
        return 0;
    }
    int fbo = 0;
    gl_get_integerv(0x8CA6, &fbo);
    return fbo;
}

// 帮助修复 Locale 冲突
void fix_mpv_locale() {
    setlocale(LC_NUMERIC, "C");
}
*/
import "C"
import "unsafe"

// getProcAddress wraps the C loader to return OpenGL function pointers for mpv.
func getProcAddress(name string) unsafe.Pointer {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))
	return C.get_gl_proc_address(cName)
}

// getCurrentFBO returns the current Framebuffer bound by GtkGLArea.
func getCurrentFBO() int {
	return int(C.get_current_fbo())
}

// fixLocale overrides the locale so libmpv does not crash upon initialization.
func fixLocale() {
	C.fix_mpv_locale()
}
