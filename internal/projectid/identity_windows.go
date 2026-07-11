//go:build windows

package projectid

import (
	"fmt"
	"os"
	"syscall"
)

func nativeIdentity(path string) (string, bool) {
	file, err := os.Open(path)
	if err != nil {
		return "", false
	}
	defer file.Close()
	var information syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(syscall.Handle(file.Fd()), &information); err != nil {
		return "", false
	}
	return fmt.Sprintf(
		"windows:%08x:%08x:%08x",
		information.VolumeSerialNumber,
		information.FileIndexHigh,
		information.FileIndexLow,
	), true
}
