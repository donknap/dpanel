package archive

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/mholt/archives"
)

type detectedFormat struct {
	extractor archives.Extractor
}

// UnArchive extracts a supported local archive into targetPath.
func UnArchive(sourcePath, targetPath string) error {
	sourceInfo, err := os.Lstat(sourcePath)
	if err != nil {
		return fmt.Errorf("inspect archive source: %w", err)
	}
	if !sourceInfo.Mode().IsRegular() {
		return fmt.Errorf("archive source %q is not a regular file", sourcePath)
	}
	if hasMultipleHardLinks(sourceInfo) {
		return fmt.Errorf("archive source %q is a hard link", sourcePath)
	}

	source, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open archive source: %w", err)
	}
	defer source.Close()

	format, err := detectFormat(source)
	if err != nil {
		return fmt.Errorf("identify archive format: %w", err)
	}
	return unarchiveFiles(source, sourceInfo, targetPath, format.extractor)
}

func detectFormat(source io.ReadSeeker) (detectedFormat, error) {
	header := make([]byte, 8)
	readCount, err := io.ReadFull(source, header)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return detectedFormat{}, err
	}
	header = header[:readCount]
	if _, err = source.Seek(0, io.SeekStart); err != nil {
		return detectedFormat{}, err
	}

	switch {
	case hasPrefix(header, []byte("PK\x03\x04")), hasPrefix(header, []byte("PK\x05\x06")), hasPrefix(header, []byte("PK\x07\x08")):
		return detectedFormat{extractor: archives.Zip{}}, nil
	case hasPrefix(header, []byte("Rar!\x1a\x07\x00")), hasPrefix(header, []byte("Rar!\x1a\x07\x01\x00")):
		return detectedFormat{extractor: archives.Rar{}}, nil
	case hasPrefix(header, []byte("7z\xbc\xaf\x27\x1c")):
		return detectedFormat{extractor: archives.SevenZip{}}, nil
	}

	var compression archives.Compression
	switch {
	case hasPrefix(header, []byte{0x1f, 0x8b}):
		compression = archives.Gz{}
	case hasPrefix(header, []byte("BZh")):
		compression = archives.Bz2{}
	case hasPrefix(header, []byte{0xfd, 0x37, 0x7a, 0x58, 0x5a, 0x00}):
		compression = archives.Xz{}
	case hasPrefix(header, []byte{0x28, 0xb5, 0x2f, 0xfd}):
		compression = archives.Zstd{}
	}
	if compression != nil {
		reader, err := compression.OpenReader(source)
		if err != nil {
			return detectedFormat{}, err
		}
		isTar, err := streamIsTar(reader)
		closeErr := reader.Close()
		if err != nil {
			return detectedFormat{}, err
		}
		if closeErr != nil {
			return detectedFormat{}, closeErr
		}
		if _, err = source.Seek(0, io.SeekStart); err != nil {
			return detectedFormat{}, err
		}
		if isTar {
			return detectedFormat{extractor: archives.CompressedArchive{
				Extraction:  archives.Tar{},
				Compression: compression,
			}}, nil
		}
		return detectedFormat{}, fmt.Errorf("%w: compressed stream is not a tar archive", ErrUnsupportedFormat)
	}

	isTar, err := streamIsTar(source)
	if err != nil {
		return detectedFormat{}, err
	}
	if _, err = source.Seek(0, io.SeekStart); err != nil {
		return detectedFormat{}, err
	}
	if isTar {
		return detectedFormat{extractor: archives.Tar{}}, nil
	}
	return detectedFormat{}, ErrUnsupportedFormat
}

func streamIsTar(reader io.Reader) (bool, error) {
	header := make([]byte, 1024)
	readCount, err := io.ReadFull(reader, header)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return false, err
	}
	if readCount < 512 {
		return false, nil
	}
	if bytes.Equal(header[:512], make([]byte, 512)) {
		return readCount >= len(header) && bytes.Equal(header[512:], make([]byte, 512)), nil
	}
	tarReader := tar.NewReader(io.MultiReader(bytes.NewReader(header[:readCount]), reader))
	_, err = tarReader.Next()
	return err == nil, nil
}

func hasPrefix(value, prefix []byte) bool {
	return len(value) >= len(prefix) && bytes.Equal(value[:len(prefix)], prefix)
}

