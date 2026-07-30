//go:build windows

package main

import (
	"os"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The tray is raw Shell_NotifyIcon rather than a systray library.
//
// Every Go systray package (getlantern/systray, fyne.io/systray) wants Run() on
// the main goroutine — that is the documented contract and it is load-bearing on
// macOS. Wails already owns the main goroutine for the whole life of the process:
// wails.Run blocks there and pumps the window's message loop. There is no way to
// give both of them the same thread, and the libraries' "external loop" escape
// hatches assume they can drive the host loop, which Wails does not expose.
//
// What is left of a systray library once it cannot own the loop is one hidden
// window, one Shell_NotifyIconW call and a popup menu — which is this file, with
// no new dependency and no temp .ico file written to disk (both Windows systray
// backends require the icon as bytes on disk). golang.org/x/sys/windows was
// already in go.mod for the injector.
var (
	user32DLL = windows.NewLazySystemDLL("user32.dll")
	shellDLL  = windows.NewLazySystemDLL("shell32.dll")
	kernelDLL = windows.NewLazySystemDLL("kernel32.dll")

	procRegisterClassExW    = user32DLL.NewProc("RegisterClassExW")
	procCreateWindowExW     = user32DLL.NewProc("CreateWindowExW")
	procDefWindowProcW      = user32DLL.NewProc("DefWindowProcW")
	procGetMessageW         = user32DLL.NewProc("GetMessageW")
	procTranslateMessage    = user32DLL.NewProc("TranslateMessage")
	procDispatchMessageW    = user32DLL.NewProc("DispatchMessageW")
	procSendMessageW        = user32DLL.NewProc("SendMessageW")
	procPostMessageW        = user32DLL.NewProc("PostMessageW")
	procDestroyWindow       = user32DLL.NewProc("DestroyWindow")
	procPostQuitMessage     = user32DLL.NewProc("PostQuitMessage")
	procCreatePopupMenu     = user32DLL.NewProc("CreatePopupMenu")
	procAppendMenuW         = user32DLL.NewProc("AppendMenuW")
	procTrackPopupMenu      = user32DLL.NewProc("TrackPopupMenu")
	procDestroyMenu         = user32DLL.NewProc("DestroyMenu")
	procSetForegroundWindow = user32DLL.NewProc("SetForegroundWindow")
	procGetCursorPos        = user32DLL.NewProc("GetCursorPos")
	procLoadIconW           = user32DLL.NewProc("LoadIconW")
	procShellNotifyIconW    = shellDLL.NewProc("Shell_NotifyIconW")
	procExtractIconW        = shellDLL.NewProc("ExtractIconW")
	procGetModuleHandleW    = kernelDLL.NewProc("GetModuleHandleW")
)

const (
	wmNull    = 0x0000
	wmDestroy = 0x0002
	wmCommand = 0x0111

	wmLButtonUp     = 0x0202
	wmLButtonDblClk = 0x0203
	wmRButtonUp     = 0x0205

	wmApp = 0x8000

	// trayCallback is what the shell sends us for mouse activity on the icon; the
	// rest are our own, posted in from other goroutines so that every Win32 call
	// still happens on the thread that owns the window.
	trayCallback    = wmApp + 1
	trayMsgShowIcon = wmApp + 2
	trayMsgHideIcon = wmApp + 3
	trayMsgQuit     = wmApp + 4

	nimAdd    = 0
	nimDelete = 2

	nifMessage = 0x01
	nifIcon    = 0x02
	nifTip     = 0x04

	menuIDShow = 1
	menuIDExit = 2

	mfString = 0x0000

	tpmRightButton = 0x0002
	tpmNoNotify    = 0x0080
	tpmReturnCmd   = 0x0100

	idiApplication = 32512

	trayClassName = "LLauncherTrayWindow"
	trayIconUID   = 1
)

type point struct {
	x, y int32
}

// winMsg mirrors MSG. lPrivate is present so the struct is not smaller than the
// one GetMessageW writes into.
type winMsg struct {
	hwnd     uintptr
	message  uint32
	wParam   uintptr
	lParam   uintptr
	time     uint32
	pt       point
	lPrivate uint32
}

type wndClassEx struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     windows.Handle
	hIcon         windows.Handle
	hCursor       windows.Handle
	hbrBackground windows.Handle
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       windows.Handle
}

// notifyIconData mirrors NOTIFYICONDATAW at its current (Vista+) size.
type notifyIconData struct {
	cbSize           uint32
	hWnd             uintptr
	uID              uint32
	uFlags           uint32
	uCallbackMessage uint32
	hIcon            windows.Handle
	szTip            [128]uint16
	dwState          uint32
	dwStateMask      uint32
	szInfo           [256]uint16
	uVersion         uint32
	szInfoTitle      [64]uint16
	dwInfoFlags      uint32
	guidItem         windows.GUID
	hBalloonIcon     windows.Handle
}

