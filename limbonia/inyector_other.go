//go:build !windows

package limbonia

import "errors"

func InjectLimbo() error {
	return errors.New("injection is only supported on Windows")
}
