package importer

import (
	"archive/tar"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
)

const (
	maxEntries   = 20000
	maxFileSize  = 512 << 20
	maxTotalSize = 4 << 30
)

type Handler struct{}

func New() *Handler { return &Handler{} }

func (self *Handler) Handle(ctx context.Context, args []string) (any, error) {
	if len(args) != 1 || args[0] != "--dpanel" {
		return nil, errors.New("import requires --dpanel")
	}
	root, err := os.OpenRoot("/dpanel")
	if err != nil {
		return nil, fmt.Errorf("open dpanel directory: %w", err)
	}
	defer root.Close()

	reader := tar.NewReader(os.Stdin)
	seen := make(map[string]struct{})
	var total int64
	for count := 0; ; count++ {
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return nil, nil
		}
		if err != nil {
			return nil, fmt.Errorf("read import archive: %w", err)
		}
		if count >= maxEntries {
			return nil, errors.New("import archive has too many entries")
		}
		name, err := entryName(header.Name)
		if err != nil {
			return nil, err
		}
		if _, exists := seen[name]; exists {
			return nil, fmt.Errorf("duplicate import path %q", name)
		}
		seen[name] = struct{}{}
		if header.Typeflag != tar.TypeDir && header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA {
			return nil, fmt.Errorf("unsupported import entry %q", name)
		}
		if err = rejectSymlinkTargets(root, name); err != nil {
			return nil, fmt.Errorf("unsafe import path %q: %w", name, err)
		}
		if header.Typeflag == tar.TypeDir {
			if header.Size != 0 {
				return nil, fmt.Errorf("import directory %q has content", name)
			}
			if err = root.MkdirAll(name, os.FileMode(header.Mode)&0o777); err != nil {
				return nil, fmt.Errorf("create import directory %q: %w", name, err)
			}
			continue
		}
		if header.Size < 0 || header.Size > maxFileSize || total > maxTotalSize-header.Size {
			return nil, fmt.Errorf("import file %q exceeds size limit", name)
		}
		total += header.Size
		if err = root.MkdirAll(path.Dir(name), 0o755); err != nil {
			return nil, fmt.Errorf("create import parent %q: %w", name, err)
		}
		if err = writeFile(root, name, reader, header.Size, os.FileMode(header.Mode)&0o777); err != nil {
			return nil, fmt.Errorf("import file %q: %w", name, err)
		}
	}
}

func entryName(name string) (string, error) {
	if name == "" || len(name) > 4096 || strings.IndexByte(name, 0) >= 0 || strings.Contains(name, "\\") || path.IsAbs(name) || path.Clean(name) != name {
		return "", fmt.Errorf("invalid import path %q", name)
	}
	for _, part := range strings.Split(name, "/") {
		if part == "" || part == "." || part == ".." {
			return "", fmt.Errorf("invalid import path %q", name)
		}
	}
	return name, nil
}

func rejectSymlinkTargets(root *os.Root, name string) error {
	for current := name; current != "."; current = path.Dir(current) {
		info, err := root.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return errors.New("target contains a symbolic link")
		}
	}
	return nil
}

func writeFile(root *os.Root, name string, reader io.Reader, size int64, mode os.FileMode) error {
	if info, err := root.Lstat(name); err == nil {
		if !info.Mode().IsRegular() {
			return errors.New("target is not a regular file")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return err
	}
	temporary := name + ".dpanel-import-" + hex.EncodeToString(random[:])
	file, err := root.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	defer root.Remove(temporary)
	if _, err = io.CopyN(file, reader, size); err != nil {
		_ = file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return root.Rename(temporary, name)
}
