package archive

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/flate"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"time"
)

// CompressionLevel controls the balance between compression speed and size.
type CompressionLevel int

var ErrUnsupportedFormat = errors.New("unsupported archive format")

const (
	Default CompressionLevel = iota
	Fast
	Best
)

type Format string

const (
	FormatZip   Format = "zip"
	FormatTar   Format = "tar"
	FormatTarGz Format = "tar.gz"
)

type CreateOption func(*createOptions) error

type createOptions struct {
	level   CompressionLevel
	sources []createSource
}

type createSource struct {
	sourcePath  string
	archivePath string
	content     []byte
	isContent   bool
}

type createEntry struct {
	info          fs.FileInfo
	nameInArchive string
	open          func() (io.ReadCloser, error)
}

func WithFile(sourcePath string, archivePath ...string) CreateOption {
	return func(options *createOptions) error {
		if len(archivePath) > 1 {
			return errors.New("archive path accepts at most one value")
		}
		name := ""
		if len(archivePath) == 1 {
			name = archivePath[0]
		}
		options.sources = append(options.sources, createSource{
			sourcePath:  sourcePath,
			archivePath: name,
		})
		return nil
	}
}

func WithContent(name string, content []byte) CreateOption {
	content = bytes.Clone(content)
	return func(options *createOptions) error {
		options.sources = append(options.sources, createSource{
			archivePath: name,
			content:     content,
			isContent:   true,
		})
		return nil
	}
}

func WithCompressionLevel(level CompressionLevel) CreateOption {
	return func(options *createOptions) error {
		if level != Fast && level != Default && level != Best {
			return fmt.Errorf("invalid compression level %d", level)
		}
		options.level = level
		return nil
	}
}

func Create(targetPath string, format Format, optionList ...CreateOption) error {
	var value createFormat
	switch format {
	case FormatZip:
		value = createFormatZip
	case FormatTar:
		value = createFormatTar
	case FormatTarGz:
		value = createFormatTarGz
	default:
		return fmt.Errorf("%w: %q", ErrUnsupportedFormat, format)
	}
	return create(targetPath, value, optionList)
}

func CreateTar(targetPath string, optionList ...CreateOption) error {
	return Create(targetPath, FormatTar, optionList...)
}

func create(targetPath string, format createFormat, optionList []CreateOption) error {
	options := createOptions{level: Default}
	for _, option := range optionList {
		if option == nil {
			return errors.New("create option must not be nil")
		}
		if err := option(&options); err != nil {
			return err
		}
	}

	entries, err := collectCreateEntries(options.sources)
	if err != nil {
		return err
	}

	targetDirectory := filepath.Dir(targetPath)
	if err = os.MkdirAll(targetDirectory, 0o755); err != nil {
		return fmt.Errorf("create archive target directory: %w", err)
	}
	if err = validateArchiveTarget(targetPath); err != nil {
		return err
	}

	temporaryFile, err := os.CreateTemp(targetDirectory, "."+filepath.Base(targetPath)+"-*")
	if err != nil {
		return fmt.Errorf("create temporary archive: %w", err)
	}
	temporaryPath := temporaryFile.Name()
	defer func() {
		_ = temporaryFile.Close()
		_ = os.Remove(temporaryPath)
	}()

	switch format {
	case createFormatZip:
		err = writeZip(temporaryFile, entries, options.level)
	case createFormatTarGz:
		err = writeTarGz(temporaryFile, entries, options.level)
	case createFormatTar:
		err = writeTar(temporaryFile, entries)
	}
	if err != nil {
		return fmt.Errorf("create archive: %w", err)
	}
	if err = temporaryFile.Sync(); err != nil {
		return fmt.Errorf("sync temporary archive: %w", err)
	}
	if err = temporaryFile.Chmod(0o644); err != nil {
		return fmt.Errorf("set archive permissions: %w", err)
	}
	if err = temporaryFile.Close(); err != nil {
		return fmt.Errorf("close temporary archive: %w", err)
	}
	if err = validateArchiveTarget(targetPath); err != nil {
		return err
	}
	if err = os.Rename(temporaryPath, targetPath); err != nil {
		return fmt.Errorf("replace archive target: %w", err)
	}
	return nil
}

type createFormat int

const (
	createFormatZip createFormat = iota
	createFormatTarGz
	createFormatTar
)

