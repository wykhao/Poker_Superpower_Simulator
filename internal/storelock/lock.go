// Package storelock prevents two local processes from writing the same history.
package storelock

import (
	"fmt"
	"os"
	"path/filepath"
)

func Acquire(dir string) (*os.File, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(dir, ".instance.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = lock(f); err != nil {
		f.Close()
		return nil, fmt.Errorf("this data directory is already in use by another simulator: %w", err)
	}
	return f, nil // Closing the file or exiting the process releases the OS lock.
}
