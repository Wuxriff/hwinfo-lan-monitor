package gui

import (
	"syscall"
	"unsafe"

	"github.com/rodrigocfd/windigo/win"
)

var (
	user32               = syscall.NewLazyDLL("user32.dll")
	procFindWindow       = user32.NewProc("FindWindowW")
	procCreateSolidBrush = syscall.NewLazyDLL("gdi32.dll").NewProc("CreateSolidBrush")
	procDwmSetAttr       = syscall.NewLazyDLL("dwmapi.dll").NewProc("DwmSetWindowAttribute")
	uxtheme              = syscall.NewLazyDLL("uxtheme.dll")
	procSetWindowTheme   = uxtheme.NewProc("SetWindowTheme")
	procCreateMutex      = syscall.NewLazyDLL("kernel32.dll").NewProc("CreateMutexW")
	procUserUILanguage   = syscall.NewLazyDLL("kernel32.dll").NewProc("GetUserDefaultUILanguage")
	procAppendMenu       = user32.NewProc("AppendMenuW")
)

const (
	dwmwaUseImmersiveDarkMode = 20
	errorAlreadyExists        = syscall.Errno(183)
	mfString                  = 0x0000
	mfSeparator               = 0x0800
	langRussian               = 0x19
)

func init() {
	// Undocumented uxtheme exports (ordinals 135/136) that switch popup menus to dark; absent on old builds.
	h, err := syscall.LoadLibrary("uxtheme.dll")
	if err != nil {
		return
	}
	getProc := syscall.NewLazyDLL("kernel32.dll").NewProc("GetProcAddress")
	preferredModeAddr, _, _ = getProc.Call(uintptr(h), 135)
	flushMenuThemesAddr, _, _ = getProc.Call(uintptr(h), 136)
}

var preferredModeAddr, flushMenuThemesAddr uintptr

func setMenusDark(dark bool) {
	if preferredModeAddr == 0 {
		return
	}
	mode := uintptr(3) // ForceLight
	if dark {
		mode = 2 // ForceDark
	}
	syscall.SyscallN(preferredModeAddr, mode)
	if flushMenuThemesAddr != 0 {
		syscall.SyscallN(flushMenuThemesAddr)
	}
}

func utf16(s string) *uint16 {
	if s == "" {
		return nil
	}
	p, _ := syscall.UTF16PtrFromString(s)
	return p
}

func findWindow(className string) win.HWND {
	h, _, _ := procFindWindow.Call(uintptr(unsafe.Pointer(utf16(className))), 0)
	return win.HWND(h)
}

func solidBrush(c win.COLORREF) win.HBRUSH {
	h, _, _ := procCreateSolidBrush.Call(uintptr(c))
	return win.HBRUSH(h)
}

func setTitleBarDark(hwnd win.HWND, dark bool) {
	v := uint32(0)
	if dark {
		v = 1
	}
	procDwmSetAttr.Call(uintptr(hwnd), dwmwaUseImmersiveDarkMode, uintptr(unsafe.Pointer(&v)), 4)
}

// setTheme applies a visual style class; empty app and id strip theming entirely.
func setTheme(hwnd win.HWND, app string, strip bool) {
	if strip {
		empty := []uint16{0}
		procSetWindowTheme.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&empty[0])), uintptr(unsafe.Pointer(&empty[0])))
		return
	}
	procSetWindowTheme.Call(uintptr(hwnd), uintptr(unsafe.Pointer(utf16(app))), 0)
}

// singleInstance returns false when another Host app already runs in this session.
// An elevated instance's mutex is not accessible to a normal one, which reports access denied instead.
func singleInstance() bool {
	h, _, err := procCreateMutex.Call(0, 0, uintptr(unsafe.Pointer(utf16(`Local\HWiNFOLanMonitor.Instance`))))
	return h != 0 && err != errorAlreadyExists
}

func windowsLanguage() string {
	id, _, _ := procUserUILanguage.Call()
	if id&0x3FF == langRussian {
		return "ru"
	}
	return "en"
}

func appendMenu(menu win.HMENU, id int, text string) {
	if text == "" {
		procAppendMenu.Call(uintptr(menu), mfSeparator, 0, 0)
		return
	}
	procAppendMenu.Call(uintptr(menu), mfString, uintptr(id), uintptr(unsafe.Pointer(utf16(text))))
}

// smallIcon loads an icon resource at the small (tray, title bar) size so Windows does not downscale a 32 px one.
func smallIcon(id int) win.HICON { return iconOfSize(id, 49) } // SM_CXSMICON

func iconOfSize(id int, metric uintptr) win.HICON {
	hinst, _, _ := procGetModuleHandle.Call(0)
	cx, _, _ := procGetSystemMetrics.Call(metric)
	h, _, _ := procLoadImage.Call(hinst, uintptr(id), 1 /* IMAGE_ICON */, cx, cx, 0)
	return win.HICON(h)
}

var (
	procLoadImage        = user32.NewProc("LoadImageW")
	procGetSystemMetrics = user32.NewProc("GetSystemMetrics")
	procGetModuleHandle  = syscall.NewLazyDLL("kernel32.dll").NewProc("GetModuleHandleW")
)
