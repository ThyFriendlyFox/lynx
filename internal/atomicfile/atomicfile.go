// Package atomicfile writes files so that a reader never sees a partial file.
package atomicfile

import (
	"io"
	"os"
	"path/filepath"
)

// Write creates path with the given bytes. It writes a temporary file in the
// same directory and renames it over path. A crash leaves either the old file
// or the new file, never a half-written one.
func Write(path string, data []byte, perm os.FileMode) error {
	return WriteFrom(path, func(w io.Writer) error {
		_, err := w.Write(data)
		return err
	}, perm)
}

// WriteFrom is Write for content produced by a callback, so that large files
// never need to be held in memory.
func WriteFrom(path string, fill func(io.Writer) error, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".lynx-tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if err := fill(tmp); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