// trayIcon is the launcher's notification-area presence.
//
// There is only ever one, which is why the window procedure can reach it through
// a package variable: a C callback gets no Go receiver, and threading one through
// window extra bytes buys nothing when the count is fixed at one.
type trayIcon struct {
	hwnd    uintptr
	icon    windows.Handle
	tooltip string

	onShow func()
	onExit func()

	// visible is only ever touched on the tray thread, inside the window
	// procedure, so it needs no lock.
	visible bool

	ready chan struct{}
}

var (
	activeTray     *trayIcon
	trayWndProcPtr = syscall.NewCallback(trayWndProc)
)

// newTray creates the hidden window and its message loop, and returns nil when
// that could not be done.
//
// A nil return is meaningful to the caller: hiding the launcher without a tray
// icon would leave a window with no way back, so the hide is skipped entirely
// rather than performed on faith.
func newTray(tooltip string, onShow, onExit func()) *trayIcon {
	t := &trayIcon{
		tooltip: tooltip,
		onShow:  onShow,
		onExit:  onExit,
		ready:   make(chan struct{}),
	}

	go t.run()
	<-t.ready

	if t.hwnd == 0 {
		return nil
	}
	return t
}

// run owns the tray window for its whole lifetime.
//
// A window belongs to the thread that created it and only that thread may pump
// its messages, so this goroutine is pinned. It cannot be the main thread —
// wails.Run is already sitting there running the UI's own loop.
func (t *trayIcon) run() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	defer func() {
		// Unblock newTray even if class registration or window creation failed
		// below; hwnd stays zero and the caller treats that as "no tray".
		select {
		case <-t.ready:
		default:
			close(t.ready)
		}
	}()

	hInst, _, _ := procGetModuleHandleW.Call(0)

	className, err := windows.UTF16PtrFromString(trayClassName)
	if err != nil {
		return
	}
	title, err := windows.UTF16PtrFromString(t.tooltip)
	if err != nil {
		return
	}

	// Published before the window exists: CreateWindowExW dispatches WM_CREATE and
	// friends synchronously, so the procedure can run before the call returns.
	activeTray = t

	wc := wndClassEx{
		lpfnWndProc:   trayWndProcPtr,
		hInstance:     windows.Handle(hInst),
		lpszClassName: className,
	}
	wc.cbSize = uint32(unsafe.Sizeof(wc))

	if atom, _, _ := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); atom == 0 {
		return
	}

	// A real top-level window, never shown, rather than a message-only one:
	// SetForegroundWindow is required to make a tray context menu dismiss when the
	// user clicks away, and it does not work on HWND_MESSAGE windows.
	hwnd, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(title)),
		0, // no WS_VISIBLE
		0, 0, 0, 0,
		0, 0, hInst, 0,
	)
	if hwnd == 0 {
		return
	}

	t.hwnd = hwnd
	t.icon = loadAppIcon(windows.Handle(hInst))
	close(t.ready)

	var m winMsg
	for {
		r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		// 0 is WM_QUIT, -1 is an error; neither leaves anything to pump.
		if int32(r) <= 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
}

// show puts the icon in the notification area, reporting whether it is there.
//
// Sent rather than posted so the answer is real: the caller hides the launcher
// window on the strength of it, and a fire-and-forget post could only ever return
// an optimistic true.
func (t *trayIcon) show() bool {
	if t == nil || t.hwnd == 0 {
		return false
	}
	r, _, _ := procSendMessageW.Call(t.hwnd, trayMsgShowIcon, 0, 0)
	return r != 0
}

// hide removes the icon. Safe to call when it was never shown.
func (t *trayIcon) hide() {
	if t == nil || t.hwnd == 0 {
		return
	}
	procSendMessageW.Call(t.hwnd, trayMsgHideIcon, 0, 0)
}

// destroy tears the whole thing down at shutdown.
//
// Worth doing rather than leaving to process exit: a tray icon whose owner has
// gone lingers in the notification area until something makes Windows notice,
// which for the user is a ghost icon that does nothing when clicked.
func (t *trayIcon) destroy() {
	if t == nil || t.hwnd == 0 {
		return
	}
	procSendMessageW.Call(t.hwnd, trayMsgQuit, 0, 0)
}

func trayWndProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	t := activeTray

	switch message {
	case trayMsgShowIcon:
		if t == nil {
			return 0
		}
		if t.addIcon() {
			return 1
		}
		return 0

	case trayMsgHideIcon:
		if t != nil {
			t.removeIcon()
		}
		return 1

	case trayMsgQuit:
		if t != nil {
			t.removeIcon()
		}
		procDestroyWindow.Call(hwnd)
		return 1

	case trayCallback:
		if t == nil {
			return 0
		}
		// The mouse message the shell is relaying is in the low word of lParam.
		switch uint32(lParam) & 0xffff {
		case wmLButtonUp, wmLButtonDblClk:
			t.invoke(t.onShow)
		case wmRButtonUp:
			t.showMenu()
		}
		return 0

	case wmCommand:
		// TPM_RETURNCMD means the menu hands its result straight back to
		// showMenu, so this only catches anything that arrives another way.
		if t != nil {
			switch wParam & 0xffff {
			case menuIDShow:
				t.invoke(t.onShow)
			case menuIDExit:
				t.invoke(t.onExit)
			}
		}
		return 0

	case wmDestroy:
		procPostQuitMessage.Call(0)
		return 0
	}

	r, _, _ := procDefWindowProcW.Call(hwnd, uintptr(message), wParam, lParam)
	return r
}

func (t *trayIcon) iconData(flags uint32) notifyIconData {
	data := notifyIconData{
		hWnd:             t.hwnd,
		uID:              trayIconUID,
		uFlags:           flags,
		uCallbackMessage: trayCallback,
		hIcon:            t.icon,
	}
	data.cbSize = uint32(unsafe.Sizeof(data))

	tip := windows.StringToUTF16(t.tooltip)
	if len(tip) > len(data.szTip) {
		tip = tip[:len(data.szTip)]
		tip[len(tip)-1] = 0
	}
	copy(data.szTip[:], tip)

	return data
}

func (t *trayIcon) addIcon() bool {
	if t.visible {
		return true
	}
	data := t.iconData(nifMessage | nifIcon | nifTip)
	r, _, _ := procShellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&data)))
	t.visible = r != 0
	return t.visible
}

func (t *trayIcon) removeIcon() {
	if !t.visible {
		return
	}
	data := t.iconData(0)
	procShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&data)))
	t.visible = false
}

// showMenu pops the Show/Exit menu at the cursor.
//
// Minimal on purpose, but never empty: while the launcher is hidden this menu is
// the only interface it has, so it must always be able to bring the window back
// and to quit.
func (t *trayIcon) showMenu() {
	menu, _, _ := procCreatePopupMenu.Call()
	if menu == 0 {
		return
	}
	defer procDestroyMenu.Call(menu)

	show, err := windows.UTF16PtrFromString("Show LLauncher")
	if err != nil {
		return
	}
	exit, err := windows.UTF16PtrFromString("Exit")
	if err != nil {
		return
	}
	procAppendMenuW.Call(menu, mfString, menuIDShow, uintptr(unsafe.Pointer(show)))
	procAppendMenuW.Call(menu, mfString, menuIDExit, uintptr(unsafe.Pointer(exit)))

	var pt point
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))

	// Both of these are the documented workaround for tray menus: without the
	// foreground switch the menu never closes when the user clicks elsewhere, and
	// without the trailing message it can stay up after a selection.
	procSetForegroundWindow.Call(t.hwnd)

	cmd, _, _ := procTrackPopupMenu.Call(
		menu,
		tpmRightButton|tpmReturnCmd|tpmNoNotify,
		uintptr(pt.x), uintptr(pt.y),
		0, t.hwnd, 0,
	)

	procPostMessageW.Call(t.hwnd, wmNull, 0, 0)

	switch cmd {
	case menuIDShow:
		t.invoke(t.onShow)
	case menuIDExit:
		t.invoke(t.onExit)
	}
}

// invoke runs a handler off the tray thread.
//
// The handlers call into the Wails runtime, which marshals onto the UI thread and
// waits for it. Running that inside the window procedure would stop pumping this
// window's messages for the duration — and if the UI thread ever came back to the
// tray, both would be waiting on each other.
func (t *trayIcon) invoke(fn func()) {
	if fn == nil {
		return
	}
	go fn()
}

// loadAppIcon returns the launcher's own icon, falling back to the generic
// application one.
//
// Pulled out of the executable rather than looked up by resource id: the id Wails
// assigns is not part of any contract, and a wrong guess yields a blank square in
// the tray with no error to notice.
func loadAppIcon(hInst windows.Handle) windows.Handle {
	if exe, err := os.Executable(); err == nil {
		if p, err := windows.UTF16PtrFromString(exe); err == nil {
			// ExtractIconW returns 1 to mean "no icons in this file", so only a
			// value above that is a real handle.
			if h, _, _ := procExtractIconW.Call(uintptr(hInst), uintptr(unsafe.Pointer(p)), 0); h > 1 {
				return windows.Handle(h)
			}
		}
	}
	h, _, _ := procLoadIconW.Call(0, idiApplication)
	return windows.Handle(h)
}
