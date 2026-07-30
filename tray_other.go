//go:build !windows

package main

// No tray on Linux, deliberately — this is shipping behaviour, not a stub.
//
// The launcher has a real Linux build (.github/workflows/release.yml uploads
// LLauncher-linux), so what happens here is what Linux users get: the window
// never hides, and simply stays open for as long as Mephi runs. That is a
// downgrade from the Windows behaviour and it is the correct one, because the
// alternative is worse than doing nothing.
//
// The plainest reason is simply that there is less to gain. Hiding is a nicety —
// getting a window that has nothing left to say out of the way — and a desktop
// with workspaces and a proper taskbar already makes a spare window cheap to
// ignore. What is given up here is small.
//
// The reasons not to reach for it anyway are more concrete. The modern Linux tray
// is StatusNotifierItem over DBus, and three things make it a bad trade:
//
//   - There is no guarantee a tray exists at all. KDE, XFCE, Cinnamon and MATE
//     host SNI; GNOME — the most common desktop by a wide margin — has none
//     without the AppIndicator extension installed. On those systems a hidden
//     window would vanish with nothing to click, recoverable only by killing the
//     process. Hiding must never be a coin flip.
//
//   - Detecting that safely means asking org.kde.StatusNotifierWatcher whether
//     IsStatusNotifierHostRegistered is true. fyne.io/systray does not expose it —
//     its onReady fires whether or not any host picked the item up — so the
//     library cannot answer the one question that would make hiding safe.
//
//   - Showing the icon only while hidden, as the Windows tray does, maps onto
//     systray.Run/Quit cycling, which that library does not properly support. A
//     raw SNI implementation instead means hand-exporting an SNI object AND a
//     com.canonical.dbusmenu tree — a large, desktop-sensitive surface that
//     nothing in this repo's test suite could exercise.
//
// So newTray returns nil, presence.hide() refuses, and the window stays put. The
// exit watcher is therefore never started on this platform, which is correct
// rather than merely tolerable: its only job is to undo a hide that never
// happened. Nothing calls WindowShow either, so nothing steals focus.
//
// If this is revisited, the order matters: probe the SNI watcher for a registered
// host FIRST, and keep returning nil when there is none. That fallback is the
// feature, not the failure.
type trayIcon struct{}

func newTray(tooltip string, onShow, onExit func()) *trayIcon { return nil }

// show reports false, which is precisely what stops the window ever being hidden
// on a platform where it might not be recoverable.
func (t *trayIcon) show() bool { return false }

func (t *trayIcon) hide() {}

func (t *trayIcon) destroy() {}
