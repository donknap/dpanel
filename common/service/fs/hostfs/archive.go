package hostfs

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"runtime"
	"sort"
	"strings"

	archiveservice "github.com/donknap/dpanel/common/service/archive"
	"github.com/mholt/archives"
	"github.com/spf13/afero"
)

func (self *Fs) UnArchive(archivePaths []string, destination string) (err error) {
	destination, err = self.pathName(destination)
	if err != nil {
		return err
	}
	info, _, err := self.LstatIfPossible(destination)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("archive destination is not a directory")
	}

	sources := make([]afero.File, 0, len(archivePaths))
	sourcePaths := make(map[string]struct{}, len(archivePaths))
	sourceInfos := make([]os.FileInfo, 0, len(archivePaths))
	defer func() {
		for _, source := range sources {
			err = errors.Join(err, source.Close())
		}
	}()
	for _, archivePath := range archivePaths {
		archivePath, err = self.pathName(archivePath)
		if err != nil {
			return err
		}
		info, _, err = self.LstatIfPossible(archivePath)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("archive source %q is not a regular file", archivePath)
		}
		if err = self.validateTransferSource(archivePath); err != nil {
			return err
		}
		source, openErr := self.Open(archivePath)
		if openErr != nil {
			return openErr
		}
		sources = append(sources, source)
		sourcePaths[archivePath] = struct{}{}
		sourceInfos = append(sourceInfos, info)
	}

	entryTree := make(archiveservice.EntryTree)
	for _, source := range sources {
		err = archiveservice.ReadEntries(source, func(name string, entry archives.FileInfo, _ io.Reader) error {
			if err := entryTree.Add(name, entry.IsDir()); err != nil {
				return err
			}
			target := path.Join(destination, name)
			if _, exists := sourcePaths[target]; exists {
				return fmt.Errorf("archive target %q is an archive source", target)
			}
			if err := self.validateTransferTarget(target); err != nil {
				return err
			}
			info, exists, err := self.pathInfo(target)
			if err != nil || !exists {
				return err
			}
			for _, sourceInfo := range sourceInfos {
				if os.SameFile(sourceInfo, info) {
					return fmt.Errorf("archive target %q is an archive source", target)
				}
			}
			if entry.IsDir() != info.IsDir() {
				return fmt.Errorf("archive target %q changes file type", target)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}

	directories := make(map[string]archives.FileInfo)
	files := make(map[string]archives.FileInfo)
	for _, source := range sources {
		err = archiveservice.ReadEntries(source, func(name string, entry archives.FileInfo, content io.Reader) error {
			target := path.Join(destination, name)
			if err := self.validateTransferTarget(target); err != nil {
				return err
			}
			if entry.IsDir() {
				if err := self.MkdirAll(target, 0o755); err != nil {
					return err
				}
				directories[target] = entry
				return nil
			}
			if err := self.MkdirAll(path.Dir(target), 0o755); err != nil {
				return err
			}
			file, err := self.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o666)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(file, content)
			err = errors.Join(copyErr, file.Close())
			if err != nil {
				return err
			}
			files[target] = entry
			return nil
		})
		if err != nil {
			return err
		}
	}
	for target, entry := range files {
		if runtime.GOOS != "windows" || len(self.roots) == 0 {
			if err = self.Chmod(target, entry.Mode().Perm()); err != nil {
				return err
			}
		}
		if !entry.ModTime().IsZero() {
			if err = self.Chtimes(target, entry.ModTime(), entry.ModTime()); err != nil {
				return err
			}
		}
	}
	paths := make([]string, 0, len(directories))
	for target := range directories {
		paths = append(paths, target)
	}
	sort.Slice(paths, func(i, j int) bool {
		return strings.Count(paths[i], "/") > strings.Count(paths[j], "/")
	})
	for _, target := range paths {
		entry := directories[target]
		if runtime.GOOS != "windows" || len(self.roots) == 0 {
			if err = self.Chmod(target, entry.Mode().Perm()); err != nil {
				return err
			}
		}
		if !entry.ModTime().IsZero() {
			if err = self.Chtimes(target, entry.ModTime(), entry.ModTime()); err != nil {
				return err
			}
		}
	}
	return nil
}
