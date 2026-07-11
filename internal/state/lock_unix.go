//go:build !windows

package state

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

type Lock struct {
	file *os.File
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
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return nil, ErrLockBusy
		}
		return nil, fmt.Errorf("lock %s: %w", path, err)
	}
	if err := recordLockOwner(file); err != nil {
		syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		file.Close()
		return nil, err
	}
	return &Lock{file: file}, nil
}

func recordLockOwner(file *os.File) error {
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
	unlockErr := syscall.Flock(int(lock.file.Fd()), syscall.LOCK_UN)
	closeErr := lock.file.Close()
	lock.file = nil
	if unlockErr != nil {
		return unlockErr
	}
	return closeErr
}
