package limbonia

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32DLL              = syscall.NewLazyDLL("kernel32.dll")
	procVirtualAllocEx       = kernel32DLL.NewProc("VirtualAllocEx")
	procWriteProcessMemory   = kernel32DLL.NewProc("WriteProcessMemory")
	procCreateRemoteThread   = kernel32DLL.NewProc("CreateRemoteThread")
	procCreateToolhelp32Snap = kernel32DLL.NewProc("CreateToolhelp32Snapshot")
	procProcess32FirstW      = kernel32DLL.NewProc("Process32FirstW")
	procProcess32NextW       = kernel32DLL.NewProc("Process32NextW")
	procLoadLibraryA         = kernel32DLL.NewProc("LoadLibraryA")
	procGetExitCodeThread    = kernel32DLL.NewProc("GetExitCodeThread")
)

const (
	PROCESS_ALL_ACCESS = 0x1F0FFF
	MEM_COMMIT         = 0x1000
	MEM_RESERVE        = 0x2000
	PAGE_READWRITE     = 0x04
	TH32CS_SNAPPROCESS = 0x00000002
	MAX_PATH           = 260
)

type ProcessEntry32 struct {
	Size              uint32
	CntUsage          uint32
	ProcessID         uint32
	DefaultHeapID     uintptr
	ModuleID          uint32
	CntThreads        uint32
	ParentProcessID   uint32
	PriorityClassBase int32
	Flags             uint32
	ExeFile           [MAX_PATH]uint16
}

func FindProcessID(processName string) (uint32, error) {
	snapshot, _, err := procCreateToolhelp32Snap.Call(uintptr(TH32CS_SNAPPROCESS), 0)
	if snapshot == 0 {
		return 0, fmt.Errorf("CreateToolhelp32Snapshot failed: %v", err)
	}
	defer syscall.CloseHandle(syscall.Handle(snapshot))

	var entry ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))

	ret, _, _ := procProcess32FirstW.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
	if ret == 0 {
		return 0, fmt.Errorf("Process32FirstW failed")
	}

	for {
		exeFile := windows.UTF16ToString(entry.ExeFile[:])
		if strings.EqualFold(exeFile, processName) {
			return entry.ProcessID, nil
		}

		ret, _, _ := procProcess32NextW.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
		if ret == 0 {
			break
		}
	}

	return 0, fmt.Errorf("Process not found: %s", processName)
}

func injectDLL(pid uint32, dllPath string) error {
	hProcess, err := windows.OpenProcess(PROCESS_ALL_ACCESS, false, pid)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(hProcess)

	dllPathBytes := append([]byte(dllPath), 0)

	remoteAddr, _, err := procVirtualAllocEx.Call(
		uintptr(hProcess),
		0,
		uintptr(len(dllPathBytes)),
		uintptr(MEM_COMMIT|MEM_RESERVE),
		uintptr(PAGE_READWRITE),
	)
	if remoteAddr == 0 {
		return fmt.Errorf("VirtualAllocEx failed: %v", err)
	}
	fmt.Println("[+] Allocated memory at:", remoteAddr)

	var written uintptr
	ret, _, err := procWriteProcessMemory.Call(
		uintptr(hProcess),
		remoteAddr,
		uintptr(unsafe.Pointer(&dllPathBytes[0])),
		uintptr(len(dllPathBytes)),
		uintptr(unsafe.Pointer(&written)),
	)
	if ret == 0 || written == 0 {
		return fmt.Errorf("WriteProcessMemory failed: %v", err)
	}
	fmt.Println("[+] Wrote DLL path:", dllPath)

	thread, _, err := procCreateRemoteThread.Call(
		uintptr(hProcess),
		0,
		0,
		procLoadLibraryA.Addr(),
		remoteAddr,
		0,
		0,
	)
	if thread == 0 {
		return fmt.Errorf("CreateRemoteThread failed: %v", err)
	}
	fmt.Println("[+] Remote thread created:", thread)

	_, err = windows.WaitForSingleObject(windows.Handle(thread), windows.INFINITE)
	if err != nil {
		return fmt.Errorf("WaitForSingleObject failed: %v", err)
	}

	var exitCode uint32
	ret, _, err = procGetExitCodeThread.Call(
		thread,
		uintptr(unsafe.Pointer(&exitCode)),
	)
	if ret == 0 {
		return fmt.Errorf("failed to get exit code: %v", err)
	}

	if exitCode == 0 {
		return fmt.Errorf("injection failed with exit code %d", exitCode)
	}

	fmt.Printf("[+] remote LoadLibrary returned: 0x%x (success)\n", exitCode)

	return nil
}

func InjectLimbo() error {

	pid, err := FindProcessID("LimbusCompany.exe")
	if err != nil {
		fmt.Printf("FindProcessID failed: %v", err)
		return err
	}

	name, _ := filepath.Abs(filepath.Join("./", "Limbonia", "Limbonia.dll"))

	fmt.Println(name)

	if _, err := os.Stat(name); err != nil {
		return err
	}

	if err := injectDLL(pid, name); err != nil {
		fmt.Printf("injectDLL failed: %v", err)
		return err
	}

	fmt.Println("injectDLL success")

	return nil
}
