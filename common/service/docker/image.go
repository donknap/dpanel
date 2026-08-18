package docker

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	ioFs "io/fs"
	"log/slog"
	"math"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/donknap/dpanel/common/function"
	dockerTypes "github.com/donknap/dpanel/common/service/docker/types"
	"github.com/donknap/dpanel/common/types/fs"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	registrySdk "github.com/we7coreteam/registry-go-sdk"
)

type ImagePullOption struct {
	Registry   dockerTypes.Registry
	Platform   string
	OnProgress func(map[string]*dockerTypes.PullProgress)
}

type ImagePushOption struct {
	Registry   dockerTypes.Registry
	All        bool
	Platform   *ocispec.Platform
	OnProgress func(map[string]*dockerTypes.PullProgress)
}

type imageProgressReader struct {
	progressItems    map[string]*dockerTypes.PullProgress
	lastProgressTime time.Time
	onProgress       func(map[string]*dockerTypes.PullProgress)
}

func (self Client) ImagePull(ctx context.Context, imageName string, option ImagePullOption) (*function.Tag, error) {
	originalImageName := function.ImageTag(imageName)
	imageNameDetail := *originalImageName
	registryConfig := option.Registry
	if registryConfig.Address == "" {
		registryConfig.Address = imageNameDetail.Registry
	}

	proxyAddresses, sourceAddress := registryConfig.APIAddresses()
	var lastErr error
	registryOptions := make([]registrySdk.Option, 0, len(proxyAddresses)+1)
	for _, address := range proxyAddresses {
		registryOptions = append(registryOptions, registrySdk.WithServer(address, "", ""))
	}
	registryOptions = append(registryOptions, registrySdk.WithRepository(imageNameDetail.BaseName, imageNameDetail.Version))
	// 每个地址只有在完整消费 Docker 拉取进度流且未出现错误后才算成功；
	// 代理地址先并发检测目标 Manifest，再按可用结果的返回顺序尝试拉取；
	// 代理请求或进度流失败时继续回退，源仓库失败时返回最终错误。
	for proxyServer := range registrySdk.New(registryOptions...).GetAvailableServers() {
		address := proxyServer.Url
		imageNameDetail.Registry = function.RegistryReference(address)
		out, err := self.Client.ImagePull(ctx, imageNameDetail.Uri(), image.PullOptions{Platform: option.Platform})
		if err == nil {
			err = (&imageProgressReader{onProgress: option.OnProgress}).read(ctx, out)
			if err == nil {
				originalTag := originalImageName.Uri()
				if tag, _, ok := strings.Cut(originalTag, "@"); ok {
					originalTag = tag
				}
				if err = self.Client.ImageTag(ctx, imageNameDetail.Uri(), originalTag); err == nil {
					return originalImageName, nil
				}
			}
		}
		if ctx.Err() != nil {
			return originalImageName, ctx.Err()
		}
		lastErr = fmt.Errorf("pull image from %s: %w", address, err)
		slog.Debug("image remote", "type", "pull", "address", address, "error", err)
	}

	if sourceAddress != "" {
		imageNameDetail.Registry = function.RegistryReference(sourceAddress)
		out, err := self.Client.ImagePull(ctx, imageNameDetail.Uri(), image.PullOptions{Platform: option.Platform, RegistryAuth: registryConfig.Auth})
		if err == nil {
			err = (&imageProgressReader{onProgress: option.OnProgress}).read(ctx, out)
			if err == nil {
				originalTag := originalImageName.Uri()
				if tag, _, ok := strings.Cut(originalTag, "@"); ok {
					originalTag = tag
				}
				if imageNameDetail.Uri() == originalTag {
					return originalImageName, nil
				}
				if err = self.Client.ImageTag(ctx, imageNameDetail.Uri(), originalTag); err == nil {
					return originalImageName, nil
				}
			}
		}
		if ctx.Err() != nil {
			return originalImageName, ctx.Err()
		}
		lastErr = fmt.Errorf("pull image from %s: %w", sourceAddress, err)
	}
	if lastErr != nil {
		return originalImageName, lastErr
	}
	return originalImageName, errors.New("image pull failed: no registry address")
}

