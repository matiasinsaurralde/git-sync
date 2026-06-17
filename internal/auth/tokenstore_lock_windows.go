//go:build windows

package auth

import (
	"fmt"
	"os"
)

// Windows has no flock(2). The file token store's writes are atomic
// (temp file + rename), which keeps a single write safe on its own; the lock
// only guards against a lost update between concurrent read-modify-write
// processes, which is rare for a credential store. Rather than pull in a
// Windows-specific locking dependency, open (and create) the lock file so the
// call still succeeds and behaves like a no-op advisory lock.
//
// flockShared / flockExclusive mirror the Unix signatures so callers compile
// unchanged across platforms.
func flockShared(path string) (func(), error)    { return flockOpen(path + ".lock") }
func flockExclusive(path string) (func(), error) { return flockOpen(path + ".lock") }

func flockOpen(lockPath string) (func(), error) {
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}
	return func() { _ = f.Close() }, nil
}