func unarchiveFiles(source *os.File, sourceInfo fs.FileInfo, targetPath string, extractor archives.Extractor) error {
	manifest := make(map[string]bool)
	err := extractor.Extract(context.Background(), source, func(_ context.Context, info archives.FileInfo) error {
		name, err := validateArchiveEntry(info)
		if err != nil {
			return err
		}
		if name != "" {
			if err = addManifestEntry(manifest, name, info.IsDir()); err != nil {
				return err
			}
		}
		if info.IsDir() {
			return nil
		}
		reader, err := info.Open()
		if err != nil {
			return err
		}
		_, err = io.Copy(io.Discard, reader)
		if err != nil {
			_ = reader.Close()
			return err
		}
		return reader.Close()
	})
	if err != nil {
		return fmt.Errorf("validate archive: %w", err)
	}
	if _, err = source.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("rewind archive: %w", err)
	}

	rootPath, err := prepareDestination(targetPath)
	if err != nil {
		return err
	}
	directories := make([]directoryMetadata, 0)
	err = extractor.Extract(context.Background(), source, func(_ context.Context, info archives.FileInfo) error {
		name, err := validateArchiveEntry(info)
		if err != nil {
			return err
		}
		if name == "" {
			return nil
		}
		if info.IsDir() {
			directoryPath, err := ensureDirectoryPath(rootPath, name)
			if err != nil {
				return err
			}
			directories = append(directories, directoryMetadata{
				path:    directoryPath,
				mode:    info.Mode().Perm(),
				modTime: info.ModTime(),
			})
			return nil
		}
		return extractRegularFile(rootPath, name, info, sourceInfo)
	})
	if err != nil {
		return fmt.Errorf("extract archive: %w", err)
	}
	for index := len(directories) - 1; index >= 0; index-- {
		if err = applyDirectoryMetadata(directories[index]); err != nil {
			return err
		}
	}
	return nil
}

func validateArchiveEntry(info archives.FileInfo) (string, error) {
	if header, ok := info.Header.(*tar.Header); ok && header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA && header.Typeflag != tar.TypeDir {
		return "", fmt.Errorf("archive entry %q is not a regular file or directory", info.NameInArchive)
	}
	if info.LinkTarget != "" || info.Mode()&fs.ModeSymlink != 0 {
		return "", fmt.Errorf("archive entry %q is a link", info.NameInArchive)
	}
	if !info.IsDir() && !info.Mode().IsRegular() {
		return "", fmt.Errorf("archive entry %q is not a regular file or directory", info.NameInArchive)
	}
	name, err := normalizeArchivePath(info.NameInArchive)
	if err != nil {
		if info.IsDir() && path.Clean(strings.ReplaceAll(info.NameInArchive, "\\", "/")) == "." {
			return "", nil
		}
		return "", fmt.Errorf("invalid archive entry %q: %w", info.NameInArchive, err)
	}
	return name, nil
}

func normalizeArchivePath(name string) (string, error) {
	if name == "" {
		return "", errors.New("archive path is empty")
	}
	if strings.IndexByte(name, 0) >= 0 {
		return "", errors.New("archive path contains NUL")
	}
	name = strings.ReplaceAll(name, "\\", "/")
	if path.IsAbs(name) || filepath.IsAbs(name) || hasWindowsVolume(name) {
		return "", errors.New("archive path is absolute")
	}
	for _, component := range strings.Split(name, "/") {
		if component == ".." {
			return "", errors.New("archive path contains parent traversal")
		}
	}
	name = path.Clean(name)
	if name == "." || name == "" {
		return "", errors.New("archive path is empty")
	}
	return name, nil
}

func hasWindowsVolume(name string) bool {
	return len(name) >= 2 && ((name[0] >= 'a' && name[0] <= 'z') || (name[0] >= 'A' && name[0] <= 'Z')) && name[1] == ':'
}

func addManifestEntry(manifest map[string]bool, name string, isDirectory bool) error {
	if _, exists := manifest[name]; exists {
		return fmt.Errorf("archive contains duplicate path %q", name)
	}
	for parent := path.Dir(name); parent != "."; parent = path.Dir(parent) {
		if directory, exists := manifest[parent]; exists && !directory {
			return fmt.Errorf("archive path %q has a non-directory parent", name)
		}
	}
	if !isDirectory {
		prefix := name + "/"
		for existingName := range manifest {
			if strings.HasPrefix(existingName, prefix) {
				return fmt.Errorf("archive file %q is a parent of another entry", name)
			}
		}
	}
	manifest[name] = isDirectory
	return nil
}

