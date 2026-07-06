//go:build windows

package platform

import "os"

// FlockLock Windows — no-op.
func FlockLock(f *os.File, path string, how int) error { return nil }

// FlockUnlock Windows — no-op.
func FlockUnlock(f *os.File) error { return nil }

// Lock constants
const (
	FlockSH = 1
	FlockEX = 2
)
