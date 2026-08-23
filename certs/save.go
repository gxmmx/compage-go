package certs

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func validateDirectory(path string) error {
	if path == "" {
		return invalid("directory is required", nil)
	}
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("certs: inspect directory: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return invalid("save location must be an existing non-symlink directory", nil)
	}
	if err := rejectSymlinkParents(path); err != nil {
		return err
	}
	return nil
}

func rejectSymlinkParents(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	current := string(filepath.Separator)
	volume := filepath.VolumeName(abs)
	if volume != "" {
		current = volume + string(filepath.Separator)
		abs = strings.TrimPrefix(abs, volume)
	}
	for _, part := range strings.Split(strings.TrimPrefix(abs, string(filepath.Separator)), string(filepath.Separator)) {
		if part == "" {
			continue
		}
		current = filepath.Join(current, part)
		info, statErr := os.Lstat(current)
		if statErr != nil {
			if errors.Is(statErr, os.ErrNotExist) {
				return nil
			}
			return statErr
		}
		if info.Mode()&os.ModeSymlink != 0 && current != "/var" {
			return &ForbiddenError{Message: "path traverses a symbolic link"}
		}
	}
	return nil
}

func atomicSave(path string, data []byte, mode fs.FileMode, opts ...Option) error {
	o, err := parseOptions(scopeSave, opts)
	if err != nil {
		return err
	}
	if path == "" {
		return invalid("path is required", nil)
	}
	dir := filepath.Dir(path)
	if err := validateDirectory(dir); err != nil {
		return err
	}
	if info, statErr := os.Lstat(path); statErr == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return &ConflictError{Message: "target is a symlink"}
		}
		if !info.Mode().IsRegular() {
			return &ConflictError{Message: "target is not a regular file"}
		}
		if !o.replace {
			return &ConflictError{Message: "target already exists"}
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return fmt.Errorf("certs: inspect target: %w", statErr)
	}
	f, err := os.CreateTemp(dir, ".certs-*")
	if err != nil {
		return fmt.Errorf("certs: create temporary file: %w", err)
	}
	tmp := f.Name()
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(tmp)
		}
	}()
	if err = f.Chmod(mode); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("certs: write file: %w", err)
	}
	if !o.replace {
		if _, err = os.Lstat(path); err == nil {
			return &ConflictError{Message: "target already exists"}
		}
	}
	if err = os.Rename(tmp, path); err != nil {
		return fmt.Errorf("certs: publish file: %w", err)
	}
	ok = true
	if d, openErr := os.Open(dir); openErr == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	return nil
}