func (self Client) ImagePush(ctx context.Context, imageName string, option ImagePushOption) error {
	imageNameDetail := function.ImageTag(imageName)
	registryAuth := ""
	if targetAddress, err := function.ParseURL(imageNameDetail.Registry); err == nil {
		if sourceAddress, err := function.ParseURL(option.Registry.Address); err == nil && strings.EqualFold(targetAddress.Host, sourceAddress.Host) {
			registryAuth = option.Registry.Auth
		}
	}

	// 推送只访问镜像中指定的目标仓库，不使用代理地址，也不做地址回退；
	// 源仓库权限只允许用于同一个源仓库地址。
	reader, err := self.Client.ImagePush(ctx, imageName, image.PushOptions{
		All:          option.All,
		RegistryAuth: registryAuth,
		Platform:     option.Platform,
	})
	if err != nil {
		return err
	}
	progressReader := imageProgressReader{onProgress: option.OnProgress}
	return progressReader.read(ctx, reader)
}

func (self *imageProgressReader) read(ctx context.Context, reader io.ReadCloser) error {
	if reader == nil {
		return errors.New("docker image response stream is empty")
	}
	defer reader.Close()

	self.progressItems = make(map[string]*dockerTypes.PullProgress)
	self.lastProgressTime = time.Now()
	decoder := json.NewDecoder(reader)
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		progress := dockerTypes.ImageProgress{}
		err := decoder.Decode(&progress)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}
		if progress.ErrorDetail.Message != "" {
			return errors.New(progress.ErrorDetail.Message)
		}
		if self.progressItems[progress.Id] == nil {
			self.progressItems[progress.Id] = &dockerTypes.PullProgress{}
		}
		if progress.ProgressDetail.Total > 0 && progress.Status == "Downloading" {
			self.progressItems[progress.Id].Downloading = math.Floor((progress.ProgressDetail.Current / progress.ProgressDetail.Total) * 100)
		}
		if progress.ProgressDetail.Total > 0 && progress.Status == "Extracting" {
			self.progressItems[progress.Id].Extracting = math.Floor((progress.ProgressDetail.Current / progress.ProgressDetail.Total) * 100)
		}
		if progress.ProgressDetail.Total > 0 && progress.Status == "Pushing" {
			self.progressItems[progress.Id].Downloading = math.Floor((progress.ProgressDetail.Current / progress.ProgressDetail.Total) * 100)
		}
		if progress.Status == "Download complete" {
			self.progressItems[progress.Id].Downloading = 100
		}
		if progress.Status == "Pull complete" || progress.Status == "Already exists" ||
			progress.Status == "Pushed" || progress.Status == "Layer already exists" {
			self.progressItems[progress.Id].Extracting = 100
			self.progressItems[progress.Id].Downloading = 100
		}
		if self.onProgress != nil && time.Since(self.lastProgressTime) >= time.Second {
			self.broadcast()
			self.lastProgressTime = time.Now()
		}
	}
	self.broadcast()
	return nil
}

func (self *imageProgressReader) broadcast() {
	if self.onProgress == nil {
		return
	}
	progressSnapshot := make(map[string]*dockerTypes.PullProgress, len(self.progressItems))
	for id, item := range self.progressItems {
		progressSnapshot[id] = &dockerTypes.PullProgress{
			Downloading: item.Downloading,
			Extracting:  item.Extracting,
		}
	}
	self.onProgress(progressSnapshot)
}