func validateArchiveTarget(targetPath string) error {
	info, err := os.Lstat(targetPath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect archive target: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("archive target %q is not a regular file", targetPath)
	}
	if hasMultipleHardLinks(info) {
		return fmt.Errorf("archive target %q is a hard link", targetPath)
	}
	return nil
}

func collectCreateEntries(sources []createSource) ([]createEntry, error) {
	entries := make([]createEntry, 0)
	names := make(map[string]struct{})
	for _, source := range sources {
		var sourceEntries []createEntry
		var err error
		if source.isContent {
			sourceEntries, err = collectContentEntry(source)
		} else {
			sourceEntries, err = collectFileEntries(source)
		}
		if err != nil {
			return nil, err
		}
		for _, entry := range sourceEntries {
			if _, exists := names[entry.nameInArchive]; exists {
				return nil, fmt.Errorf("duplicate archive path %q", entry.nameInArchive)
			}
			names[entry.nameInArchive] = struct{}{}
			entries = append(entries, entry)
		}
	}
	return entries, nil
}

func collectContentEntry(source createSource) ([]createEntry, error) {
	name, err := normalizeArchivePath(source.archivePath)
	if err != nil {
		return nil, fmt.Errorf("invalid content archive path: %w", err)
	}
	content := source.content
	return []createEntry{{
		info:          memoryFileInfo{name: path.Base(name), size: int64(len(content))},
		nameInArchive: name,
		open: func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(content)), nil
		},
	}}, nil
}

