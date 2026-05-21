//go:build darwin

package execenv

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

func cloneFile(src, dst string) error {
	if _, err := os.Lstat(dst); err == nil {
		return fmt.Errorf("create %s: %w", dst, os.ErrExist)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat %s: %w", dst, err)
	}

	if err := unix.Clonefile(src, dst, 0); err != nil {
		_ = os.Remove(dst)
		if isCloneUnsupportedErr(err) {
			return fmt.Errorf("%w: %v", errCloneUnsupported, err)
		}
		return err
	}
	return nil
}

func isCloneUnsupportedErr(err error) bool {
	return errors.Is(err, unix.ENOTSUP) ||
		errors.Is(err, unix.EOPNOTSUPP) ||
		errors.Is(err, unix.EXDEV) ||
		errors.Is(err, unix.EINVAL) ||
		errors.Is(err, unix.ENOSYS)
}
