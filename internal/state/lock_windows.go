//go:build windows

package state

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

const (
	lockfileFailImmediately = 0x00000001
	lockfileExclusiveLock   = 0x00000002
)

var (
	kernel32Lock     = syscall.NewLazyDLL("kernel32.dll")
	procLockFileEx   = kernel32Lock.NewProc("LockFileEx")
	procUnlockFileEx = kernel32Lock.NewProc("UnlockFileEx")
)

type Lock struct {
	file       *os.File
	overlapped syscall.Overlapped
}

func AcquireLock(stateHome string) (*Lock, error) {
	return AcquireFileLock(filepath.Join(stateHome, "manager.lock"))
}

func AcquireFileLock(path string) (*Lock, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	lock := &Lock{file: file}
	result, _, callErr := procLockFileEx.Call(
		file.Fd(),
		lockfileFailImmediately|lockfileExclusiveLock,
		0,
		1,
		0,
		uintptr(unsafe.Pointer(&lock.overlapped)),
	)
	if result == 0 {
		file.Close()
		if errors.Is(callErr, syscall.Errno(33)) {
			return nil, ErrLockBusy
		}
		return nil, fmt.Errorf("lock %s: %w", path, callErr)
	}
	if err := recordWindowsLockOwner(file); err != nil {
		lock.Close()
		return nil, err
	}
	return lock, nil
}

func recordWindowsLockOwner(file *os.File) error {
	if err := file.Truncate(0); err != nil {
		return err
	}
	if _, err := file.Seek(0, 0); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(file, "%d\n", os.Getpid()); err != nil {
		return err
	}
	return file.Sync()
}

func (lock *Lock) Close() error {
	if lock == nil || lock.file == nil {
		return nil
	}
	result, _, callErr := procUnlockFileEx.Call(
		lock.file.Fd(),
		0,
		1,
		0,
		uintptr(unsafe.Pointer(&lock.overlapped)),
	)
	closeErr := lock.file.Close()
	lock.file = nil
	if result == 0 {
		return callErr
	}
	return closeErr
}
