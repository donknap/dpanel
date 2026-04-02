package imports

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"io"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

func getTarName(paths ...string) string {
	return strings.TrimPrefix(path.Join(paths...), "/")
}

func WithImportFile(file *os.File, fileName string) ImportFileOption {
	return func(self *ImportFile) (err error) {
		if file == nil {
			return errors.New("invalid file")
		}
		defer func() {
			_ = file.Close()
		}()
		fileInfo, _ := file.Stat()
		err = self.tarWrite.WriteHeader(&tar.Header{
			Name:    getTarName(self.targetRootPath, fileName),
			Size:    fileInfo.Size(),
			Mode:    int64(fileInfo.Mode()),
			ModTime: fileInfo.ModTime(),
		})
		if err != nil {
			return err
		}
		_, err = io.Copy(self.tarWrite, file)
		return nil
	}
}

func WithImportFilePath(sourcePath string, fileName string) ImportFileOption {
	return func(self *ImportFile) (err error) {
		if _, err = os.Stat(sourcePath); err != nil {
			return err
		}
		file, err := os.Open(sourcePath)
		if err != nil {
			return err
		}
		if fileName == "" {
			fileName = path.Base(sourcePath)
		}
		return WithImportFile(file, fileName)(self)
	}
}

func WithImportContent(containerTargetPath string, content []byte, perm os.FileMode) ImportFileOption {
	return func(self *ImportFile) (err error) {
		err = self.tarWrite.WriteHeader(&tar.Header{
			Name:    getTarName(self.targetRootPath, containerTargetPath),
			Size:    int64(len(content)),
			Mode:    int64(perm),
			ModTime: time.Now(),
		})
		if err != nil {
			return err
		}
		_, err = self.tarWrite.Write(content)
		return err
	}
}

func WithImportPath(rootPath string) ImportFileOption {
	return func(self *ImportFile) (err error) {
		if _, err = os.Stat(rootPath); err != nil {
			return err
		}
		err = filepath.Walk(rootPath, func(pathName string, info os.FileInfo, err error) error {
			realPath, _ := filepath.Rel(rootPath, pathName)
			containerTargetPath := filepath.ToSlash(realPath)
			headerName := getTarName(self.targetRootPath, containerTargetPath)
			if headerName == "" {
				return nil
			}
			if info.IsDir() {
				err = self.tarWrite.WriteHeader(&tar.Header{
					Typeflag: tar.TypeDir,
					Name:     headerName,
					Size:     0,
					Mode:     int64(info.Mode()),
				})
			} else {
				err = self.tarWrite.WriteHeader(&tar.Header{
					Name:    headerName,
					Size:    info.Size(),
					Mode:    int64(info.Mode()),
					ModTime: info.ModTime(),
				})
				if err != nil {
					return err
				}
				file, err := os.Open(pathName)
				if err != nil {
					return err
				}
				_, err = io.Copy(self.tarWrite, file)
				_ = file.Close()
			}
			return nil
		})
		if err != nil {
			return err
		}
		return err
	}
}

func WithImportZip(reader *zip.Reader) ImportFileOption {
	return func(self *ImportFile) (err error) {
		if reader == nil {
			return errors.New("zip reader is nil")
		}
		for _, file := range reader.File {
			if file.FileInfo().IsDir() {
				err = self.tarWrite.WriteHeader(&tar.Header{
					Typeflag: tar.TypeDir,
					Name:     getTarName(self.targetRootPath, file.Name),
					Size:     0,
					Mode:     int64(file.Mode()),
				})
			} else {
				err = self.tarWrite.WriteHeader(&tar.Header{
					Name:    getTarName(self.targetRootPath, file.Name),
					Size:    file.FileInfo().Size(),
					Mode:    int64(file.FileInfo().Mode()),
					ModTime: file.FileInfo().ModTime(),
				})
				if err != nil {
					return err
				}
				var fr io.ReadCloser
				fr, err = file.Open()
				if err != nil {
					return err
				}
				_, err = io.Copy(self.tarWrite, fr)
				if err != nil {
					return err
				}
				_ = fr.Close()
			}
		}
		return nil
	}
}

func WithImportZipFile(zipPath string) ImportFileOption {
	return func(self *ImportFile) (err error) {
		reader, err := zip.OpenReader(zipPath)
		if err != nil {
			return err
		}
		defer func() {
			_ = reader.Close()
		}()

		return WithImportZip(&reader.Reader)(self)
	}
}

func WithImportTar(reader *tar.Reader) ImportFileOption {
	return func(self *ImportFile) (err error) {
		for {
			header, err := reader.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				slog.Warn("docker import file", "error", err)
				break
			}
			header.Name = getTarName(self.targetRootPath, header.Name)
			err = self.tarWrite.WriteHeader(header)
			if err != nil {
				return err
			}
			_, err = io.Copy(self.tarWrite, reader)
			if err != nil {
				return err
			}
		}
		return nil
	}
}

func WithImportTarFile(tarPath string) ImportFileOption {
	return func(self *ImportFile) (err error) {
		file, err := os.Open(tarPath)
		if err != nil {
			return err
		}
		reader := tar.NewReader(file)
		defer func() {
			_ = file.Close()
		}()
		return WithImportTar(reader)(self)
	}
}

func WithImportTarGzFile(tarPath string) ImportFileOption {
	return func(self *ImportFile) (err error) {
		file, err := os.Open(tarPath)
		if err != nil {
			return err
		}
		gzReader, err := gzip.NewReader(file)
		if err != nil {
			return err
		}
		defer func() {
			_ = gzReader.Close()
			_ = file.Close()
		}()
		reader := tar.NewReader(gzReader)
		return WithImportTar(reader)(self)
	}
}

func WithImportFileInTar(reader *tar.Reader, newFileName string, match func(header *tar.Header) bool) ImportFileOption {
	return func(self *ImportFile) (err error) {
		if match == nil {
			return nil
		}
		for {
			header, err := reader.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}
			if match(header) {
				header.Name = getTarName(self.targetRootPath, newFileName)
				if err := self.tarWrite.WriteHeader(header); err != nil {
					return err
				}
				if _, err := io.Copy(self.tarWrite, reader); err != nil {
					return err
				}
				break
			}
		}
		return nil
	}
}