func collectFileEntries(source createSource) ([]createEntry, error) {
	rootInfo, err := os.Lstat(source.sourcePath)
	if err != nil {
		return nil, fmt.Errorf("inspect archive source %q: %w", source.sourcePath, err)
	}
	if err = validateSourceInfo(source.sourcePath, rootInfo); err != nil {
		return nil, err
	}

	rootName := source.archivePath
	if rootName == "" {
		rootName = filepath.Base(filepath.Clean(source.sourcePath))
	}
	rootName, err = normalizeArchivePath(rootName)
	if err != nil {
		return nil, fmt.Errorf("invalid archive path for %q: %w", source.sourcePath, err)
	}

	if rootInfo.Mode().IsRegular() {
		return []createEntry{newDiskEntry(source.sourcePath, rootName, rootInfo)}, nil
	}

	entries := make([]createEntry, 0)
	err = filepath.WalkDir(source.sourcePath, func(filePath string, directoryEntry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := directoryEntry.Info()
		if err != nil {
			return err
		}
		if err = validateSourceInfo(filePath, info); err != nil {
			return err
		}
		relativePath, err := filepath.Rel(source.sourcePath, filePath)
		if err != nil {
			return err
		}
		name := rootName
		if relativePath != "." {
			name = path.Join(rootName, filepath.ToSlash(relativePath))
		}
		name, err = normalizeArchivePath(name)
		if err != nil {
			return fmt.Errorf("invalid archive path for %q: %w", filePath, err)
		}
		entries = append(entries, newDiskEntry(filePath, name, info))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("collect archive source %q: %w", source.sourcePath, err)
	}
	return entries, nil
}

func validateSourceInfo(filePath string, info fs.FileInfo) error {
	if !info.IsDir() && !info.Mode().IsRegular() {
		return fmt.Errorf("archive source %q is not a regular file or directory", filePath)
	}
	if info.Mode().IsRegular() && hasMultipleHardLinks(info) {
		return fmt.Errorf("archive source %q is a hard link", filePath)
	}
	return nil
}

func newDiskEntry(filePath, name string, info fs.FileInfo) createEntry {
	return createEntry{
		info:          info,
		nameInArchive: name,
		open: func() (io.ReadCloser, error) {
			currentInfo, err := os.Lstat(filePath)
			if err != nil {
				return nil, err
			}
			if err = validateSourceInfo(filePath, currentInfo); err != nil {
				return nil, err
			}
			if !os.SameFile(info, currentInfo) {
				return nil, fmt.Errorf("archive source %q changed while creating archive", filePath)
			}
			file, err := os.Open(filePath)
			if err != nil {
				return nil, err
			}
			openedInfo, err := file.Stat()
			if err != nil {
				_ = file.Close()
				return nil, err
			}
			if !openedInfo.Mode().IsRegular() || !os.SameFile(info, openedInfo) {
				_ = file.Close()
				return nil, fmt.Errorf("archive source %q changed while creating archive", filePath)
			}
			return file, nil
		},
	}
}

func writeZip(writer io.Writer, entries []createEntry, level CompressionLevel) error {
	zipWriter := zip.NewWriter(writer)
	zipWriter.RegisterCompressor(zip.Deflate, func(writer io.Writer) (io.WriteCloser, error) {
		return flate.NewWriter(writer, flateLevel(level))
	})
	for _, entry := range entries {
		header, err := zip.FileInfoHeader(entry.info)
		if err != nil {
			_ = zipWriter.Close()
			return err
		}
		header.Name = entry.nameInArchive
		if entry.info.IsDir() {
			header.Name += "/"
			header.Method = zip.Store
		} else {
			header.Method = zip.Deflate
		}
		entryWriter, err := zipWriter.CreateHeader(header)
		if err != nil {
			_ = zipWriter.Close()
			return err
		}
		if entry.info.IsDir() {
			continue
		}
		if err = copyCreateEntry(entryWriter, entry); err != nil {
			_ = zipWriter.Close()
			return err
		}
	}
	return zipWriter.Close()
}

func writeTarGz(writer io.Writer, entries []createEntry, level CompressionLevel) error {
	gzipWriter, err := gzip.NewWriterLevel(writer, flateLevel(level))
	if err != nil {
		return err
	}
	tarWriter := tar.NewWriter(gzipWriter)
	for _, entry := range entries {
		header, err := tar.FileInfoHeader(entry.info, "")
		if err != nil {
			_ = tarWriter.Close()
			_ = gzipWriter.Close()
			return err
		}
		header.Name = entry.nameInArchive
		if err = tarWriter.WriteHeader(header); err != nil {
			_ = tarWriter.Close()
			_ = gzipWriter.Close()
			return err
		}
		if entry.info.IsDir() {
			continue
		}
		if err = copyCreateEntry(tarWriter, entry); err != nil {
			_ = tarWriter.Close()
			_ = gzipWriter.Close()
			return err
		}
	}
	if err = tarWriter.Close(); err != nil {
		_ = gzipWriter.Close()
		return err
	}
	return gzipWriter.Close()
}

func writeTar(writer io.Writer, entries []createEntry) error {
	tarWriter := tar.NewWriter(writer)
	for _, entry := range entries {
		header, err := tar.FileInfoHeader(entry.info, "")
		if err != nil {
			_ = tarWriter.Close()
			return err
		}
		header.Name = entry.nameInArchive
		if err = tarWriter.WriteHeader(header); err != nil {
			_ = tarWriter.Close()
			return err
		}
		if entry.info.IsDir() {
			continue
		}
		if err = copyCreateEntry(tarWriter, entry); err != nil {
			_ = tarWriter.Close()
			return err
		}
	}
	return tarWriter.Close()
}

func copyCreateEntry(writer io.Writer, entry createEntry) error {
	reader, err := entry.open()
	if err != nil {
		return err
	}
	_, err = io.Copy(writer, reader)
	if err != nil {
		_ = reader.Close()
		return err
	}
	return reader.Close()
}

func flateLevel(level CompressionLevel) int {
	switch level {
	case Fast:
		return flate.BestSpeed
	case Best:
		return flate.BestCompression
	default:
		return flate.DefaultCompression
	}
}

func hasMultipleHardLinks(info fs.FileInfo) bool {
	value := reflect.ValueOf(info.Sys())
	if !value.IsValid() {
		return false
	}
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return false
		}
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return false
	}
	field := value.FieldByName("Nlink")
	if !field.IsValid() {
		return false
	}
	switch field.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return field.Int() > 1
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return field.Uint() > 1
	default:
		return false
	}
}

type memoryFileInfo struct {
	name string
	size int64
}

func (info memoryFileInfo) Name() string       { return info.name }
func (info memoryFileInfo) Size() int64        { return info.size }
func (info memoryFileInfo) Mode() fs.FileMode  { return 0o644 }
func (info memoryFileInfo) ModTime() time.Time { return time.Time{} }
func (info memoryFileInfo) IsDir() bool        { return false }
func (info memoryFileInfo) Sys() any           { return nil }
