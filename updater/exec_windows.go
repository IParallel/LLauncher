//go:build windows

package updater

// markExecutable is a no-op on Windows, where executability comes from the file
// extension and the ACL rather than a mode bit. Split out by build tag so the
// extraction path has no OS branch in it.
func markExecutable(path string) error {
	return nil
}
