//go:build !aix && !darwin && !dragonfly && !freebsd && !illumos && !linux && !netbsd && !openbsd && !solaris && !windows

package projectid

func nativeIdentity(string) (string, bool) {
	return "", false
}
