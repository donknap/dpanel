package archive

import (
	"archive/tar"
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type Option struct {
	Root string
}

// ZipToTar converts a ZIP file to a tar file, optionally selecting one ZIP directory.
func ZipToTar(ctx context.Context, zipPath, tarPath string, option Option) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	root := strings.Trim(option.Root, "/")
	if root != "" {
		name, err := normalizeArchivePath(root)
		if err != nil {
			return fmt.Errorf("invalid ZIP root: %w", err)
		}
		root = name
	}
	sourceInfo, err := os.Lstat(zipPath)
	if err != nil {
		return err
	}
	if !sourceInfo.Mode().IsRegular() || hasMultipleHardLinks(sourceInfo) {
		return fmt.Errorf("ZIP source %q is not a regular file", zipPath)
	}
	zipArchive, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer zipArchive.Close()

	if err = os.MkdirAll(filepath.Dir(tarPath), 0o755); err != nil {
		return err
	}
	if err = validateArchiveTarget(tarPath); err != nil {
		return err
	}
	if targetInfo, statErr := os.Lstat(tarPath); statErr == nil && os.SameFile(sourceInfo, targetInfo) {
		return fmt.Errorf("tar target %q is the ZIP source", tarPath)
	} else if statErr != nil && !errors.Is(statErr, fs.ErrNotExist) {
		return statErr
	}
	temporaryFile, err := os.CreateTemp(filepath.Dir(tarPath), "."+filepath.Base(tarPath)+"-*")
	if err != nil {
		return err
	}
	temporaryPath := temporaryFile.Name()
	defer func() {
		_ = temporaryFile.Close()
		_ = os.Remove(temporaryPath)
	}()

	tarWriter := tar.NewWriter(temporaryFile)
	for _, zipFile := range zipArchive.File {
		if err = ctx.Err(); err != nil {
			return err
		}
		if strings.HasPrefix(zipFile.Name, "__MACOSX") {
			continue
		}
		name, err := normalizeArchivePath(zipFile.Name)
		if err != nil {
			return fmt.Errorf("invalid ZIP entry %q: %w", zipFile.Name, err)
		}
		if root != "" {
			if name == root && zipFile.FileInfo().IsDir() {
				continue
			}
			if !strings.HasPrefix(name, root+"/") {
				continue
			}
			name = strings.TrimPrefix(name, root+"/")
		}
		if name == "." || name == "" || path.IsAbs(name) {
			continue
		}
		info := zipFile.FileInfo()
		if !info.IsDir() && !info.Mode().IsRegular() {
			return fmt.Errorf("ZIP entry %q is not a regular file or directory", zipFile.Name)
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = name
		if info.IsDir() {
			header.Name += "/"
		}
		if err = tarWriter.WriteHeader(header); err != nil {
			return err
		}
		if info.IsDir() {
			continue
		}
		reader, err := zipFile.Open()
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(tarWriter, contextReader{ctx: ctx, reader: reader})
		closeErr := reader.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if err = tarWriter.Close(); err != nil {
		return err
	}
	if err = temporaryFile.Close(); err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if err = validateArchiveTarget(tarPath); err != nil {
		return err
	}
	return os.Rename(temporaryPath, tarPath)
}
