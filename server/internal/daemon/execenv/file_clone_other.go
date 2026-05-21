//go:build !darwin && !linux

package execenv

func cloneFile(src, dst string) error {
	return errCloneUnsupported
}
