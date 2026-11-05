package sync

import (
	"archive/tar"
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	exec2 "os/exec"
	path2 "path"
	"path/filepath"
	"strings"
	"time"

	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/exec/local"
	"github.com/donknap/dpanel/common/service/oci"
)

func ByGit(ctx context.Context, source, targetPath, tempPath string, option GitOption) (err error) {
	if source == "" || targetPath == "" || tempPath == "" {
		return fmt.Errorf("source, target path and temporary path are required")
	}
	if _, err := exec2.LookPath("git"); err != nil {
		return fmt.Errorf("find git executable: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	var branch string
	gitUrl := source

	if b, a, ok := strings.Cut(gitUrl, "#"); ok {
		gitUrl = b
		branch = a
	} else {
		branch = ""
	}

	// 下载完成后才覆盖目标，避免下载失败破坏已有内容。
	if err = os.MkdirAll(tempPath, 0700); err != nil {
		return err
	}
	downloadPath, err := os.MkdirTemp(tempPath, "git-*")
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, os.RemoveAll(downloadPath))
	}()

	slog.Debug("sync git download", "path", downloadPath)

	args := []string{
		"clone", "--depth", "1",
	}
	if branch != "" {
		args = append(args, "-b", branch)
	}
	args = append(args, gitUrl, downloadPath)
	cmd, err := local.New(
		local.WithCommandName("git"),
		local.WithArgs(args...),
		local.WithEnv(os.Environ()),
		local.WithCtx(ctx),
	)
	if err != nil {
		return err
	}
	defer cmd.Close()
	_, err = cmd.RunWithResult()
	if err != nil {
		return err
	}
	if option.RemoveGit {
		if err = os.RemoveAll(filepath.Join(downloadPath, ".git")); err != nil {
			return err
		}
	}
	if option.MaxBytes > 0 {
		var total int64
		err = filepath.WalkDir(downloadPath, func(file string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if err = ctx.Err(); err != nil {
				return err
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if !entry.IsDir() && !info.Mode().IsRegular() {
				return fmt.Errorf("unsupported Git entry %s", file)
			}
			total += info.Size()
			if total > option.MaxBytes {
				return fmt.Errorf("Git repository exceeds %d bytes", option.MaxBytes)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	_, err = function.CopyDir(targetPath, downloadPath, function.CopyDirOption{Truncate: true})
	return err
}

// ByZip 同步远程 zip
// Root 只筛选归档内目录，解压时保留该目录前缀。
func ByZip(ctx context.Context, source, targetPath, tempPath string, option ZipOption) (err error) {
	if source == "" || targetPath == "" || tempPath == "" {
		return fmt.Errorf("source, target path and temporary path are required")
	}
	if err = os.MkdirAll(tempPath, 0700); err != nil {
		return err
	}
	zipTempFile, err := os.CreateTemp(tempPath, "zip-*.zip")
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, os.Remove(zipTempFile.Name()))
	}()

	if err = zipTempFile.Close(); err != nil {
		return err
	}
	if err = ByUrl(ctx, source, zipTempFile.Name(), UrlOption{}); err != nil {
		return err
	}
	zipArchive, err := zip.OpenReader(zipTempFile.Name())
	if err != nil {
		return err
	}
	defer func() {
		_ = zipArchive.Close()
	}()
	if err = os.MkdirAll(targetPath, os.ModePerm); err != nil {
		return err
	}
	root, err := os.OpenRoot(targetPath)
	if err != nil {
		return err
	}
	defer root.Close()
	for _, file := range zipArchive.File {
		if file.FileInfo().IsDir() {
			continue
		}
		if strings.HasPrefix(file.Name, "__MACOSX") {
			continue
		}

		archiveFilePath := strings.TrimPrefix(path2.Clean("/"+file.Name), "/")
		if archiveFilePath == "" || archiveFilePath == "." {
			continue
		}
		if option.Root != "" {
			rootPath := strings.TrimPrefix(path2.Clean("/"+option.Root), "/")
			if archiveFilePath != rootPath && !strings.HasPrefix(archiveFilePath, rootPath+"/") {
				continue
			}
		}
		if option.StripRoot && option.Root != "" {
			rootPath := strings.TrimPrefix(path2.Clean("/"+option.Root), "/")
			archiveFilePath = strings.TrimPrefix(archiveFilePath, rootPath+"/")
		}
		targetFilePath := filepath.FromSlash(archiveFilePath)
		err = func() (err error) {
			err = root.MkdirAll(filepath.Dir(targetFilePath), os.ModePerm)
			if err != nil {
				return err
			}
			targetFile, err := root.OpenFile(targetFilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
			if err != nil {
				return err
			}
			defer func() {
				err = errors.Join(err, targetFile.Close())
			}()

			sourceFile, err := file.Open()
			if err != nil {
				return err
			}
			defer func() {
				_ = sourceFile.Close()
			}()

			_, err = io.Copy(targetFile, sourceFile)
			return err
		}()
		if err != nil {
			return err
		}
	}
	return nil
}

func ByOci(ctx context.Context, source, targetPath, tempPath string, option OciOption) (err error) {
	if source == "" || targetPath == "" || tempPath == "" {
		return fmt.Errorf("source, target path and temporary path are required")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	if err = os.MkdirAll(tempPath, 0700); err != nil {
		return err
	}
	archiveFile, err := os.CreateTemp(tempPath, "oci-*.tar")
	if err != nil {
		return err
	}
	archivePath := archiveFile.Name()
	defer func() {
		err = errors.Join(err, os.Remove(archivePath))
	}()
	if err = archiveFile.Close(); err != nil {
		return err
	}

	labels, err := oci.PullImage(ctx, source, archivePath, oci.PullOption{
		Platform:     option.Platform,
		DockerClient: option.DockerClient,
	})
	if err != nil {
		return err
	}
	for name, value := range option.ExpectedLabels {
		if labels[name] != value {
			return fmt.Errorf("OCI image %s has invalid %s label", source, name)
		}
	}
	archiveFile, err = os.Open(archivePath)
	if err != nil {
		return err
	}
	defer archiveFile.Close()
	if err = os.MkdirAll(targetPath, 0700); err != nil {
		return err
	}
	root, err := os.OpenRoot(targetPath)
	if err != nil {
		return err
	}
	defer root.Close()
	reader := tar.NewReader(archiveFile)
	for {
		if err = ctx.Err(); err != nil {
			return err
		}
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		name := strings.TrimPrefix(header.Name, "./")
		if name == "" && header.Typeflag == tar.TypeDir {
			continue
		}
		if name == "" || path2.IsAbs(name) || path2.Clean(name) == ".." || strings.HasPrefix(path2.Clean(name), "../") || strings.Contains(name, "\\") || !filepath.IsLocal(filepath.FromSlash(name)) {
			return fmt.Errorf("invalid OCI source archive path %q", header.Name)
		}
		name = filepath.FromSlash(name)
		var content io.Reader = reader
		size := header.Size
		switch header.Typeflag {
		case tar.TypeDir:
			if err = root.MkdirAll(name, 0700); err != nil {
				return err
			}
			continue
		case tar.TypeReg, tar.TypeRegA:
		case tar.TypeSymlink:
			// 源镜像只提供仓库文件，链接保存为文本，避免解包后访问源目录之外。
			content = strings.NewReader(header.Linkname)
			size = int64(len(header.Linkname))
		default:
			return fmt.Errorf("unsupported OCI source archive entry %q", header.Name)
		}
		if size < 0 {
			return fmt.Errorf("invalid OCI source entry size for %q", header.Name)
		}
		if err = root.MkdirAll(filepath.Dir(name), 0700); err != nil {
			return err
		}
		file, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return err
		}
		_, err = io.CopyN(file, content, size)
		closeErr := file.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
}

func ByUrl(ctx context.Context, source, targetPath string, option UrlOption) error {
	if source == "" || targetPath == "" {
		return fmt.Errorf("source and target path are required")
	}
	if option.Timeout < 0 || option.MaxBytes < 0 {
		return fmt.Errorf("URL source timeout and size limit must not be negative")
	}
	if option.Timeout == 0 {
		option.Timeout = 30 * time.Minute
	}
	if option.MaxBytes == 0 {
		option.MaxBytes = 20 << 30
	}
	response, err := function.SafeHTTPGet(ctx, source, option.Timeout, option.MaxBytes)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("source download returned %s", response.Status)
	}
	flag := os.O_WRONLY | os.O_TRUNC
	if !option.RequireExistingFile {
		if err = os.MkdirAll(filepath.Dir(targetPath), os.ModePerm); err != nil {
			return err
		}
		flag |= os.O_CREATE
	}
	file, err := os.OpenFile(targetPath, flag, 0666)
	if err != nil {
		return err
	}
	_, err = io.Copy(file, response.Body)
	return errors.Join(err, file.Close())
}
