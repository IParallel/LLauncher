//go:build windows

package main

import (
	"testing"
	"unsafe"
)

// The tray structs are hand-written mirrors of Win32 ones and their size IS the
// contract: NOTIFYICONDATAW and WNDCLASSEXW are both dispatched on cbSize, and a
// wrong one is rejected with no error anybody sees — the icon simply never
// appears, or the window class never registers, and the launcher hides itself
// into nothing. Pinning the sizes here turns a silent misfire into a failing test.
func TestWin32StructSizes(t *testing.T) {
	cases := []struct {
		name string
		got  uintptr
		want uintptr
	}{
		// x64 sizes from the Windows SDK.
		{"NOTIFYICONDATAW", unsafe.Sizeof(notifyIconData{}), 976},
		{"WNDCLASSEXW", unsafe.Sizeof(wndClassEx{}), 80},
		{"MSG", unsafe.Sizeof(winMsg{}), 48},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("sizeof(%s) = %d, want %d", c.name, c.got, c.want)
		}
	}
}
