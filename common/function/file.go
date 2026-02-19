package function

import (
	"errors"
	"io"
	"os"
	"path/filepath"
)

func FileExists(file ...string) bool {
	var err error
	for _, value := range file {
		if _, err = os.Stat(value); err != nil {
			return false
		}
	}
	return true
}

func CopyFileFromPath(targetPath string, sourceFile ...string) error {
	if stat, err := os.Stat(targetPath); err == nil && !stat.IsDir() {
		return errors.New("targetPath not dir")
	}
	_ = os.MkdirAll(targetPath, os.ModePerm)
	for _, name := range sourceFile {
		err := func() error {
			sf, err := os.Open(name)
			if err != nil {
				return err
			}
			defer func() {
				_ = sf.Close()
			}()
			sfStat, _ := sf.Stat()
			tf, err := os.OpenFile(filepath.Join(targetPath, filepath.Base(sf.Name())), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, sfStat.Mode())
			if err != nil {
				return err
			}
			defer func() {
				_ = tf.Close()
			}()
			_, err = io.Copy(tf, sf)
			return err
		}()
		if err != nil {
			return err
		}
	}
	return nil
}

func CopyDir(targetPath, sourcePath string) error {
	if stat, err := os.Stat(targetPath); err == nil && !stat.IsDir() {
		return errors.New("targetPath not dir")
	}
	_ = os.MkdirAll(targetPath, os.ModePerm)
	return os.CopyFS(targetPath, os.DirFS(sourcePath))
}

func FileSeekToLastNLines(file *os.File, n int) error {
	stat, err := file.Stat()
	if err != nil {
		return err
	}
	size := stat.Size()
	if size == 0 {
		return nil
	}

	var chunkSize int64 = 4096
	var offset = size
	var lineCount int
	buf := make([]byte, chunkSize)

	for offset > 0 && lineCount <= n {
		readSize := chunkSize
		if offset < chunkSize {
			readSize = offset
		}
		offset -= readSize
		_, err := file.Seek(offset, io.SeekStart)
		if err != nil {
			return err
		}

		_, err = io.ReadFull(file, buf[:readSize])
		if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
			return err
		}

		for i := int(readSize) - 1; i >= 0; i-- {
			if buf[i] == '\n' {
				lineCount++
				if lineCount == n+1 {
					_, err = file.Seek(offset+int64(i)+1, io.SeekStart)
					return err
				}
			}
		}
	}
	_, err = file.Seek(0, io.SeekStart)
	return err
}
