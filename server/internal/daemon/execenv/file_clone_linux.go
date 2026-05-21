//go:build linux

package execenv

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

func cloneFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open %s: %w", src, err)
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("create %s: %w", dst, err)
	}
	closed := false
	defer func() {
		if !closed {
			_ = out.Close()
		}
	}()

	if err := unix.IoctlFileClone(int(out.Fd()), int(in.Fd())); err != nil {
		closed = true
		_ = out.Close()
		_ = os.Remove(dst)
		if isCloneUnsupportedErr(err) {
			return fmt.Errorf("%w: %v", errCloneUnsupported, err)
		}
		return err
	}
	if err := out.Close(); err != nil {
		closed = true
		return fmt.Errorf("close %s: %w", dst, err)
	}
	closed = true
	return nil
}

func isCloneUnsupportedErr(err error) bool {
	return errors.Is(err, unix.ENOTSUP) ||
		errors.Is(err, unix.EOPNOTSUPP) ||
		errors.Is(err, unix.ENOTTY) ||
		errors.Is(err, unix.EXDEV) ||
		errors.Is(err, unix.EINVAL) ||
		errors.Is(err, unix.ENOSYS)
}
