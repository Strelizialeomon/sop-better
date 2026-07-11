package state

import "errors"

var ErrLockBusy = errors.New("another release operation is already running")
