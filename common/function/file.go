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

func CopyFile(targetPath string, sourceFile ...string) error {
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
