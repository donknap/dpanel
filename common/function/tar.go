package function

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path"
	"path/filepath"
)

func Tar(destFile string, srcPaths []string, internalRootPath string, useGz bool, ignoreCallback func(path string, info os.FileInfo) bool) error {
	file, err := os.Create(destFile)
	if err != nil {
		return err
	}
	defer file.Close()

	var outWriter io.Writer = file
	if useGz {
		gw := gzip.NewWriter(file)
		outWriter = gw
		defer gw.Close()
	}

	tw := tar.NewWriter(outWriter)
	defer tw.Close()

	absDestFile, err := filepath.Abs(destFile)
	if err != nil {
		absDestFile = destFile
	}

	for _, srcPath := range srcPaths {
		cleanSrc := filepath.Clean(srcPath)
		baseDir := filepath.Dir(cleanSrc)

		err = filepath.Walk(cleanSrc, func(currentPath string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			absCurrent, _ := filepath.Abs(currentPath)

			if absCurrent == absDestFile {
				return nil
			}

			// 核心逻辑：执行忽略回调判断
			if ignoreCallback != nil && ignoreCallback(absCurrent, info) {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			relPath, err := filepath.Rel(baseDir, currentPath)
			if err != nil {
				return err
			}

			tarName := filepath.ToSlash(relPath)

			if internalRootPath != "" {
				tarName = path.Join(internalRootPath, tarName)
			}

			header, err := tar.FileInfoHeader(info, info.Name())
			if err != nil {
				return err
			}
			header.Name = tarName

			if err := tw.WriteHeader(header); err != nil {
				return err
			}

			if info.Mode().IsRegular() {
				f, err := os.Open(currentPath)
				if err != nil {
					return err
				}
				defer f.Close()

				if _, err := io.Copy(tw, f); err != nil {
					return err
				}
			}

			return nil
		})

		if err != nil {
			return err
		}
	}

	return nil
}
