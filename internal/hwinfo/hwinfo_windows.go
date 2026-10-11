// Package hwinfo finds, inspects and launches the HWiNFO program itself.
package hwinfo

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

const processQueryLimitedInformation = 0x1000

var (
	kernel32                  = syscall.NewLazyDLL("kernel32.dll")
	procQueryFullProcessImage = kernel32.NewProc("QueryFullProcessImageNameW")
	shell32                   = syscall.NewLazyDLL("shell32.dll")
	procShellExecute          = shell32.NewProc("ShellExecuteW")
)

var exeNames = []string{"hwinfo64.exe", "hwinfo32.exe", "hwinfo_arm64.exe"}

// FindRunning returns the executable path of a running HWiNFO, or "".
func FindRunning() string {
	snap, err := syscall.CreateToolhelp32Snapshot(syscall.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return ""
	}
	defer syscall.CloseHandle(snap)
	var e syscall.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	for err = syscall.Process32First(snap, &e); err == nil; err = syscall.Process32Next(snap, &e) {
		name := strings.ToLower(syscall.UTF16ToString(e.ExeFile[:]))
		for _, n := range exeNames {
			if name == n {
				if p := imagePath(e.ProcessID); p != "" {
					return p
				}
			}
		}
	}
	return ""
}

func imagePath(pid uint32) string {
	h, err := syscall.OpenProcess(processQueryLimitedInformation, false, pid)
	if err != nil {
		return ""
	}
	defer syscall.CloseHandle(h)
	buf := make([]uint16, 1024)
	size := uint32(len(buf))
	r, _, _ := procQueryFullProcessImage.Call(uintptr(h), 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)))
	if r == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf[:size])
}

// IsRunning reports whether any HWiNFO process exists.
func IsRunning() bool {
	snap, err := syscall.CreateToolhelp32Snapshot(syscall.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return false
	}
	defer syscall.CloseHandle(snap)
	var e syscall.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	for err = syscall.Process32First(snap, &e); err == nil; err = syscall.Process32Next(snap, &e) {
		name := strings.ToLower(syscall.UTF16ToString(e.ExeFile[:]))
		for _, n := range exeNames {
			if name == n {
				return true
			}
		}
	}
	return false
}

// Launch starts HWiNFO through the shell so Windows shows the UAC prompt its manifest requires.
func Launch(exe string) error {
	return ShellOpen(exe, filepath.Dir(exe))
}

// ShellOpen opens a file or URL with its default handler.
func ShellOpen(target, dir string) error {
	verb, _ := syscall.UTF16PtrFromString("open")
	file, _ := syscall.UTF16PtrFromString(target)
	var wd *uint16
	if dir != "" {
		wd, _ = syscall.UTF16PtrFromString(dir)
	}
	r, _, _ := procShellExecute.Call(0, uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(file)), 0, uintptr(unsafe.Pointer(wd)), 1)
	if r <= 32 {
		return syscall.Errno(r)
	}
	return nil
}

// DarkMode reads the color mode from HWiNFO64.INI beside exe. HWiNFO writes
// DarkMode=0 for light; a missing key or any other value means dark.
func DarkMode(exe string) (dark bool, ok bool) {
	ini := strings.TrimSuffix(exe, filepath.Ext(exe)) + ".INI"
	f, err := os.Open(ini)
	if err != nil {
		return false, false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		k, v, found := strings.Cut(strings.TrimSpace(sc.Text()), "=")
		if found && strings.EqualFold(strings.TrimSpace(k), "DarkMode") {
			return strings.TrimSpace(v) != "0", true
		}
	}
	return true, true
}
