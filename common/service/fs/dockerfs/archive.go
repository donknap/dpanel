package dockerfs

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	archiveservice "github.com/donknap/dpanel/common/service/archive"
	"github.com/donknap/dpanel/common/service/fs/tempfs"
	"github.com/mholt/archives"
)

type archiveSource struct {
	file   *os.File
	offset int64
	size   int64
}

func (source archiveSource) Reader() *io.SectionReader {
	return io.NewSectionReader(source.file, source.offset, source.size)
}

type archiveEntryMetadata struct {
	mode    os.FileMode
	modTime time.Time
}

func (self *Fs) UnArchive(archivePaths []string, destination string) (err error) {
	destination, err = self.pathName(destination)
	if err != nil {
		return err
	}
	destinationInfo, _, err := self.LstatIfPossible(destination)
	if err != nil {
		return err
	}
	if !destinationInfo.IsDir() || destinationInfo.Mode()&os.ModeSymlink != 0 {
		return errors.New("archive destination is not a directory")
	}
	if len(archivePaths) == 0 {
		return errors.New("archive file list is empty")
	}
	temporaryFileSystem, err := tempfs.New()
	if err != nil {
		return err
	}
	sources := make([]archiveSource, 0, len(archivePaths))
	defer func() {
		for _, source := range sources {
			err = errors.Join(err, source.file.Close())
		}
		err = errors.Join(err, temporaryFileSystem.Close())
	}()
	sourcePaths := make(map[string]struct{}, len(archivePaths))
	for index, archivePath := range archivePaths {
		archivePath, err = self.pathName(archivePath)
		if err != nil {
			return err
		}
		info, _, statErr := self.LstatIfPossible(archivePath)
		if statErr != nil {
			return statErr
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("archive source %q is not a regular file", archivePath)
		}
		if err = self.validateExportSource(archivePath); err != nil {
			return err
		}
		backendSource, pathErr := self.backendPath(archivePath)
		if pathErr != nil {
			return pathErr
		}
		transport, _, copyErr := self.sdk.Client.CopyFromContainer(self.sdk.Ctx, self.targetContainerName, backendSource)
		if copyErr != nil {
			return copyErr
		}
		file, createErr := temporaryFileSystem.Create(fmt.Sprintf("/%d.tar", index))
		if createErr != nil {
			return errors.Join(createErr, transport.Close())
		}
		_, copyErr = io.Copy(file, transport)
		copyErr = errors.Join(copyErr, transport.Close())
		if copyErr != nil {
			return errors.Join(copyErr, file.Close())
		}
		source, inspectErr := inspectArchiveTransport(file)
		if inspectErr != nil {
			return errors.Join(inspectErr, file.Close())
		}
		sources = append(sources, source)
		sourcePaths[archivePath] = struct{}{}
	}

	entryTree := make(archiveservice.EntryTree)
	directories := make(map[string]archiveEntryMetadata)
	files := make(map[string]archiveEntryMetadata)
	for _, source := range sources {
		err = archiveservice.ReadEntries(source.Reader(), func(name string, entry archives.FileInfo, _ io.Reader) error {
			if err := entryTree.Add(name, entry.IsDir()); err != nil {
				return err
			}
			target := path.Join(destination, name)
			if _, exists := sourcePaths[target]; exists {
				return fmt.Errorf("archive target %q is an archive source", target)
			}
			if err := self.validateImportTarget(target); err != nil {
				return err
			}
			info, _, err := self.LstatIfPossible(target)
			if err != nil && !isNotExist(err) {
				return err
			}
			if err == nil && entry.IsDir() != info.IsDir() {
				return fmt.Errorf("archive target %q changes file type", target)
			}
			if entry.IsDir() {
				directories[target] = archiveEntryMetadata{mode: entry.Mode().Perm(), modTime: entry.ModTime()}
			} else {
				files[target] = archiveEntryMetadata{mode: entry.Mode().Perm(), modTime: entry.ModTime()}
			}
			return nil
		})
		if err != nil {
			return err
		}
	}

	backendDestination, err := self.backendPath(destination)
	if err != nil {
		return err
	}
	archiveReader, archiveWriter := io.Pipe()
	writeResult := make(chan error, 1)
	go func() {
		tarWriter := tar.NewWriter(archiveWriter)
		var writeErr error
		for _, source := range sources {
			writeErr = archiveservice.ReadEntries(source.Reader(), func(name string, entry archives.FileInfo, content io.Reader) error {
				header, err := tar.FileInfoHeader(entry.FileInfo, "")
				if err != nil {
					return err
				}
				header.Name = name
				header.Mode = int64(entry.Mode().Perm())
				if !entry.IsDir() {
					header.Mode = 0o644
				}
				header.Uid, header.Gid = 0, 0
				header.Uname, header.Gname = "", ""
				if entry.IsDir() {
					header.Name += "/"
					header.Mode = 0o755
				}
				if err = tarWriter.WriteHeader(header); err != nil {
					return err
				}
				if content != nil {
					_, err = io.Copy(tarWriter, content)
				}
				return err
			})
			if writeErr != nil {
				break
			}
		}
		writeErr = errors.Join(writeErr, tarWriter.Close())
		_ = archiveWriter.CloseWithError(writeErr)
		writeResult <- writeErr
	}()
	copyErr := self.sdk.ContainerImport(self.sdk.Ctx, self.targetContainerName, backendDestination, archiveReader)
	_ = archiveReader.CloseWithError(copyErr)
	if err = errors.Join(copyErr, <-writeResult); err != nil {
		return err
	}
	for target, metadata := range files {
		if err = self.Chmod(target, metadata.mode); err != nil {
			return err
		}
		if !metadata.modTime.IsZero() {
			if err = self.Chtimes(target, metadata.modTime, metadata.modTime); err != nil {
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
		metadata := directories[target]
		if err = self.Chmod(target, metadata.mode); err != nil {
			return err
		}
		if !metadata.modTime.IsZero() {
			if err = self.Chtimes(target, metadata.modTime, metadata.modTime); err != nil {
				return err
			}
		}
	}
	return nil
}

func inspectArchiveTransport(file *os.File) (archiveSource, error) {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return archiveSource{}, err
	}
	reader := tar.NewReader(file)
	header, err := reader.Next()
	if err != nil {
		return archiveSource{}, fmt.Errorf("read Docker archive transport: %w", err)
	}
	if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA {
		return archiveSource{}, errors.New("Docker archive transport does not contain a regular file")
	}
	offset, err := file.Seek(0, io.SeekCurrent)
	if err != nil {
		return archiveSource{}, err
	}
	if _, err = reader.Next(); err != io.EOF {
		if err == nil {
			return archiveSource{}, errors.New("Docker archive transport contains multiple files")
		}
		return archiveSource{}, fmt.Errorf("read Docker archive transport: %w", err)
	}
	return archiveSource{file: file, offset: offset, size: header.Size}, nil
}
