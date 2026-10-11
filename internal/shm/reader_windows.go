package shm

import (
	"errors"
	"fmt"
	"syscall"
	"unsafe"
)

const (
	MappingName = `Global\HWiNFO_SENS_SM2`
	MutexName   = `Global\HWiNFO_SM2_MUTEX`

	fileMapRead    = 0x0004
	synchronize    = 0x00100000
	mutexWaitMs    = 10
	waitObject0    = 0
	waitAbandoned  = 0x80
	errFileMissing = syscall.Errno(2)
)

// ErrNotAvailable means HWiNFO is not running or Shared Memory Support is off.
var ErrNotAvailable = errors.New("HWiNFO shared memory not found")

var (
	kernel32            = syscall.NewLazyDLL("kernel32.dll")
	procOpenFileMapping = kernel32.NewProc("OpenFileMappingW")
	procOpenMutex       = kernel32.NewProc("OpenMutexW")
	procReleaseMutex    = kernel32.NewProc("ReleaseMutex")
	procVirtualQuery    = kernel32.NewProc("VirtualQuery")
)

type memoryBasicInformation struct {
	BaseAddress       uintptr
	AllocationBase    uintptr
	AllocationProtect uint32
	PartitionId       uint16
	RegionSize        uintptr
	State             uint32
	Protect           uint32
	Type              uint32
}

// ReadRaw copies the whole HWiNFO shared memory view. The mapping is opened and
// closed on every call because HWiNFO re-creates it when the sensor set changes.
func ReadRaw() ([]byte, error) {
	name, _ := syscall.UTF16PtrFromString(MappingName)
	h, _, callErr := procOpenFileMapping.Call(fileMapRead, 0, uintptr(unsafe.Pointer(name)))
	if h == 0 {
		if errors.Is(callErr, errFileMissing) {
			return nil, ErrNotAvailable
		}
		return nil, fmt.Errorf("OpenFileMapping: %w", callErr)
	}
	defer syscall.CloseHandle(syscall.Handle(h))

	addr, err := syscall.MapViewOfFile(syscall.Handle(h), fileMapRead, 0, 0, 0)
	if err != nil {
		return nil, fmt.Errorf("MapViewOfFile: %w", err)
	}
	defer syscall.UnmapViewOfFile(addr)

	var mbi memoryBasicInformation
	if r, _, err := procVirtualQuery.Call(addr, uintptr(unsafe.Pointer(&mbi)), unsafe.Sizeof(mbi)); r == 0 {
		return nil, fmt.Errorf("VirtualQuery: %w", err)
	}
	view := unsafe.Slice(*(**byte)(unsafe.Pointer(&addr)), mbi.RegionSize)

	unlock := lockMutex()
	defer unlock()

	n, err := Size(view)
	if err != nil {
		return nil, err
	}
	if n > len(view) {
		return nil, fmt.Errorf("%w: view %d bytes, header needs %d", ErrTruncated, len(view), n)
	}
	return append([]byte(nil), view[:n]...), nil
}

// lockMutex takes HWiNFO's mutex when it is accessible. It is advisory: a reader
// that cannot open it (e.g. HWiNFO elevated, we are not) still gets a usable copy.
func lockMutex() func() {
	name, _ := syscall.UTF16PtrFromString(MutexName)
	h, _, _ := procOpenMutex.Call(synchronize, 0, uintptr(unsafe.Pointer(name)))
	if h == 0 {
		return func() {}
	}
	ev, _ := syscall.WaitForSingleObject(syscall.Handle(h), mutexWaitMs)
	if ev != waitObject0 && ev != waitAbandoned {
		syscall.CloseHandle(syscall.Handle(h))
		return func() {}
	}
	return func() {
		procReleaseMutex.Call(h)
		syscall.CloseHandle(syscall.Handle(h))
	}
}

// Read returns a parsed snapshot of HWiNFO shared memory.
func Read() (*Snapshot, error) {
	b, err := ReadRaw()
	if err != nil {
		return nil, err
	}
	return Parse(b)
}