func prepareDestination(targetPath string) (string, error) {
	absolutePath, err := filepath.Abs(targetPath)
	if err != nil {
		return "", fmt.Errorf("resolve archive destination: %w", err)
	}
	info, err := os.Lstat(absolutePath)
	if errors.Is(err, fs.ErrNotExist) {
		if err = os.MkdirAll(absolutePath, 0o755); err != nil {
			return "", fmt.Errorf("create archive destination: %w", err)
		}
		info, err = os.Lstat(absolutePath)
	}
	if err != nil {
		return "", fmt.Errorf("inspect archive destination: %w", err)
	}
	if !info.IsDir() || info.Mode()&fs.ModeSymlink != 0 {
		return "", fmt.Errorf("archive destination %q is not a directory", targetPath)
	}
	return absolutePath, nil
}

func ensureDirectoryPath(rootPath, archivePath string) (string, error) {
	currentPath := rootPath
	for _, component := range strings.Split(archivePath, "/") {
		currentPath = filepath.Join(currentPath, component)
		info, err := os.Lstat(currentPath)
		if errors.Is(err, fs.ErrNotExist) {
			if err = os.Mkdir(currentPath, 0o755); err != nil {
				return "", fmt.Errorf("create archive directory %q: %w", archivePath, err)
			}
			continue
		}
		if err != nil {
			return "", fmt.Errorf("inspect archive directory %q: %w", archivePath, err)
		}
		if !info.IsDir() || info.Mode()&fs.ModeSymlink != 0 {
			return "", fmt.Errorf("archive target %q contains a non-directory parent", archivePath)
		}
	}
	return currentPath, nil
}

func extractRegularFile(rootPath, name string, info archives.FileInfo, sourceInfo fs.FileInfo) error {
	reader, err := info.Open()
	if err != nil {
		return err
	}
	err = writeRegularFile(rootPath, name, info, reader, sourceInfo)
	if err != nil {
		_ = reader.Close()
		return err
	}
	return reader.Close()
}

type regularFileInfo interface {
	Mode() fs.FileMode
	ModTime() time.Time
}

func writeRegularFile(rootPath, name string, info regularFileInfo, reader io.Reader, sourceInfo fs.FileInfo) error {
	parentPath := path.Dir(name)
	if parentPath != "." {
		if _, err := ensureDirectoryPath(rootPath, parentPath); err != nil {
			return err
		}
	}
	targetPath := filepath.Join(rootPath, filepath.FromSlash(name))
	if err := validateRegularTarget(targetPath, sourceInfo); err != nil {
		return err
	}

	temporaryFile, err := os.CreateTemp(filepath.Dir(targetPath), ".archive-*")
	if err != nil {
		return err
	}
	temporaryPath := temporaryFile.Name()
	defer func() {
		_ = temporaryFile.Close()
		_ = os.Remove(temporaryPath)
	}()
	if _, err = io.Copy(temporaryFile, reader); err != nil {
		return err
	}
	if err = temporaryFile.Chmod(info.Mode().Perm()); err != nil {
		return err
	}
	if !info.ModTime().IsZero() {
		if err = os.Chtimes(temporaryPath, info.ModTime(), info.ModTime()); err != nil {
			return err
		}
	}
	if err = temporaryFile.Close(); err != nil {
		return err
	}
	if err = validateRegularTarget(targetPath, sourceInfo); err != nil {
		return err
	}
	return os.Rename(temporaryPath, targetPath)
}

func validateRegularTarget(targetPath string, sourceInfo fs.FileInfo) error {
	info, err := os.Lstat(targetPath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("archive target %q is not a regular file", targetPath)
	}
	if hasMultipleHardLinks(info) {
		return fmt.Errorf("archive target %q is a hard link", targetPath)
	}
	if os.SameFile(sourceInfo, info) {
		return fmt.Errorf("archive target %q is the archive source", targetPath)
	}
	return nil
}

type directoryMetadata struct {
	path    string
	mode    fs.FileMode
	modTime time.Time
}

func applyDirectoryMetadata(metadata directoryMetadata) error {
	info, err := os.Lstat(metadata.path)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&fs.ModeSymlink != 0 {
		return fmt.Errorf("archive directory %q changed during extraction", metadata.path)
	}
	if err = os.Chmod(metadata.path, metadata.mode); err != nil {
		return err
	}
	if !metadata.modTime.IsZero() {
		if err = os.Chtimes(metadata.path, metadata.modTime, metadata.modTime); err != nil {
			return err
		}
	}
	return nil
}
