package store

import (
	"fmt"
	"os"
	"syscall"
)

// withLock serializes writes across processes (multiple CLI invocations, the
// TUI, etc.) sharing the same store directory (requirements §7: concurrent
// access without data corruption). It takes an exclusive advisory lock via
// flock(2), which macOS and Linux both support.
func (s *Store) withLock(fn func() error) error {
	f, err := os.OpenFile(s.lockPath(), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return fmt.Errorf("store: opening lock file: %w", err)
	}
	defer f.Close()

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("store: acquiring lock: %w", err)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)

	return fn()
}
