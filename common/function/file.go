package function

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
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

type CopyMode int

const (
	CopyModeError CopyMode = iota
	CopyModeOverwrite
	CopyModeSkip
	CopyModeKeepBoth
)

type CopyDirOption struct {
	Mode CopyMode
	// 使用源目录内以 / 分隔的精确相对路径，不匹配子目录中的同名文件。
	ExcludeFileList []string
}

// CopyDir 返回根目录以下本次新建的路径，失败时也返回，供调用方按逆序精确清理。
// 同名目录合并；Mode 只处理同名普通文件，符号链接和文件类型冲突返回错误。
func CopyDir(targetPath, sourcePath string, option CopyDirOption) ([]string, error) {
	if option.Mode < CopyModeError || option.Mode > CopyModeKeepBoth {
		return nil, fmt.Errorf("invalid copy mode %d", option.Mode)
	}
	source, err := os.OpenRoot(sourcePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = source.Close() }()
	if err = os.MkdirAll(targetPath, os.ModePerm); err != nil {
		return nil, err
	}
	info, err := os.Lstat(targetPath)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, os.ErrPermission
	}
	target, err := os.OpenRoot(targetPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = target.Close() }()
	var fileList []string
	err = fs.WalkDir(source.FS(), ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if name == "." {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if err = target.Mkdir(name, info.Mode().Perm()|0700); err != nil {
				if !errors.Is(err, os.ErrExist) {
					return err
				}
				info, err = target.Lstat(name)
				if err != nil {
					return err
				}
				if !info.IsDir() {
					return &os.PathError{Op: "copy", Path: name, Err: os.ErrPermission}
				}
			} else {
				fileList = append(fileList, filepath.Join(targetPath, name))
			}
			return nil
		}
		if !info.Mode().IsRegular() {
			return &os.PathError{Op: "copy", Path: name, Err: os.ErrPermission}
		}
		if InArray(option.ExcludeFileList, name) {
			return nil
		}
		targetName := name
		copyName := name
		file, err := target.OpenFile(copyName, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
		if errors.Is(err, os.ErrExist) {
			var targetInfo os.FileInfo
			targetInfo, err = target.Lstat(name)
			if err != nil {
				return err
			}
			if !targetInfo.Mode().IsRegular() {
				return &os.PathError{Op: "copy", Path: name, Err: os.ErrPermission}
			}
			switch option.Mode {
			case CopyModeError:
				return &os.PathError{Op: "copy", Path: name, Err: os.ErrExist}
			case CopyModeSkip:
				return nil
			case CopyModeOverwrite:
				// 完整复制后再替换，读取或写入失败不破坏旧文件，也不沿旧文件的硬链接写入。
				copyName = filepath.Join(filepath.Dir(name), ".dpanel-copy-"+uuid.NewString())
				file, err = target.OpenFile(copyName, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
			case CopyModeKeepBoth:
				ext := filepath.Ext(name)
				for index := 1; ; index++ {
					targetName = fmt.Sprintf("%s (%d)%s", strings.TrimSuffix(name, ext), index, ext)
					copyName = targetName
					file, err = target.OpenFile(copyName, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
					if !errors.Is(err, os.ErrExist) {
						break
					}
				}
			}
		}
		if err != nil {
			return err
		}
		if copyName == targetName {
			fileList = append(fileList, filepath.Join(targetPath, targetName))
		}
		sourceFile, err := source.Open(name)
		if err == nil {
			_, err = io.Copy(file, sourceFile)
			err = errors.Join(err, sourceFile.Close())
		}
		err = errors.Join(err, file.Close())
		if err == nil && copyName != targetName {
			err = target.Rename(copyName, targetName)
		}
		if err != nil {
			return errors.Join(err, target.Remove(copyName))
		}
		return nil
	})
	return fileList, err
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
