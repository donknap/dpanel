package logic

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
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/define"
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

type SyncOption struct {
	Context context.Context
}

type GitOption struct {
	SyncOption
	TempDownloadPath string
	RemoveGit        bool
	MaxBytes         int64
}

type ZipOption struct {
	SyncOption
	Root string
}

type OciOption struct {
	SyncOption
	Platform       v1.Platform
	ExpectedLabels map[string]string
}

func (self Store) SyncByGit(source, targetPath string, option GitOption) error {
	if source == "" || targetPath == "" {
		return fmt.Errorf("source and target path are required")
	}
	if _, err := exec2.LookPath("git"); err != nil {
		return function.ErrorMessage(define.ErrorMessageSystemStoreNotFoundGit)
	}
	ctx := option.Context
	if ctx == nil {
		ctx = context.Background()
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

	// 先创建一个临时目录，下载完成后再同步数据，否则失败时原先的数据会被删除
	if option.TempDownloadPath == "" {
		// 仅当内部生成临时目录的时候才删除，如果是外部传递的，由外部来维护
		var err error
		option.TempDownloadPath, err = storage.Local{}.CreateTempDir("")
		if err != nil {
			return err
		}
		defer func() {
			_ = os.RemoveAll(option.TempDownloadPath)
		}()
	}

	slog.Debug("store git download", "path", option.TempDownloadPath)

	args := []string{
		"clone", "--depth", "1",
	}
	if branch != "" {
		args = append(args, "-b", branch)
	}
	args = append(args, gitUrl, option.TempDownloadPath)
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
		if err = os.RemoveAll(filepath.Join(option.TempDownloadPath, ".git")); err != nil {
			return err
		}
	}
	if option.MaxBytes > 0 {
		var total int64
		err = filepath.WalkDir(option.TempDownloadPath, func(file string, entry fs.DirEntry, err error) error {
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
	if filepath.Clean(option.TempDownloadPath) != filepath.Clean(targetPath) {
		err = os.RemoveAll(targetPath)
		if err != nil {
			return err
		}
		err = os.CopyFS(targetPath, os.DirFS(option.TempDownloadPath))
		if err != nil {
			return err
		}
	}
	return nil
}

// SyncByZip 同步远程 zip
// Root 只筛选归档内目录，解压时保留该目录前缀。
func (self Store) SyncByZip(source, targetPath string, option ZipOption) error {
	if source == "" || targetPath == "" {
		return fmt.Errorf("source and target path are required")
	}
	zipTempFile, err := storage.Local{}.CreateTempFile("")
	if err != nil {
		return err
	}
	defer func() {
		_ = zipTempFile.Close()
		_ = os.RemoveAll(zipTempFile.Name())
	}()

	ctx := option.Context
	if ctx == nil {
		ctx = context.Background()
	}
	response, err := function.SafeHTTPGet(ctx, source, 30*time.Minute, 20<<30)
	if err != nil {
		return err
	}
	defer func() {
		_ = response.Body.Close()
	}()
	if response.StatusCode != http.StatusOK {
		return function.ErrorMessage(define.ErrorMessageSystemStoreDownloadFailed, "url", source, "error", response.Status)
	}
	_, err = io.Copy(zipTempFile, response.Body)
	if err != nil {
		return err
	}
	zipArchive, err := zip.OpenReader(zipTempFile.Name())
	if err != nil {
		return err
	}
	defer func() {
		_ = zipArchive.Close()
	}()
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
		targetFilePath := function.SafePathJoin(targetPath, archiveFilePath)
		err = func() error {
			err = os.MkdirAll(filepath.Dir(targetFilePath), os.ModePerm)
			if err != nil {
				return err
			}
			targetFile, err := os.OpenFile(targetFilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
			if err != nil {
				return err
			}
			defer func() {
				_ = targetFile.Close()
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

func (self Store) SyncByOci(source, targetPath string, maxBytes int64, maxEntry int, option OciOption) error {
	if source == "" || targetPath == "" {
		return fmt.Errorf("source and target path are required")
	}
	if maxBytes <= 0 || maxEntry <= 0 {
		return fmt.Errorf("OCI source size and entry limits are required")
	}
	ctx := option.Context
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	archiveFile, err := os.CreateTemp(storage.Local{}.GetLocalTempDir(), "dpanel-oci-*.tar")
	if err != nil {
		return err
	}
	archivePath := archiveFile.Name()
	if err = archiveFile.Close(); err != nil {
		_ = os.Remove(archivePath)
		return err
	}
	defer os.Remove(archivePath)

	// 文件内容之外，tar 还包含条目头、对齐填充和结束块。
	archiveMaxBytes := maxBytes + int64(maxEntry)*1024 + 1024
	labels, err := oci.PullImage(ctx, source, archivePath, option.Platform, archiveMaxBytes)
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
	var total int64
	for count := 0; ; count++ {
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
		if count >= maxEntry {
			return fmt.Errorf("OCI source archive has too many entries")
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
		if size < 0 || size > maxBytes-total {
			return fmt.Errorf("OCI source archive exceeds %d bytes", maxBytes)
		}
		total += size
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

func (self Store) SyncByUrl(targetPath, url string) error {
	_ = os.MkdirAll(filepath.Dir(targetPath), os.ModePerm)
	file, err := os.OpenFile(targetPath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0666)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()

	response, err := function.SafeHTTPGet(context.Background(), url, 30*time.Minute, 20<<30)
	if err != nil {
		return err
	}
	defer func() {
		_ = response.Body.Close()
	}()

	if response.StatusCode != http.StatusOK {
		return function.ErrorMessage(define.ErrorMessageSystemStoreDownloadFailed, "url", url, "error", response.Status)
	}
	_, err = io.Copy(file, response.Body)
	if err != nil {
		return err
	}
	return nil
}
