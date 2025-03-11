package docker

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"
)

func NewFileImport(containerRootPath string, opts ...ImportFileOption) (*ImportFile, error) {
	buffer := new(bytes.Buffer)
	o := &ImportFile{
		containerRootPath: containerRootPath,
		tar:               tar.NewWriter(buffer),
	}
	for _, opt := range opts {
		err := opt(o)
		if err != nil {
			return nil, err
		}
	}
	o.Reader = buffer
	return o, nil
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
		defer func() {
			_ = file.Close()
		}()
		fileInfo, _ := file.Stat()
		if fileName == "" {
			fileName = filepath.Base(sourcePath)
		}
		err = self.tar.WriteHeader(&tar.Header{
			Name:    filepath.Join(self.containerRootPath, fileName),
			Size:    fileInfo.Size(),
			Mode:    int64(fileInfo.Mode()),
			ModTime: fileInfo.ModTime(),
		})
		if err != nil {
			return err
		}
		_, err = io.Copy(self.tar, file)
		return err
	}
}

func WithImportContent(containerTargetPath string, content []byte, perm os.FileMode) ImportFileOption {
	return func(self *ImportFile) (err error) {
		err = self.tar.WriteHeader(&tar.Header{
			Name:    filepath.Join(self.containerRootPath, containerTargetPath),
			Size:    int64(len(content)),
			Mode:    int64(perm),
			ModTime: time.Now(),
		})
		if err != nil {
			return err
		}
		_, err = self.tar.Write(content)
		return err
	}
}

func WithImportPath(rootPath string) ImportFileOption {
	return func(self *ImportFile) (err error) {
		err = filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
			containerTargetPath, _ := filepath.Rel(rootPath, path)
			if info.IsDir() {
				err = self.tar.WriteHeader(&tar.Header{
					Typeflag: tar.TypeDir,
					Name:     filepath.Join(self.containerRootPath, containerTargetPath),
					Size:     0,
					Mode:     int64(os.ModePerm),
				})
			} else {
				err = self.tar.WriteHeader(&tar.Header{
					Name:    filepath.Join(self.containerRootPath, containerTargetPath),
					Size:    info.Size(),
					Mode:    int64(info.Mode()),
					ModTime: info.ModTime(),
				})
				if err != nil {
					return err
				}
				file, err := os.Open(path)
				if err != nil {
					return err
				}
				_, err = io.Copy(self.tar, file)
			}
			return nil
		})
		if err != nil {
			return err
		}
		return err
	}
}

func WithImportZip(reader *zip.ReadCloser) ImportFileOption {
	return func(self *ImportFile) (err error) {
		if reader == nil {
			return errors.New("zip reader is nil")
		}
		defer func() {
			_ = reader.Close()
		}()
		for _, file := range reader.File {
			if file.FileInfo().IsDir() {
				err = self.tar.WriteHeader(&tar.Header{
					Typeflag: tar.TypeDir,
					Name:     filepath.Join(self.containerRootPath, file.Name),
					Size:     0,
					Mode:     int64(os.ModePerm),
				})
			} else {
				err = self.tar.WriteHeader(&tar.Header{
					Name:    filepath.Join(self.containerRootPath, file.Name),
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
				_, err = io.Copy(self.tar, fr)
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
		return WithImportZip(reader)(self)
	}
}

func WithImportTar(reader *tar.Reader) ImportFileOption {
	return func(self *ImportFile) (err error) {
		for {
			header, err := reader.Next()
			if err == io.EOF {
				break
			}
			header.Name = filepath.Join(self.containerRootPath, header.Name)
			err = self.tar.WriteHeader(header)
			if err != nil {
				return err
			}
			_, err = io.Copy(self.tar, reader)
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
