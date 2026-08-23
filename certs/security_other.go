//go:build !linux && !darwin

package certs

import (
	"context"
	"os"
)

func splitSecuritySupported() bool { return false }
func effectiveUID() int            { return -1 }
func effectiveGID() int            { return -1 }
func supplementaryGroups() ([]int, error) {
	return nil, &UnavailableError{Message: "split owner/group security is unsupported on this platform"}
}
func fileOwnership(os.FileInfo) (int, int, bool) { return 0, 0, false }
func openNoFollow(string, int, os.FileMode) (*os.File, error) {
	return nil, &UnavailableError{Message: "safe FileStore access is unsupported on this platform"}
}
func acquireFileLock(context.Context, *os.File) error {
	return &UnavailableError{Message: "FileStore locking is unsupported on this platform"}
}
func acquireFileReadLock(context.Context, *os.File) error {
	return &UnavailableError{Message: "FileStore locking is unsupported on this platform"}
}
func releaseFileLock(*os.File) {}
