package function

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func Unzip(targetPath, sourceZipPath string) error {
	r, err := zip.OpenReader(sourceZipPath)
	if err != nil {
		return fmt.Errorf("failed to open zip file %s: %w", sourceZipPath, err)
	}
	defer r.Close()

	if err := os.MkdirAll(targetPath, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create target directory %s: %w", targetPath, err)
	}

	for _, f := range r.File {
		err := extractFile(f, targetPath)
		if err != nil {
			return fmt.Errorf("failed to extract %s: %w", f.Name, err)
		}
	}

	return nil
}

func extractFile(f *zip.File, targetDir string) error {
	fpath := filepath.Join(targetDir, filepath.FromSlash(f.Name))
	if !strings.HasPrefix(fpath, filepath.Clean(targetDir)+string(filepath.Separator)) {
		return fmt.Errorf("illegal file path: %s", f.Name)
	}
	if f.FileInfo().IsDir() {
		return os.MkdirAll(fpath, f.Mode())
	}
	if err := os.MkdirAll(filepath.Dir(fpath), 0755); err != nil {
		return err
	}
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
	if err != nil {
		return err
	}
	defer outFile.Close()
	_, err = io.Copy(outFile, rc)
	return err
}
