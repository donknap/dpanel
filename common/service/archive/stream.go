package archive

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/mholt/archives"
)

// EntryTree checks type conflicts when several archives are extracted together.
// Later archives may replace a file at the same path.
type EntryTree map[string]bool

func (entries EntryTree) Add(name string, isDir bool) error {
	if directory, exists := entries[name]; exists {
		if directory != isDir {
			return fmt.Errorf("archive target %q changes file type", name)
		}
		return nil
	}
	for parent := path.Dir(name); parent != "."; parent = path.Dir(parent) {
		if directory, exists := entries[parent]; exists && !directory {
			return fmt.Errorf("archive target %q has a non-directory parent", name)
		}
	}
	if !isDir {
		prefix := name + "/"
		for existingName := range entries {
			if strings.HasPrefix(existingName, prefix) {
				return fmt.Errorf("archive file %q is a parent of another entry", name)
			}
		}
	}
	entries[name] = isDir
	return nil
}

// ReadEntries reads validated regular files and directories from a seekable archive.
// The content reader is valid only for the duration of the callback.
func ReadEntries(source io.ReadSeeker, handle func(string, archives.FileInfo, io.Reader) error) error {
	if _, err := source.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("rewind archive: %w", err)
	}
	format, err := detectFormat(source)
	if err != nil {
		return fmt.Errorf("identify archive format: %w", err)
	}
	manifest := make(map[string]bool)
	err = format.extractor.Extract(context.Background(), source, func(_ context.Context, info archives.FileInfo) error {
		name, err := validateArchiveEntry(info)
		if err != nil {
			return err
		}
		if name == "" {
			return nil
		}
		if err = addManifestEntry(manifest, name, info.IsDir()); err != nil {
			return err
		}
		if info.IsDir() {
			return handle(name, info, nil)
		}
		reader, err := info.Open()
		if err != nil {
			return err
		}
		if err = handle(name, info, reader); err != nil {
			_ = reader.Close()
			return err
		}
		_, err = io.Copy(io.Discard, reader)
		return errors.Join(err, reader.Close())
	})
	if err != nil {
		return fmt.Errorf("read archive: %w", err)
	}
	return nil
}
