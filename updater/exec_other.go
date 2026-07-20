//go:build !windows

package updater

import "os"

// markExecutable makes an extracted file runnable.
//
// Go's archive/zip hands the mode back through FileInfo, but nothing applies it:
// os.Create makes 0666&^umask, so a Mephi binary comes out of the archive
// non-executable and fails at exec with "permission denied". 0755 rather than
// 0750 so the binary still runs if the install directory is later shared or the
// launcher and the game end up under different accounts.
func markExecutable(path string) error {
	return os.Chmod(path, 0o755)
}