func (self Client) ImageInspectFileList(ctx context.Context, imageID string) (pathInfo []*fs.FileData, pathList []string, err error) {
	_, err = self.Client.ImageInspect(ctx, imageID)
	if err != nil {
		return nil, nil, err
	}
	out, err := self.Client.ImageSave(ctx, []string{
		imageID,
	})
	if err != nil {
		return nil, nil, err
	}
	defer out.Close()

	tarReader := tar.NewReader(out)
	for {
		header, err := tarReader.Next()
		if err != nil {
			break
		}
		if header.FileInfo().IsDir() {
			continue
		}

		name := header.Name
		isPossibleLayer := strings.HasSuffix(name, ".tar") ||
			strings.HasSuffix(name, ".tar.gz") ||
			strings.HasSuffix(name, ".tgz") ||
			strings.HasPrefix(name, "blobs/") ||
			strings.Contains(name, "/layer.tar")

		if !isPossibleLayer {
			continue
		}

		bufReader := bufio.NewReader(tarReader)
		var layerReader io.Reader = bufReader
		var gzReader *gzip.Reader

		magic, err := bufReader.Peek(2)
		if err == nil && len(magic) == 2 && magic[0] == 0x1f && magic[1] == 0x8b {
			gzReader, err = gzip.NewReader(bufReader)
			if err == nil {
				layerReader = gzReader
			}
		}

		layerTar := tar.NewReader(layerReader)
		tarFileList, err := getFileListFromTar(layerTar)

		if gzReader != nil {
			_ = gzReader.Close()
		}

		if err != nil {
			slog.Debug("docker image inspect file list: skip non-tar layer", "name", name, "error", err)
			continue
		}
		pathInfo = append(pathInfo, tarFileList...)
	}
	sort.Slice(pathInfo, func(i, j int) bool {
		return pathInfo[i].IsDir && !pathInfo[j].IsDir
	})
	sort.Slice(pathInfo, func(i, j int) bool {
		if pathInfo[i].IsDir != pathInfo[j].IsDir {
			return pathInfo[i].IsDir
		}
		return pathInfo[i].Name < pathInfo[j].Name
	})
	pathList = make([]string, 0)
	pathInfo = function.PluckArrayWalk(pathInfo, func(i *fs.FileData) (*fs.FileData, bool) {
		if function.InArray(pathList, i.Name) {
			return nil, false
		} else {
			pathList = append(pathList, i.Name)
			return i, true
		}
	})
	return pathInfo, pathList, nil
}

func getFileListFromTar(tarReader *tar.Reader) (files []*fs.FileData, err error) {
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}

		// always ensure relative path notations are not parsed as part of the filename
		name := path.Clean(header.Name)
		if name == "." {
			continue
		}

		switch header.Typeflag {
		case tar.TypeXGlobalHeader:
			return nil, fmt.Errorf("unexptected tar file: (XGlobalHeader): type=%v name=%s", header.Typeflag, name)
		case tar.TypeXHeader:
			return nil, fmt.Errorf("unexptected tar file (XHeader): type=%v name=%s", header.Typeflag, name)
		default:
			files = append(files, &fs.FileData{
				Path:      filepath.Join("/", header.Name),
				Name:      filepath.Join("/", header.Name),
				Mod:       os.FileMode(header.Mode),
				ModStr:    os.FileMode(header.Mode).String(),
				ModTime:   header.ModTime,
				Change:    fs.ChangeDefault,
				Size:      header.Size,
				User:      fmt.Sprintf("%d", header.Uid),
				Group:     fmt.Sprintf("%d", header.Gid),
				LinkName:  header.Linkname,
				IsDir:     header.Typeflag == tar.TypeDir,
				IsSymlink: false,
			})
		}
	}
	return files, nil
}

func (self Client) ImageLoadFsFile(ctx context.Context, file ioFs.File) error {
	reader, err := self.Client.ImageLoad(ctx, file, client.ImageLoadWithQuiet(false))
	if err != nil {
		return err
	}
	defer reader.Body.Close()

	_, err = io.Copy(io.Discard, reader.Body)
	if err != nil {
		return err
	}
	return nil
}

func (self Client) ImageRemoveAll(ctx context.Context, imageName string) error {
	imageInfo, err := self.Client.ImageInspect(ctx, imageName)
	if err != nil {
		return err
	}
	for _, tag := range imageInfo.RepoTags {
		_, err = self.Client.ImageRemove(ctx, tag, image.RemoveOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}
